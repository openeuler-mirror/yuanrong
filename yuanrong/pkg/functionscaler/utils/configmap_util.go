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

// Package utils -
package utils

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"yuanrong/pkg/common/faas_common/constant"
	"yuanrong/pkg/common/faas_common/k8sclient"
	"yuanrong/pkg/common/faas_common/logger/log"
	"yuanrong/pkg/functionscaler/config"
	"yuanrong/pkg/functionscaler/types"
)

const (
	defaultLabelLength = 3
)

// DeleteConfigMapByFuncInfo -
func DeleteConfigMapByFuncInfo(funcSpec *types.FunctionSpecification) {
	if config.GlobalConfig.Scenario != types.ScenarioWiseCloud {
		return
	}
	configmapName := GetConfigmapName(funcSpec.FuncMetaData.FuncName, funcSpec.FuncMetaData.Version)
	nameSpace := config.GlobalConfig.NameSpace
	if nameSpace == "" {
		nameSpace = constant.DefaultNameSpace
	}
	err := k8sclient.GetkubeClient().DeleteK8sConfigMap(nameSpace, configmapName)
	if err != nil {
		log.GetLogger().Errorf("delete configmap error, error is ", err.Error())
	}
}

// GetConfigmapName -
func GetConfigmapName(functionName string, functionVersion string) string {
	return strings.ToLower(fmt.Sprintf("%s-%s", strings.ReplaceAll(functionName, "_", "-"), functionVersion))
}

// IsNeedRaspSideCar -
func IsNeedRaspSideCar(funcSpec *types.FunctionSpecification) bool {
	if funcSpec.ExtendedMetaData.RaspConfig.RaspImage == "" || funcSpec.ExtendedMetaData.RaspConfig.InitImage == "" {
		return false
	}
	if net.ParseIP(funcSpec.ExtendedMetaData.RaspConfig.RaspServerIP) == nil {
		log.GetLogger().Warnf("failed to parse rasp ip: %s ", funcSpec.ExtendedMetaData.RaspConfig.RaspServerIP)
		return false
	}
	if !isValidPort(funcSpec.ExtendedMetaData.RaspConfig.RaspServerPort) {
		log.GetLogger().Warnf("failed to parse rasp "+
			"port: %s ", funcSpec.ExtendedMetaData.RaspConfig.RaspServerPort)
		return false
	}
	return true
}

func isValidPort(port string) bool {
	p, err := strconv.Atoi(port)
	if err != nil {
		return false
	}
	return p > 0 && p <= 65535 // port should between 0 and 65535
}
