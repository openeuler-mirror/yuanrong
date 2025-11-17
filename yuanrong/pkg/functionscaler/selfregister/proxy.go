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

// Package selfregister -
package selfregister

import (
	"os"
	"strings"
	"sync"
	"time"

	"yuanrong/pkg/common/faas_common/constant"
	"yuanrong/pkg/common/faas_common/loadbalance"
	"yuanrong/pkg/common/faas_common/logger/log"
	"yuanrong/pkg/common/faas_common/types"
	"yuanrong/pkg/functionscaler/config"
)

const (
	// HashRingSize the concurrent hash ring length
	HashRingSize = 5000
	// GetHashLenInternal -
	GetHashLenInternal  = 10 * time.Millisecond
	etcdPathElementsLen = 14
)

var (
	// SelfInstanceID proxy is the singleton proxy
	SelfInstanceID string
	// SelfInstanceName is the instanceName used when discovery type is module
	SelfInstanceName string
	selfInstanceSpec *types.InstanceSpecification
)

var (
	// GlobalSchedulerProxy -
	GlobalSchedulerProxy = NewSchedulerProxy(
		loadbalance.NewConcurrentCHGeneric(HashRingSize),
	)
)

// SchedulerProxy is used to get instances from FaaSScheduler via a grpc stream
type SchedulerProxy struct {
	FaaSSchedulers sync.Map
	// used to select a FaaSScheduler by the func info Concurrent Consistent Hash
	loadBalance loadbalance.LoadBalance
}

func init() {
	log.GetLogger().Infof("set SelfInstanceID to %s", os.Getenv("INSTANCE_ID"))
	SelfInstanceID = os.Getenv("INSTANCE_ID")
}

// SetSelfInstanceName -
func SetSelfInstanceName(instanceName string) {
	log.GetLogger().Infof("set SelfInstanceName to %s", instanceName)
	SelfInstanceName = instanceName
}

// SetSelfInstanceSpec -
func SetSelfInstanceSpec(insSpec *types.InstanceSpecification) {
	selfInstanceSpec = insSpec
}

// GetSchedulerProxyName -
func GetSchedulerProxyName() string {
	schedulerDiscovery := config.GlobalConfig.SchedulerDiscovery
	if schedulerDiscovery != nil && schedulerDiscovery.KeyPrefixType == constant.SchedulerKeyTypeModule {
		return SelfInstanceName
	}
	return SelfInstanceID
}

// NewSchedulerProxy return an instance pool which get the instance from the remote FaaSScheduler
func NewSchedulerProxy(lb loadbalance.LoadBalance) *SchedulerProxy {
	return &SchedulerProxy{
		loadBalance: lb,
	}
}

// Add an FaaSScheduler
func (sp *SchedulerProxy) Add(faaSScheduler *types.InstanceInfo, exclusivity string) {
	sp.FaaSSchedulers.Store(faaSScheduler.InstanceName, faaSScheduler)
	if exclusivity != "" {
		// do not add exclusivity scheduler to load balance
		log.GetLogger().Infof("no need to add scheduler %s to load balance for exclusivity %s",
			faaSScheduler.InstanceName, exclusivity)
		return
	}
	log.GetLogger().Debugf("add faasscheduler to proxy, id is %s, name is %s",
		faaSScheduler.InstanceID, faaSScheduler.InstanceName)
	sp.loadBalance.Add(faaSScheduler.InstanceName, 0)
}

// Remove a FaaSScheduler
func (sp *SchedulerProxy) Remove(faasScheduler *types.InstanceInfo) {
	sp.loadBalance.Remove(faasScheduler.InstanceName)
	sp.FaaSSchedulers.Delete(faasScheduler.InstanceName)
}

// Reset - reset hash anchor point
func (sp *SchedulerProxy) Reset() {
	sp.loadBalance.Reset()
}

// Contains - if hash ring contains this scheduelr
func (sp *SchedulerProxy) Contains(id string) bool {
	_, ok := sp.FaaSSchedulers.Load(id)
	return ok
}

// CheckFuncOwner determine etcd event should or not to be deal with
func (sp *SchedulerProxy) CheckFuncOwner(funcKey string) bool {
	log.GetLogger().Debugf("check which faas scheduler instance should process function %s", funcKey)
	// select one FaaSScheduler by the func key
	next := sp.loadBalance.Next(funcKey, false)
	faasSchedulerName, ok := next.(string)
	if !ok {
		log.GetLogger().Errorf("failed to parse the result of load balance: %+v", next)
		return false
	}
	if strings.TrimSpace(faasSchedulerName) == "" {
		log.GetLogger().Errorf("no available faas scheduler was found")
		return false
	}
	faaSSchedulerData, ok := sp.FaaSSchedulers.Load(faasSchedulerName)
	if !ok {
		log.GetLogger().Errorf("failed to get the faas scheduler named %s", faasSchedulerName)
		return false
	}
	faaSScheduler, ok := faaSSchedulerData.(*types.InstanceInfo)
	if !ok {
		log.GetLogger().Errorf("invalid faas scheduler named %s: %#v", faasSchedulerName, faaSSchedulerData)
		return false
	}
	if faaSScheduler.InstanceName != GetSchedulerProxyName() {
		log.GetLogger().Warnf("instanceID self is: %s, hash computed: %s", GetSchedulerProxyName(),
			faaSScheduler.InstanceName)
		return false
	}
	log.GetLogger().Infof("this scheduler %s should process function %s", SelfInstanceID, funcKey)
	return true
}

// WaitForHash wait for num of concurrent hash node to add
func (sp *SchedulerProxy) WaitForHash(num int) {
	if num == 0 {
		return
	}
	for {
		hashLen := 0
		sp.FaaSSchedulers.Range(func(k, v interface{}) bool {
			hashLen++
			return true
		})
		if hashLen < num {
			time.Sleep(GetHashLenInternal)
			continue
		}
		log.GetLogger().Infof("succeeded to create num: %d of hash ring node", num)
		return
	}
}
