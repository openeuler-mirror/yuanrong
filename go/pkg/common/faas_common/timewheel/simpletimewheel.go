/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package timewheel -
package timewheel

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	minPace           = 2 * time.Millisecond
	minSlotNum        = 1
	notifyChannelSize = 1000
)

var (
	timeTriggerPool = sync.Pool{New: func() interface{} {
		return &timeTrigger{}
	}}
)

type timeTrigger struct {
	taskID      string
	times       int
	index       int64
	circle      int64
	circleCount int64
	disable     bool
	ch          chan struct{}
	prev        *timeTrigger
	next        *timeTrigger
}

// SimpleTimeWheel will trigger task at given interval by given times, it contains a certain number of slots and moves
// from one slot to another with a pace which is also the granularity of time wheel, task interval will be measured
// with a number of slots and recorded in the slot arrays, each slot has a linked list to trigger a series of tasks
// when time wheel moves to this slot
type SimpleTimeWheel struct {
	ticker      *time.Ticker
	pace        time.Duration
	perimeter   int64
	slotNum     int64
	curSlot     int64
	pendingTask int
	slots       []*timeTrigger
	readyList   []string
	record      *sync.Map
	notifyCh    chan struct{}
	readyCh     chan struct{}
	stopCh      chan struct{}
	sync.RWMutex
}

// NewSimpleTimeWheel will create a SimpleTimeWheel
func NewSimpleTimeWheel(pace time.Duration, slotNum int64) TimeWheel {
	if pace < minPace {
		pace = minPace
	}
	if slotNum < minSlotNum {
		slotNum = minSlotNum
	}
	timeWheel := &SimpleTimeWheel{
		ticker:    time.NewTicker(pace),
		pace:      pace,
		perimeter: slotNum * int64(pace),
		slotNum:   slotNum,
		curSlot:   0,
		slots:     make([]*timeTrigger, slotNum, slotNum),
		record:    new(sync.Map),
		notifyCh:  make(chan struct{}, notifyChannelSize),
		readyCh:   make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}
	go timeWheel.run()
	return timeWheel
}

func (gt *SimpleTimeWheel) run() {
	for {
		select {
		case <-gt.ticker.C:
			gt.Lock()
			gt.curSlot = (gt.curSlot + 1) % int64(len(gt.slots))
			gt.Unlock()
			gt.checkAndFireTrigger()
		case <-gt.stopCh:
			gt.ticker.Stop()
			return
		}
	}
}

func (gt *SimpleTimeWheel) checkAndFireTrigger() {
	trigger := gt.slots[gt.curSlot]
	var readyList []string
	for trigger != nil {
		if !trigger.disable && trigger.circleCount == trigger.circle {
			trigger.circleCount = 0
			if trigger.times == 0 {
				trigger.disable = true
				gt.record.Delete(trigger.taskID)
				gt.removeTrigger(trigger)
				continue
			}
			readyList = append(readyList, trigger.taskID)
			select {
			case trigger.ch <- struct{}{}:
			default:
			}
			if trigger.times > 0 {
				trigger.times--
			}
		}
		trigger.circleCount++
		trigger = trigger.next
	}
	gt.Lock()
	gt.readyList = readyList
	gt.Unlock()
	if len(readyList) != 0 {
		gt.readyCh <- struct{}{}
	}
}

// Wait will block until tasks are triggered and returns triggered task list
func (gt *SimpleTimeWheel) Wait() []string {
	select {
	case _, ok := <-gt.readyCh:
		if !ok {
			return nil
		}
	}
	gt.RLock()
	readyList := gt.readyList
	gt.RUnlock()
	return readyList
}

// AddTask will add a task which will be triggered periodically over an given interval with given times (-1 means to
// run endlessly), considering that pace has a reasonable size and the logic below won't cost more time than that,
// AddTask won't catch up with the curSlot, so we don't need a mutex. it's also worth noticing that interval can't be
// smaller than the circumference of this time wheel
func (gt *SimpleTimeWheel) AddTask(taskID string, interval time.Duration, times int) (<-chan struct{}, error) {
	if interval < time.Duration(gt.perimeter) {
		return nil, ErrInvalidTaskInterval
	}
	if _, exist := gt.record.Load(taskID); exist {
		return nil, fmt.Errorf("%s, taskId: %s", ErrTaskAlreadyExist.Error(), taskID)
	}
	trigger, ok := timeTriggerPool.Get().(*timeTrigger)
	if !ok {
		return nil, errors.New("not a timeTrigger type")
	}
	gt.Lock()
	curSlot := gt.curSlot
	circle := (int64(interval)/int64(gt.pace) + curSlot + 1) / gt.slotNum
	circleCount := int64(1)
	index := (int64(interval)/int64(gt.pace) + curSlot + 1) % gt.slotNum
	if index > curSlot {
		circleCount--
	}
	trigger.taskID = taskID
	trigger.times = times
	trigger.circle = circle
	trigger.circleCount = circleCount
	trigger.index = index
	trigger.disable = false
	trigger.ch = make(chan struct{}, 1)
	trigger.prev = nil
	trigger.next = gt.slots[index]
	if gt.slots[index] != nil {
		gt.slots[index].prev = trigger
	}
	gt.slots[index] = trigger
	gt.Unlock()
	gt.record.Store(taskID, trigger)
	return trigger.ch, nil
}

// DelTask will delete a task in SimpleTimeWheel and remove its trigger
func (gt *SimpleTimeWheel) DelTask(taskID string) error {
	object, exist := gt.record.Load(taskID)
	if !exist {
		return nil
	}
	gt.record.Delete(taskID)
	trigger, ok := object.(*timeTrigger)
	if !ok {
		return errors.New("not a timeTrigger type")
	}
	// since caller no longer need this task, it's ok that this trigger still fires
	trigger.disable = true
	gt.removeTrigger(trigger)
	timeTriggerPool.Put(trigger)
	return nil
}

// Stop will stop time wheel
func (gt *SimpleTimeWheel) Stop() {
	close(gt.stopCh)
	close(gt.readyCh)
}

// removeTrigger won't set trigger's prev and next to nil since checkAndFireTrigger may processing this trigger right
// now and we don't want to lose track of the next trigger
func (gt *SimpleTimeWheel) removeTrigger(trigger *timeTrigger) {
	gt.Lock()
	defer gt.Unlock()
	// special treatment if this trigger is the head of linked list
	if trigger.prev == nil {
		if trigger.index >= int64(len(gt.slots)) {
			fmt.Errorf("trigger.index is out of slots slice")
		} else {
			gt.slots[trigger.index] = trigger.next
		}
		if trigger.next != nil {
			trigger.next.prev = nil
		}
	} else {
		trigger.prev.next = trigger.next
		if trigger.next != nil {
			trigger.next.prev = trigger.prev
		}
	}
}
