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

// Package main -
package main

import (
	"errors"
	"fmt"
	"time"

	_ "go.uber.org/automaxprocs"
	"yuanrong.org/kernel/runtime/libruntime/api"

	"yuanrong/pkg/common/faas_common/logger/log"
	"yuanrong/pkg/common/faas_common/trafficlimit"
	"yuanrong/pkg/common/faas_common/utils"
	"yuanrong/pkg/functionscaler"
	"yuanrong/pkg/functionscaler/config"
	"yuanrong/pkg/functionscaler/healthcheck"
	"yuanrong/pkg/functionscaler/instancepool"
	"yuanrong/pkg/functionscaler/registry"
	"yuanrong/pkg/functionscaler/rollout"
	"yuanrong/pkg/functionscaler/selfregister"
	"yuanrong/pkg/functionscaler/signalmanager"
	"yuanrong/pkg/functionscaler/state"
)

const (
	shutdownWaitTime = 5 * time.Second
)

var (
	stopCh = make(chan struct{})
	errCh  = make(chan error)
)

// InitHandlerLibruntime is the init handler called by runtime based on multi libruntime
func InitHandlerLibruntime(args []api.Arg, libruntimeAPI api.LibruntimeAPI) ([]byte, error) {
	log.SetupLoggerLibruntime(libruntimeAPI.GetFormatLogger())
	var err error
	defer func() {
		if err != nil {
			fmt.Printf("panic, module: faasscheduler, err: %s\n", err.Error())
			log.GetLogger().Errorf("panic, module: faasscheduler, err: %s", err.Error())
		}
		log.GetLogger().Sync()
	}()
	if len(args) == 0 || libruntimeAPI == nil {
		return []byte(""), errors.New("init args empty")
	}
	if args[0].Type != api.Value {
		return []byte(""), errors.New("arg type error")
	}
	if err = config.InitConfig(args[0].Data); err != nil {
		return []byte(""), err
	}
	if err = config.InitEtcd(stopCh); err != nil {
		log.GetLogger().Errorf("failed to init etcd ,err:%s", err.Error())
		return nil, err
	}
	state.InitState()
	if err = setupFunctionSchedulerLibruntime(libruntimeAPI); err != nil {
		return []byte(""), err
	}
	registry.StartRegistry()
	if err = healthcheck.StartHealthCheck(errCh); err != nil {
		return []byte(""), err
	}
	config.ClearSensitiveInfo()
	return []byte(""), nil
}

// CallHandlerLibruntime is the call handler called by runtime based on multi libruntime
func CallHandlerLibruntime(args []api.Arg) ([]byte, error) {
	traceID := string(args[len(args)-1].Data)

	if functionscaler.GetGlobalScheduler() == nil {
		return nil, fmt.Errorf("faas scheduler is not initialized, traceID: %s", traceID)
	}
	return functionscaler.GetGlobalScheduler().ProcessInstanceRequestLibruntime(args, traceID)
}

// CheckpointHandlerLibruntime is the checkpoint handler called by runtime based on multi libruntime
func CheckpointHandlerLibruntime(checkpointID string) ([]byte, error) {
	return state.GetStateByte()
}

// RecoverHandlerLibruntime is the recover handler called by runtime based on multi libruntime
func RecoverHandlerLibruntime(stateData []byte, libruntimeAPI api.LibruntimeAPI) error {
	var err error
	log.SetupLoggerLibruntime(libruntimeAPI.GetFormatLogger())
	log.GetLogger().Infof("trigger: libruntime faasscheduler.RecoverHandler")
	if err = state.SetState(stateData); err != nil {
		return fmt.Errorf("libruntime faaS scheduler recover error is :%s", err.Error())
	}
	state.InitState()
	if err = state.RecoverConfig(); err != nil {
		return fmt.Errorf("libruntime recover config error:%s", err.Error())
	}
	if err = config.InitEtcd(stopCh); err != nil {
		log.GetLogger().Errorf("failed to init etcd ,err:%s", err.Error())
		return err
	}
	if err = setupFunctionSchedulerLibruntime(libruntimeAPI); err != nil {
		log.GetLogger().Errorf("libruntime recover initHandler error:%s", err.Error())
		return fmt.Errorf("faaS frontend recover initHandler error of libruntime is :%s", err.Error())
	}
	if functionscaler.GetGlobalScheduler() != nil {
		functionscaler.GetGlobalScheduler().Recover()
	}
	state.Update(config.GlobalConfig)
	registry.StartRegistry()
	config.ClearSensitiveInfo()
	return nil
}

// ShutdownHandlerLibruntime is the shutdown handler called by runtime based on multi libruntime
func ShutdownHandlerLibruntime(gracePeriodSecond uint64) error {
	log.GetLogger().Infof("trigger: faasscheduler.ShutdownHandler")
	utils.SafeCloseChannel(stopCh)
	time.Sleep(shutdownWaitTime)
	log.GetLogger().Infof("faasschedulerLibruntime exit")
	log.GetLogger().Sync()
	return nil
}

// SignalHandlerLibruntime is the signal handler called by runtime based on multi libruntime
func SignalHandlerLibruntime(signal int, payload []byte) error {
	return nil
}

func setupFunctionSchedulerLibruntime(fsClient api.LibruntimeAPI) error {
	rollout.SetRolloutSdkClient(fsClient)
	if err := registry.InitRegistry(stopCh); err != nil {
		return err
	}
	if err := selfregister.RegisterToEtcd(stopCh); err != nil {
		return err
	}

	signalmanager.GetSignalManager().SetKillFunc(fsClient.Kill)

	instancepool.SetGlobalSdkClient(fsClient)
	functionscaler.InitGlobalScheduler(stopCh)
	registry.ProcessETCDList()
	trafficlimit.SetFunctionLimitRate(config.GlobalConfig.FunctionLimitRate)
	return nil
}
