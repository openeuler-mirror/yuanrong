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

// Package dynamicconfigmanager -
package dynamicconfigmanager

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"yuanrong.org/kernel/pkg/common/faas_common/constant"
	"yuanrong.org/kernel/pkg/common/faas_common/k8sclient"
	"yuanrong.org/kernel/pkg/common/faas_common/logger/log"
	"yuanrong.org/kernel/pkg/common/faas_common/urnutils"
	"yuanrong.org/kernel/pkg/functionscaler/config"
	"yuanrong.org/kernel/pkg/functionscaler/types"
)

const (
	// DynamicConfigSuffix dynamic config suffix
	DynamicConfigSuffix = "-dynamic-config"
	// DynamicConfigMapName default dynamic configmap name
	DynamicConfigMapName = "dynamic-config"
	// DefaultDynamicConfigPath default dynamic config path
	DefaultDynamicConfigPath = "/opt/dynamic-config"
	// DynamicConfigMapNameKey default dynamic configmap key
	DynamicConfigMapNameKey = "dynamic-config.properties"
)

// HandleUpdateFunctionEvent DynamicConfigManager hande function update event
func HandleUpdateFunctionEvent(funcSpec *types.FunctionSpecification) {
	if config.GlobalConfig.Scenario == types.ScenarioWiseCloud {
		return
	}
	log.GetLogger().Infof("handling dynamic config update for function %s", funcSpec.FuncKey)
	// the dynamic configuration of the latest version function may change from
	// enabled to disable. In this case, need to delete the old configmap.
	if !funcSpec.ExtendedMetaData.DynamicConfig.Enabled {
		deleteConfigMap(funcSpec)
		return
	}
	createOrUpdateConfigMap(funcSpec)
}

// HandleDeleteFunctionEvent DynamicConfigManager hande function delete event
func HandleDeleteFunctionEvent(funcSpec *types.FunctionSpecification) {
	if config.GlobalConfig.Scenario == types.ScenarioWiseCloud {
		return
	}
	log.GetLogger().Infof("handling dynamic config delete for function %s", funcSpec.FuncKey)
	deleteConfigMap(funcSpec)
}

func createOrUpdateConfigMap(funcSpec *types.FunctionSpecification) {
	log.GetLogger().Infof("dynamic config is enabled, start to create or update configmap for "+
		"function %s", funcSpec.FuncKey)
	cmName := urnutils.CrNameByURN(funcSpec.FuncMetaData.FunctionVersionURN) + "-dynamic-config"
	nameSpace := config.GlobalConfig.NameSpace
	if nameSpace == "" {
		nameSpace = constant.DefaultNameSpace
	}
	expectedConfigmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: nameSpace,
		},
		Data: buildDynamicConfigContent(funcSpec),
	}
	err := k8sclient.GetkubeClient().CreateOrUpdateConfigMap(expectedConfigmap)
	if err != nil {
		log.GetLogger().Errorf("Create or update Configmap failed!Configmap.Name: %s, Error: %s",
			cmName, err.Error())
	}
}

func deleteConfigMap(funcSpec *types.FunctionSpecification) {
	cmName := urnutils.CrNameByURN(funcSpec.FuncMetaData.FunctionVersionURN) + "-dynamic-config"
	nameSpace := config.GlobalConfig.NameSpace
	if nameSpace == "" {
		nameSpace = constant.DefaultNameSpace
	}
	err := k8sclient.GetkubeClient().DeleteK8sConfigMap(nameSpace, cmName)
	if err != nil {
		log.GetLogger().Errorf("Delete Configmap failed!Configmap.Name: %s, Error: %s",
			cmName, err.Error())
	}
}

func buildDynamicConfigContent(funcSpec *types.FunctionSpecification) map[string]string {
	var dynamicConfigContent = make(map[string]string)
	var builder strings.Builder
	for _, configKV := range funcSpec.ExtendedMetaData.DynamicConfig.ConfigContent {
		builder.WriteString(fmt.Sprintf("%s=%s\n", configKV.Name, configKV.Value))
	}
	dynamicConfigContent[DynamicConfigMapNameKey] = builder.String()
	return dynamicConfigContent
}
