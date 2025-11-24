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

// Package selfregister contains service route logic
package selfregister

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"yuanrong.org/kernel/pkg/common/faas_common/constant"
	"yuanrong.org/kernel/pkg/common/faas_common/etcd3"
	"yuanrong.org/kernel/pkg/common/faas_common/logger/log"
	commonTypes "yuanrong.org/kernel/pkg/common/faas_common/types"
	commonUtils "yuanrong.org/kernel/pkg/common/faas_common/utils"
	"yuanrong.org/kernel/pkg/functionscaler/config"
	"yuanrong.org/kernel/pkg/functionscaler/rollout"
	"yuanrong.org/kernel/pkg/functionscaler/types"
)

const (
	defaultLeaseTTL = 15
)

var (
	// Registered -
	Registered bool
	// RegisterKey -
	RegisterKey         string
	selfRegister        *etcd3.EtcdRegister
	selfLocker          *etcd3.EtcdLocker
	maxContendTime      = 300
	contendWaitInterval = 1 * time.Second
)

// RegisterToEtcd -
func RegisterToEtcd(stopCh <-chan struct{}) error {
	discoveryConfig := config.GlobalConfig.SchedulerDiscovery
	log.GetLogger().Infof("start to register to etcd, discoveryConfig %+v", discoveryConfig)
	if discoveryConfig == nil || len(discoveryConfig.RegisterMode) == 0 {
		return nil
	}
	if discoveryConfig != nil && discoveryConfig.RegisterMode == types.RegisterTypeContend {
		log.GetLogger().Infof("start to contend for instance name in etcd")
		selfCurVer := os.Getenv(CurrentVersionEnvKey)
		etcdCurVer := rollout.GetGlobalRolloutHandler().CurrentVersion
		selfLocker = &etcd3.EtcdLocker{
			EtcdClient:     etcd3.GetRouterEtcdClient(),
			LeaseTTL:       defaultLeaseTTL,
			StopCh:         stopCh,
			LockCallback:   putInsSpecForInstanceKey,
			UnlockCallback: delInsSpecForInstanceKey,
			FailCallback:   unsetInstanceRegister,
		}
		if config.GlobalConfig.EnableRollout && selfCurVer != etcdCurVer {
			log.GetLogger().Infof("rollout is enable, this scheduler's version %s doesn't equal to current "+
				"version %s, contend for rollout instead",
				selfCurVer, etcdCurVer)
			return ContendRolloutInEtcd(stopCh)
		}
		if err := contendInstanceInEtcd(stopCh); err != nil {
			return err
		}
	} else {
		log.GetLogger().Infof("start to register for instance name in etcd")
		key, value, err := getInstanceKeyAndValue()
		if err != nil {
			log.GetLogger().Errorf("failed to get service key and value error %s", err.Error())
			return err
		}
		selfRegister = &etcd3.EtcdRegister{
			EtcdClient:  etcd3.GetRouterEtcdClient(),
			InstanceKey: key,
			Value:       value,
			StopCh:      stopCh,
		}
		if err = selfRegister.Register(); err != nil {
			log.GetLogger().Errorf("failed to register to etcd, register failed error %s", err.Error())
			return err
		}
	}
	log.GetLogger().Infof("succeed to register to etcd")
	if err := RegisterRolloutToEtcd(stopCh); err != nil {
		log.GetLogger().Errorf("failed to register to etcd for rollout, register failed error %s", err.Error())
		return err
	}
	return nil
}

func contendInstanceInEtcd(stopCh <-chan struct{}) error {
	log.GetLogger().Infof("start to contend for instance key in etcd")
	var err error
	for i := 0; i < maxContendTime; i++ {
		err = selfLocker.TryLockWithPrefix(constant.ModuleSchedulerPrefix, contendFilterForInstance)
		if err != nil {
			log.GetLogger().Errorf("failed to contend for rollout key, lock failed error %s", err.Error())
			time.Sleep(contendWaitInterval)
			continue
		}
		break
	}
	if err != nil {
		log.GetLogger().Errorf("failed to contend for instance name, lock error %s", err.Error())
		return err
	}
	// succeed to lock instance key, set SelfInstanceName from this key
	log.GetLogger().Infof("succeed to contend for instance name, lock key is %s", selfLocker.GetLockedKey())
	return processLockedInstanceKey(selfLocker.GetLockedKey())
}

func processLockedInstanceKey(lockedKey string) error {
	info, err := commonUtils.GetModuleSchedulerInfoFromEtcdKey(lockedKey)
	if err != nil {
		log.GetLogger().Errorf("failed to register to etcd, get instanceInfo failed error %s", err.Error())
		return err
	}
	Registered = true
	RegisterKey = lockedKey
	SetSelfInstanceName(info.InstanceName)
	log.GetLogger().Infof("succeed to set registerKey to %s selfInstanceName %s", RegisterKey, info.InstanceName)
	return nil
}

