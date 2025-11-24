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

// Package registry -
package registry

import (
	"os"
	"strings"
	"sync"
	"time"

	"yuanrong.org/kernel/pkg/common/faas_common/alarm"
	"yuanrong.org/kernel/pkg/common/faas_common/constant"
	"yuanrong.org/kernel/pkg/common/faas_common/etcd3"
	"yuanrong.org/kernel/pkg/common/faas_common/instance"
	"yuanrong.org/kernel/pkg/common/faas_common/logger/log"
	"yuanrong.org/kernel/pkg/common/faas_common/types"
	"yuanrong.org/kernel/pkg/common/faas_common/urnutils"
	"yuanrong.org/kernel/pkg/common/faas_common/utils"
	"yuanrong.org/kernel/pkg/functionscaler/config"
	"yuanrong.org/kernel/pkg/functionscaler/selfregister"
)

// FaasSchedulerRegistry watches faasscheduler instance event of etcd
type FaasSchedulerRegistry struct {
	subscriberChans          []chan SubEvent
	stopDelayRemove          map[string]chan struct{}
	FunctionScheduler        map[string]*types.InstanceSpecification
	ModuleScheduler          map[string]*types.InstanceSpecification
	functionSchedulerWatcher etcd3.Watcher
	moduleSchedulerWatcher   etcd3.Watcher
	discoveryKeyType         string
	functionListDoneCh       chan struct{}
	moduleListDoneCh         chan struct{}
	stopCh                   <-chan struct{}
	sync.RWMutex
}

// SchedulerInfo scheduler info
type SchedulerInfo struct {
	SchedulerFuncKey      string                `json:"schedulerFuncKey"`
	SchedulerIDList       []string              `json:"schedulerIDList"`
	SchedulerInstanceList []*types.InstanceInfo `json:"schedulerInstanceList"`
}

// NewFaasSchedulerRegistry will create FaasSchedulerRegistry
func NewFaasSchedulerRegistry(stopCh <-chan struct{}) *FaasSchedulerRegistry {
	discoveryKeyType := constant.SchedulerKeyTypeFunction
	if config.GlobalConfig.SchedulerDiscovery != nil {
		discoveryKeyType = config.GlobalConfig.SchedulerDiscovery.KeyPrefixType
	}
	faasSchedulerRegistry := &FaasSchedulerRegistry{
		stopDelayRemove:    make(map[string]chan struct{}, constant.DefaultMapSize),
		FunctionScheduler:  make(map[string]*types.InstanceSpecification, constant.DefaultMapSize),
		ModuleScheduler:    make(map[string]*types.InstanceSpecification, constant.DefaultMapSize),
		discoveryKeyType:   discoveryKeyType,
		functionListDoneCh: make(chan struct{}, 1),
		moduleListDoneCh:   make(chan struct{}, 1),
		stopCh:             stopCh,
	}
	return faasSchedulerRegistry
}

func (fsr *FaasSchedulerRegistry) initWatcher(etcdClient *etcd3.EtcdClient) {
	fsr.initFunctionSchedulerWatcher(etcdClient)
	fsr.initModuleSchedulerWatcher(etcdClient)
	fsr.WaitForETCDList()
}

func (fsr *FaasSchedulerRegistry) initFunctionSchedulerWatcher(etcdClient *etcd3.EtcdClient) {
	fsr.functionSchedulerWatcher = etcd3.NewEtcdWatcher(
		instanceEtcdPrefix,
		fsr.functionSchedulerFilter,
		fsr.functionSchedulerHandler,
		fsr.stopCh,
		etcdClient)
	fsr.functionSchedulerWatcher.StartList()
}

func (fsr *FaasSchedulerRegistry) initModuleSchedulerWatcher(etcdClient *etcd3.EtcdClient) {
	fsr.moduleSchedulerWatcher = etcd3.NewEtcdWatcher(
		constant.ModuleSchedulerPrefix,
		fsr.moduleSchedulerFilter,
		fsr.moduleSchedulerHandler,
		fsr.stopCh,
		etcdClient)
	fsr.moduleSchedulerWatcher.StartList()
}

// GetFaaSScheduler -
func (fsr *FaasSchedulerRegistry) GetFaaSScheduler(instanceID, keyType string) *types.InstanceSpecification {
	fsr.RLock()
	var insSpec *types.InstanceSpecification
	if keyType == constant.SchedulerKeyTypeFunction {
		insSpec = fsr.FunctionScheduler[instanceID]
	} else if keyType == constant.SchedulerKeyTypeModule {
		insSpec = fsr.ModuleScheduler[instanceID]
	}
	fsr.RUnlock()
	return insSpec
}

