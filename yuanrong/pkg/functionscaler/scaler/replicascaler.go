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

// Package scaler -
package scaler

import (
	"sync"
	"time"

	"yuanrong/pkg/common/faas_common/instanceconfig"
	"yuanrong/pkg/common/faas_common/logger/log"
	"yuanrong/pkg/functionscaler/config"
	"yuanrong/pkg/functionscaler/metrics"
	"yuanrong/pkg/functionscaler/types"
	"yuanrong/pkg/functionscaler/utils"
)

// ReplicaScaler will scale based on a given instance number
type ReplicaScaler struct {
	metricsCollector metrics.Collector
	funcKeyWithRes   string
	concurrentNum    int
	pendingRsvInsNum int
	targetRsvInsNum  int
	currentRsvInsNum int
	createError      error
	enable           bool
	scaleLimit       int
	scaleUpHandler   ScaleUpHandler
	scaleDownHandler ScaleDownHandler
	sync.RWMutex
}

// NewReplicaScaler will create a ReplicaScaler
func NewReplicaScaler(funcKeyWithRes string, metricsCollector metrics.Collector, scaleUpHandler ScaleUpHandler,
	scaleDownHandler ScaleDownHandler) InstanceScaler {
	replicaScaler := &ReplicaScaler{
		metricsCollector: metricsCollector,
		funcKeyWithRes:   funcKeyWithRes,
		targetRsvInsNum:  0,
		enable:           false,
		scaleUpHandler:   scaleUpHandler,
		scaleDownHandler: scaleDownHandler,
	}
	log.GetLogger().Infof("create replica scaler for function %s, isManaged is %v", replicaScaler.funcKeyWithRes)
	return replicaScaler
}

// SetEnable will configure the enable of scaler
func (rs *ReplicaScaler) SetEnable(enable bool) {
	rs.Lock()
	if enable == rs.enable {
		rs.Unlock()
		return
	}
	rs.enable = enable
	rs.Unlock()
	if enable {
		rs.handleScale()
	}
}

// TriggerScale will trigger scale
func (rs *ReplicaScaler) TriggerScale() {
	rs.RLock()
	if !rs.enable {
		rs.RUnlock()
		return
	}
	rs.RUnlock()
	rs.handleScale()
}

// CheckScaling will check if scaler is scaling
func (rs *ReplicaScaler) CheckScaling() bool {
	rs.RLock()
	isScaling := rs.currentRsvInsNum != rs.targetRsvInsNum
	rs.RUnlock()
	return isScaling
}

// UpdateCreateMetrics will update create metrics
func (rs *ReplicaScaler) UpdateCreateMetrics(coldStartTime time.Duration) {
}

// HandleInsThdUpdate will update instance thread metrics, totalInsThd increase should be coupled with pendingInsThd
// decrease for better consistency
func (rs *ReplicaScaler) HandleInsThdUpdate(inUseInsThdDiff, totalInsThdDiff int) {
	rs.metricsCollector.UpdateInsThdMetrics(inUseInsThdDiff)
	// replica scaler won't handle scale when only inUseInsThdDiff is non-zero
	if totalInsThdDiff == 0 {
		return
	}
	rs.Lock()
	rs.currentRsvInsNum += totalInsThdDiff / rs.concurrentNum
	rs.Unlock()
	rs.handleScale()
}

// HandleFuncSpecUpdate -
func (rs *ReplicaScaler) HandleFuncSpecUpdate(funcSpec *types.FunctionSpecification) {
	rs.Lock()
	rs.concurrentNum = utils.GetConcurrentNum(funcSpec.InstanceMetaData.ConcurrentNum)
	rs.Unlock()
	log.GetLogger().Infof("config concurrentNum to %d for replica scaler %s", rs.concurrentNum, rs.funcKeyWithRes)
	// some error may be cleared after function update
	rs.handleScale()
}

// HandleInsConfigUpdate -
func (rs *ReplicaScaler) HandleInsConfigUpdate(insConfig *instanceconfig.Configuration) {
	replicaNum := int(insConfig.InstanceMetaData.MinInstance)
	if replicaNum < 0 {
		replicaNum = 0
	}
	rs.Lock()
	prevTargetRsvInsNum := rs.targetRsvInsNum
	rs.targetRsvInsNum = replicaNum
	log.GetLogger().Infof("config reserved num from %d to %d for replica scaler %s", prevTargetRsvInsNum,
		replicaNum, rs.funcKeyWithRes)
	if !rs.enable {
		rs.Unlock()
		return
	}
	rs.Unlock()
	rs.handleScale()
}