func getInstanceKeyAndValue() (string, string, error) {
	clusterID := os.Getenv("CLUSTER_ID")
	nodeIP := os.Getenv("NODE_IP")
	podName := os.Getenv("POD_NAME")
	podIP := os.Getenv("POD_IP")

	err := validateEnvs(clusterID, nodeIP, podName, podIP)
	if err != nil {
		return "", "", err
	}
	key := fmt.Sprintf("/sn/faas-scheduler/instances/%s/%s/%s", clusterID, nodeIP, podName)

	schedulerInfo := commonTypes.InstanceSpecification{
		InstanceID:     podName,
		RuntimeID:      constant.ModuleScheduler,
		DataSystemHost: "",
		RuntimeAddress: fmt.Sprintf("%s:%s", podIP, config.GlobalConfig.ModuleConfig.ServicePort),
		InstanceStatus: commonTypes.InstanceStatus{
			Code: int32(constant.KernelInstanceStatusRunning),
		},
	}
	value, err := json.Marshal(schedulerInfo)
	if err != nil {
		return "", "", err
	}
	return key, string(value), nil
}

func validateEnvs(clusterID, nodeIP, podName, podIP string) error {
	if clusterID == "" || nodeIP == "" || podName == "" || podIP == "" {
		log.GetLogger().Errorf("can not find envs, clusterID %s, nodeIP %s podName %s podIP %s",
			clusterID, nodeIP, podName, podIP)
		return fmt.Errorf("can not find envs")
	}
	return nil
}

func contendFilterForInstance(key, value []byte) bool {
	_, err := commonUtils.GetModuleSchedulerInfoFromEtcdKey(string(key))
	if err != nil {
		return true
	}
	return false
}

func putInsSpecForInstanceKey(locker *etcd3.EtcdLocker) error {
	lockedKey := locker.GetLockedKey()
	log.GetLogger().Infof("start to put insSpec for instance key %s", lockedKey)
	if len(lockedKey) == 0 {
		log.GetLogger().Errorf("failed to get locked key")
		return errors.New("locked key is empty")
	}
	if selfInstanceSpec == nil {
		log.GetLogger().Errorf("failed to get insSpec of this scheduler %s", SelfInstanceID)
		return errors.New("insSpec not found")
	}
	selfInsSpecData, err := json.Marshal(selfInstanceSpec)
	if err != nil {
		log.GetLogger().Errorf("failed to marshal insSpec error %s", err.Error())
		return err
	}
	if err = processEtcdPut(locker.EtcdClient, lockedKey, string(selfInsSpecData)); err != nil {
		log.GetLogger().Errorf("failed to put insSpec for instance key into etcd error %s", err.Error())
		return err
	}
	log.GetLogger().Infof("succeed to put insSpec for instance key %s", lockedKey)
	return nil
}

func delInsSpecForInstanceKey(locker *etcd3.EtcdLocker) error {
	lockedKey := locker.GetLockedKey()
	log.GetLogger().Infof("start to clean insSpec for instance key %s", lockedKey)
	if len(lockedKey) == 0 {
		log.GetLogger().Errorf("failed to get locked key")
		return errors.New("locked key is empty")
	}
	if exist, err := isKeyExist(locker.EtcdClient, lockedKey); err != nil || !exist {
		return fmt.Errorf("key not exist or get error %s, no need clean it", err.Error())
	}
	if err := processEtcdPut(locker.EtcdClient, lockedKey, ""); err != nil {
		log.GetLogger().Errorf("failed to clean insSpec for instance key in etcd error %s", err.Error())
		return err
	}
	Registered = false
	RegisterKey = ""
	log.GetLogger().Infof("succeed to clean insSpec for instance key %s", lockedKey)
	return nil
}

func unsetInstanceRegister() {
	Registered = false
	RegisterKey = ""
	log.GetLogger().Warnf("locker of key %s failed, unset register status", RegisterKey)
}

func processEtcdPut(client *etcd3.EtcdClient, key, value string) error {
	ctx := etcd3.CreateEtcdCtxInfoWithTimeout(context.Background(), etcd3.DurationContextTimeout)
	err := client.Put(ctx, key, value)
	if err != nil {
		log.GetLogger().Errorf("failed to put key %s value %s to etcd %s error %s", key, value, err.Error())
		return err
	}
	return nil
}

func isKeyExist(client *etcd3.EtcdClient, key string) (bool, error) {
	ctx := etcd3.CreateEtcdCtxInfoWithTimeout(context.Background(), etcd3.DurationContextTimeout)
	rsp, err := client.Get(ctx, key)
	if err != nil {
		log.GetLogger().Errorf("failed to check key %s exist error %s", key, err.Error())
		return false, err
	}
	if len(rsp.Kvs) == 0 {
		log.GetLogger().Warnf("locker key has been deleted, skip")
		return false, nil
	}
	return true, nil
}