// WaitForETCDList -
func (fsr *FaasSchedulerRegistry) WaitForETCDList() {
	log.GetLogger().Infof("start to wait faasscheduler ETCD list")
	if fsr.discoveryKeyType == constant.SchedulerKeyTypeModule {
		var (
			functionListDone bool
			moduleListDone   bool
		)
		for !functionListDone || !moduleListDone {
			select {
			case <-fsr.functionListDoneCh:
				log.GetLogger().Infof("receive function scheduler list done, stop waiting ETCD list")
				functionListDone = true
			case <-fsr.moduleListDoneCh:
				log.GetLogger().Infof("receive module scheduler list done, stop waiting ETCD list")
				moduleListDone = true
			case <-fsr.stopCh:
				log.GetLogger().Warnf("registry is stopped, stop waiting ETCD list")
				return
			}
		}
	} else {
		select {
		case <-fsr.functionListDoneCh:
			log.GetLogger().Infof("receive function scheduler list done, stop waiting ETCD list")
		case <-fsr.stopCh:
			log.GetLogger().Warnf("registry is stopped, stop waiting ETCD list")
		}
	}
}

// RunWatcher -
func (fsr *FaasSchedulerRegistry) RunWatcher() {
	go fsr.functionSchedulerWatcher.StartWatch()
	go fsr.moduleSchedulerWatcher.StartWatch()
}

func (fsr *FaasSchedulerRegistry) functionSchedulerFilter(event *etcd3.Event) bool {
	return !isFaaSScheduler(event.Key)
}

func (fsr *FaasSchedulerRegistry) moduleSchedulerFilter(event *etcd3.Event) bool {
	return !strings.Contains(event.Key, constant.ModuleSchedulerPrefix)
}

func (fsr *FaasSchedulerRegistry) moduleSchedulerHandler(event *etcd3.Event) {
	log.GetLogger().Infof("module scheduler event type %d received: %+v", event.Type, event.Key)
	if event.Type == etcd3.SYNCED {
		log.GetLogger().Infof("received module faasscheduler synced event")
		fsr.moduleListDoneCh <- struct{}{}
		return
	}
	fsr.handleEvent(event, constant.SchedulerKeyTypeModule)
}

func (fsr *FaasSchedulerRegistry) functionSchedulerHandler(event *etcd3.Event) {
	log.GetLogger().Infof("function scheduler event type %d received: %+v", event.Type, event.Key)
	if event.Type == etcd3.SYNCED {
		log.GetLogger().Infof("received function faasscheduler synced event")
		fsr.functionListDoneCh <- struct{}{}
		return
	}
	fsr.handleEvent(event, constant.SchedulerKeyTypeFunction)
}

func (fsr *FaasSchedulerRegistry) handleEvent(event *etcd3.Event, keyType string) {
	switch event.Type {
	case etcd3.PUT:
		if keyType == constant.SchedulerKeyTypeModule {
			fsr.handleModuleSchedulerUpdate(event)
		}
		if keyType == constant.SchedulerKeyTypeFunction {
			fsr.handleFunctionSchedulerUpdate(event)
		}
	case etcd3.DELETE:
		if keyType == constant.SchedulerKeyTypeModule {
			fsr.handleModuleSchedulerRemove(event)
		}
		if keyType == constant.SchedulerKeyTypeFunction {
			fsr.handleFunctionSchedulerRemove(event)
		}
	default:
		log.GetLogger().Warnf("unsupported event type d% for key %s", event.Type, event.Key)
	}
}

// when registerMode is set to registerByContend, the etcd value of module scheduler may be empty if no scheduler locks
// this key
func (fsr *FaasSchedulerRegistry) handleModuleSchedulerUpdate(event *etcd3.Event) {
	insSpec := instance.GetInsSpecFromEtcdValue(event.Key, event.Value)
	insInfo, err := utils.GetModuleSchedulerInfoFromEtcdKey(event.Key)
	if err != nil {
		log.GetLogger().Errorf("failed to parse instanceInfo from key %s error %s", event.Key, err.Error())
		return
	}
	if fsr.discoveryKeyType == constant.SchedulerKeyTypeModule {
		exclusivity := ""
		if insSpec != nil {
			insInfo.InstanceID = insSpec.InstanceID
			if insSpec.CreateOptions != nil {
				exclusivity = insSpec.CreateOptions[constant.SchedulerExclusivityKey]
			}
		}
		selfregister.GlobalSchedulerProxy.Add(insInfo, exclusivity)
	}
	fsr.Lock()
	fsr.ModuleScheduler[insInfo.InstanceName] = insSpec
	fsr.Unlock()
	fsr.publishEvent(SubEventTypeUpdate, insSpec)
	// 目标的效果是，老版本scheduler退出后， rolloutObject置为false，新的scheduler抢锁，registered置为true
	if !selfregister.Registered && insInfo.InstanceName == selfregister.SelfInstanceName &&
		len(insSpec.InstanceID) == 0 {
		selfregister.ReplaceRolloutSubject(fsr.stopCh)
	}
}