// HandleCreateError handles instance create error
func (rs *ReplicaScaler) HandleCreateError(createError error) {
	log.GetLogger().Infof("handle create error %s for function %s", createError, rs.funcKeyWithRes)
	if utils.IsUnrecoverableError(createError) {
		if utils.IsNoNeedToRePullError(createError) {
			rs.Lock()
			rs.enable = false
			rs.createError = createError
			rs.Unlock()
			return
		}
	}
	// if unrecoverable error is cleared then trigger scale, if createError stays nil then skip scale
	rs.Lock()
	noNeedScale := rs.createError == nil && createError == nil
	rs.createError = createError
	rs.Unlock()
	if noNeedScale {
		return
	}
	rs.handleScale()
}

// GetExpectInstanceNumber get function minInstance in etcd
func (rs *ReplicaScaler) GetExpectInstanceNumber() int {
	rs.RLock()
	enable := rs.enable
	currentReplicaNum := rs.targetRsvInsNum
	rs.RUnlock()
	if !enable {
		return 0
	}
	return currentReplicaNum
}

// Destroy will destroy scaler
func (rs *ReplicaScaler) Destroy() {
	rs.Lock()
	rs.enable = false
	rs.targetRsvInsNum = 0
	rs.Unlock()
}

func (rs *ReplicaScaler) handlePendingInsNumIncrease(insDiff int) {
	rs.Lock()
	rs.pendingRsvInsNum += insDiff
	rs.Unlock()
}

func (rs *ReplicaScaler) handlePendingInsNumDecrease(insDiff int) {
	rs.Lock()
	rs.pendingRsvInsNum -= insDiff
	rs.Unlock()
}

func (rs *ReplicaScaler) handleScale() {
	rs.RLock()
	enable := rs.enable
	scaleNum := rs.targetRsvInsNum - rs.currentRsvInsNum
	rs.RUnlock()
	log.GetLogger().Infof("parameters for handle scale of function %s targetRsvInsNum %d currentRsvInsNum %d "+
		"pendingRsvInsNum %d scaleNum %d", rs.funcKeyWithRes, rs.targetRsvInsNum, rs.currentRsvInsNum,
		rs.pendingRsvInsNum, scaleNum)
	if !enable || config.GlobalConfig.DisableReplicaScaler {
		log.GetLogger().Warnf("replicaScaler of function %s disable, targetNum is %d, currentNum is %d, pendingNum "+
			"is %d, funcKey is %s", rs.funcKeyWithRes, rs.targetRsvInsNum, rs.currentRsvInsNum, rs.pendingRsvInsNum)
		return
	}
	if scaleNum > 0 {
		rs.Lock()
		if rs.pendingRsvInsNum >= 0 {
			scaleNum -= rs.pendingRsvInsNum
			if scaleNum <= 0 {
				rs.Unlock()
				log.GetLogger().Warnf("scaleNum <= 0, no need to scale up, "+
					"targetNum is %d, currentNum is %d, pendingNum is %d, funcKey is %s",
					rs.targetRsvInsNum, rs.currentRsvInsNum, rs.pendingRsvInsNum, rs.funcKeyWithRes)
				return
			}
		}
		rs.pendingRsvInsNum += scaleNum
		rs.Unlock()
		log.GetLogger().Infof("calculate scale up instance number for function %s is %d", rs.funcKeyWithRes, scaleNum)
		rs.scaleUpHandler(scaleNum, rs.handlePendingInsNumDecrease)
	} else if scaleNum < 0 {
		rs.Lock()
		if rs.pendingRsvInsNum <= 0 {
			scaleNum -= rs.pendingRsvInsNum
			if scaleNum >= 0 {
				rs.Unlock()
				log.GetLogger().Warnf("scaleNum >= 0, no need to scale down, "+
					"targetNum is %d, currentNum is %d, pendingNum is %d, funcKey is %s",
					rs.targetRsvInsNum, rs.currentRsvInsNum, rs.pendingRsvInsNum, rs.funcKeyWithRes)
				return
			}
		}
		rs.pendingRsvInsNum += scaleNum
		rs.Unlock()
		log.GetLogger().Infof("calculate scale down instance number for function %s is %d", rs.funcKeyWithRes,
			-scaleNum)
		rs.scaleDownHandler(-scaleNum, rs.handlePendingInsNumIncrease)
	}
}
