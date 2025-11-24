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
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSimpleTimeWheelBasic(t *testing.T) {
	timeWheel := NewSimpleTimeWheel(5*time.Millisecond, 10)
	defer timeWheel.Stop()
	time.Sleep(11 * time.Millisecond)
	taskName := "TestSimpleTimeWheelBasic_" + "task-1"
	ch, err := timeWheel.AddTask(taskName, 500*time.Millisecond, -1)
	addTime := time.Now()
	if err != nil {
		t.Errorf("failed to add task error %s", err)
	}
	var triggerTime time.Time
	select {
	case <-time.NewTimer(750 * time.Millisecond).C:
		t.Errorf("timeout waiting for timeWheel to trigger after %d", time.Now().Sub(addTime).Milliseconds())
	case <-ch:
		triggerTime = time.Now()
		interval := int(math.Floor(float64(triggerTime.Sub(addTime).Milliseconds())))
		assert.Equal(t, true, interval >= 450 && interval <= 750)
	}

	err = timeWheel.DelTask(taskName)
	if err != nil {
		t.Errorf("failed to delete task error %s", err)
	}
	select {
	case <-time.NewTimer(200 * time.Millisecond).C:
	case <-ch:
		t.Errorf("trigger should not fire")
	}
}

func TestSimpleTimeWheel_Wait(t *testing.T) {
	readyCh := make(chan struct{})
	readyList := []string{"TestSimpleTimeWheel_Wait_task1", "TestSimpleTimeWheel_Wait_task2"}

	wheel := &SimpleTimeWheel{
		readyCh:   readyCh,
		readyList: readyList,
	}

	go func() {
		readyCh <- struct{}{}
	}()

	result := wheel.Wait()
	assert.Equal(t, readyList, result, "The readyList should be returned")
	close(readyCh)
	result = wheel.Wait()
	assert.Nil(t, result, "The result should be nil when channel is closed")
}

func TestSimpleTimeWheelCombination(t *testing.T) {
	timeWheel := NewSimpleTimeWheel(5*time.Millisecond, 10)
	defer timeWheel.Stop()
	var (
		err          error
		task1Ch      <-chan struct{}
		task2Ch      <-chan struct{}
		task3Ch      <-chan struct{}
		task2AddTime time.Time
		task3AddTime time.Time
	)
	wg := sync.WaitGroup{}
	wg.Add(1)
	task1Name := "TestSimpleTimeWheelCombination_" + "task-1"
	task2Name := "TestSimpleTimeWheelCombination_" + "task-2"
	task3Name := "TestSimpleTimeWheelCombination_" + "task-3"
	go func() {
		task1Ch, err = timeWheel.AddTask(task1Name, time.Duration(500)*time.Millisecond, -1)
		if err != nil {
			t.Errorf("failed to add task error %s", err)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		task2Ch, err = timeWheel.AddTask(task2Name, time.Duration(500)*time.Millisecond, -1)
		task2AddTime = time.Now()
		if err != nil {
			t.Errorf("failed to add task error %s", err)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		task3Ch, err = timeWheel.AddTask(task3Name, time.Duration(500)*time.Millisecond, -1)
		task3AddTime = time.Now()
		if err != nil {
			t.Errorf("failed to add task error %s", err)
		}
		wg.Done()
	}()
	wg.Wait()
	err = timeWheel.DelTask(task1Name)
	if err != nil {
		t.Errorf("failed to delete task error %s", err)
	}
	done := 0
	timer := time.NewTimer(900 * time.Millisecond)
	defer timer.Stop()
	for done != 2 {
		select {
		case <-timer.C:
			t.Errorf("timeout waiting for timeWheel to trigger")
		case <-task1Ch:
			t.Errorf("trigger should not fire")
		case <-task2Ch:
			interval := int(math.Floor(float64(time.Now().Sub(task2AddTime).Milliseconds())))
			if interval < 450 || interval > 800 {
				t.Errorf("task2's trigger interval %d is out of range [450, 800]", interval)
			}
			done++
		case <-task3Ch:
			interval := int(math.Floor(float64(time.Now().Sub(task3AddTime).Milliseconds())))
			if interval < 450 || interval > 800 {
				t.Errorf("task3's trigger interval %d is out of range [450, 800]", interval)
			}
			done++
		}
	}
}

func TestSimpleTimeWheel_Stop(t *testing.T) {
	timeWheel := NewSimpleTimeWheel(2*time.Millisecond, 10)
	timeWheel.Stop()
}

func TestNewSimpleTimeWheel(t *testing.T) {
	timeWheel := NewSimpleTimeWheel(minPace-1, 0)
	defer timeWheel.Stop()
	assert.NotNil(t, timeWheel)
}

func TestSimpleTimeWheelBasic1(t *testing.T) {
	timeWheel := NewSimpleTimeWheel(10*time.Millisecond, 10)
	defer timeWheel.Stop()
	time.Sleep(11 * time.Millisecond)
	task1Name := "TestSimpleTimeWheelBasic1_" + "task-1"
	ch, err := timeWheel.AddTask(task1Name, 1000*time.Millisecond, -1)
	addTime := time.Now()
	if err != nil {
		t.Errorf("failed to add task error %s", err)
	}
	var triggerTime time.Time
	select {
	case <-time.NewTimer(10000 * time.Millisecond).C:
		t.Errorf("timeout waiting for timeWheel to trigger %s", time.Now().Format(time.RFC3339Nano))
	case <-ch:
		triggerTime = time.Now()
		interval := int(math.Floor(float64(triggerTime.Sub(addTime).Milliseconds())))
		t.Logf("show invterval %d\n", interval)
		assert.Equal(t, true, interval >= 800 && interval <= 1400)
	}

	err = timeWheel.DelTask(task1Name)
	if err != nil {
		t.Errorf("failed to delete task error %s", err)
	}
	select {
	case <-time.NewTimer(1000 * time.Millisecond).C:
	case <-ch:
		t.Errorf("trigger should not fire")
	}
}