func (fsr *FaasSchedulerRegistry) handleModuleSchedulerRemove(event *etcd3.Event) {
	insSpec := instance.GetInsSpecFromEtcdValue(event.Key, event.Value)
	insInfo, err := utils.GetModuleSchedulerInfoFromEtcdKey(event.Key)
	if err != nil {
		log.GetLogger().Errorf("failed to parse instanceInfo from key %s error %s", event.Key, err.Error())
		return
	}
	if fsr.discoveryKeyType == constant.SchedulerKeyTypeModule {
		selfregister.GlobalSchedulerProxy.Remove(insInfo)
	}
	fsr.Lock()
	delete(fsr.ModuleScheduler, insInfo.InstanceName)
	fsr.Unlock()
	fsr.publishEvent(SubEventTypeRemove, insSpec)
}

func (fsr *FaasSchedulerRegistry) handleFunctionSchedulerUpdate(event *etcd3.Event) {
	insSpec := instance.GetInsSpecFromEtcdValue(event.Key, event.Value)
	insInfo, err := utils.GetFunctionInstanceInfoFromEtcdKey(event.Key)
	if err != nil {
		log.GetLogger().Errorf("failed to parse instanceInfo from key %s error %s", event.Key, err.Error())
		return
	}
	if fsr.discoveryKeyType == constant.SchedulerKeyTypeFunction {
		if utils.CheckFaaSSchedulerInstanceFault(insSpec.InstanceStatus) {
			go fsr.delFunctionSchedulerFromProxy(insSpec, insInfo)
		} else if insSpec.InstanceStatus.Code == int32(constant.KernelInstanceStatusRunning) ||
			insSpec.InstanceStatus.Code == int32(constant.KernelInstanceStatusCreating) {
			fsr.addFunctionSchedulerToProxy(insSpec, insInfo)
		}
	}
	fsr.Lock()
	fsr.FunctionScheduler[insInfo.InstanceName] = insSpec
	fsr.Unlock()
	if insSpec.InstanceID == selfregister.SelfInstanceID {
		selfregister.SetSelfInstanceSpec(insSpec)
	}
}

func (fsr *FaasSchedulerRegistry) handleFunctionSchedulerRemove(event *etcd3.Event) {
	insSpec := instance.GetInsSpecFromEtcdValue(event.Key, event.Value)
	insInfo, err := utils.GetFunctionInstanceInfoFromEtcdKey(event.Key)
	if err != nil {
		log.GetLogger().Errorf("failed to parse instanceInfo from key %s error %s", event.Key, err.Error())
		return
	}
	if fsr.discoveryKeyType == constant.SchedulerKeyTypeFunction {
		selfregister.GlobalSchedulerProxy.Remove(insInfo)
	}
	fsr.Lock()
	delete(fsr.FunctionScheduler, insInfo.InstanceName)
	fsr.Unlock()
	fsr.publishEvent(SubEventTypeRemove, insSpec)
}

func (fsr *FaasSchedulerRegistry) addFunctionSchedulerToProxy(insSpec *types.InstanceSpecification,
	info *types.InstanceInfo) {
	exclusivity := ""
	if insSpec.CreateOptions != nil {
		exclusivity = insSpec.CreateOptions[constant.SchedulerExclusivityKey]
	}
	if !selfregister.GlobalSchedulerProxy.Contains(info.InstanceName) {
		selfregister.GlobalSchedulerProxy.Add(info, exclusivity)
	}
	fsr.Lock()
	if stopRemoveCh, ok := fsr.stopDelayRemove[info.InstanceName]; ok {
		// 这里表示有scheduler从故障恢复到正常了
		utils.SafeCloseChannel(stopRemoveCh)
		delete(fsr.stopDelayRemove, info.InstanceName)
		fsr.Unlock()
		return
	}
	selfregister.GlobalSchedulerProxy.Reset()
	fsr.Unlock()
	fsr.publishEvent(SubEventTypeUpdate, insSpec)
}

func (fsr *FaasSchedulerRegistry) delFunctionSchedulerFromProxy(insSpec *types.InstanceSpecification,
	info *types.InstanceInfo) {
	fsr.Lock()
	if !selfregister.GlobalSchedulerProxy.Contains(info.InstanceName) {
		fsr.Unlock()
		return
	}
	if _, exist := fsr.stopDelayRemove[info.InstanceName]; exist {
		fsr.Unlock()
		return
	}
	ch := make(chan struct{})
	fsr.stopDelayRemove[info.InstanceName] = ch
	fsr.Unlock()
	go func(info *types.InstanceInfo, ch chan struct{}) {
		log.GetLogger().Infof("start to delay delete faasscheduler %s", info.InstanceName)
		delayRemoveTimer := time.NewTimer(constant.SchedulerRecoverTime)
		defer delayRemoveTimer.Stop()
		select {
		case <-ch:
			log.GetLogger().Infof("faasscheduler %s recovered, won't delete hash node", info.InstanceName)
			return
		case <-delayRemoveTimer.C:
			log.GetLogger().Infof("delay timer triggers, deleting faasscheduler %s now", info.InstanceName)
			fsr.Lock()
			if _, exist := fsr.stopDelayRemove[info.InstanceName]; exist {
				delete(fsr.stopDelayRemove, info.InstanceName)
				selfregister.GlobalSchedulerProxy.Remove(info)
				fsr.Unlock()
				fsr.publishEvent(SubEventTypeRemove, insSpec)
			} else {
				fsr.Unlock()
			}
			reportOrClearAlarm()
		}
	}(info, ch)
}

func reportOrClearAlarm() {
	if config.GlobalConfig.AlarmConfig.EnableAlarm {
		alarmDetail := &alarm.Detail{
			SourceTag: os.Getenv(constant.PodNameEnvKey) + "|" + os.Getenv(constant.PodIPEnvKey) +
				"|" + os.Getenv(constant.ClusterName) + "|FaaSSchedulerHashRingRemoved",
			OpType:         alarm.GenerateAlarmLog,
			Details:        "faasscheduler has removed from hash ring",
			StartTimestamp: int(time.Now().Unix()),
			EndTimestamp:   0,
		}

		alarmInfo := &alarm.LogAlarmInfo{
			AlarmID:    alarm.FaaSSchedulerRemovedFromHashRing00001,
			AlarmName:  "FaaSSchedulerRemoved",
			AlarmLevel: alarm.Level2,
		}
		alarm.ReportOrClearAlarm(alarmInfo, alarmDetail)
	}
}

// isFaaSScheduler used to filter the etcd event which stands for a faas scheduler
func isFaaSScheduler(etcdPath string) bool {
	info, err := utils.GetFunctionInstanceInfoFromEtcdKey(etcdPath)
	if err != nil {
		return false
	}
	return strings.Contains(info.FunctionName, "faasscheduler")
}

// GetSchedulerInfo return scheduler info
func (fsr *FaasSchedulerRegistry) GetSchedulerInfo() *SchedulerInfo {
	schedulerInfo := &SchedulerInfo{}
	selfregister.GlobalSchedulerProxy.FaaSSchedulers.Range(func(key, value any) bool {
		faasSchedulerID, ok := key.(string)
		if !ok {
			return true
		}
		faaSScheduler, ok := value.(*types.InstanceInfo)
		if !ok {
			return true
		}
		schedulerInfo.SchedulerIDList = append(schedulerInfo.SchedulerIDList, faasSchedulerID)
		schedulerInfo.SchedulerFuncKey = urnutils.CombineFunctionKey(faaSScheduler.TenantID,
			faaSScheduler.FunctionName, faaSScheduler.Version)
		schedulerInfo.SchedulerInstanceList = append(schedulerInfo.SchedulerInstanceList, faaSScheduler)
		return true
	})
	return schedulerInfo
}

// addSubscriberChan will add channel, subscribed by FaaSScheduler
func (fsr *FaasSchedulerRegistry) addSubscriberChan(subChan chan SubEvent) {
	fsr.Lock()
	fsr.subscriberChans = append(fsr.subscriberChans, subChan)
	fsr.Unlock()
}

// publishEvent will publish instance event via channel
func (fsr *FaasSchedulerRegistry) publishEvent(eventType EventType, insSpec *types.InstanceSpecification) {
	for _, subChan := range fsr.subscriberChans {
		if subChan != nil {
			subChan <- SubEvent{
				EventType: eventType,
				EventMsg:  insSpec,
			}
		}
	}
}
