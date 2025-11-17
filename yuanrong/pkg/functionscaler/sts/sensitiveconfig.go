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

// Package sts provide methods for obtaining sensitive information
package sts

import (
	"fmt"

	"github.com/json-iterator/go"
	"huawei.com/wisesecurity/sts-sdk/pkg/remote"

	"yuanrong/pkg/common/faas_common/logger/log"
)

// GetEnvMap - environment variables for sensitive configuration items
func GetEnvMap(configs map[string]string) (map[string]string, error) {
	envMap := make(map[string]string)
	configIDs := getSensitiveValue(configs)
	stsConfigs, err := GetSensitiveConfigIDs(configIDs)
	if err != nil {
		return envMap, err
	}

	sensitiveMap, err := ParseStsResponseStrict(stsConfigs)
	if err != nil {
		return envMap, err
	}

	// envMap key indicates the sensitive configuration name, and value indicates the encrypted value, for example,
	// common.password.etcd.value: ENC(key=servicekek, value=xxx).
	for k, v := range configs {
		envMap[k] = sensitiveMap[v]
	}
	return envMap, nil
}

// return configIDs slice， such as [Service/WiseFunctionService/common.password.etcd.value/dev
// Service/WiseFunctionService/common.password.iamAuth.value/dev]
func getSensitiveValue(configItems map[string]string) []string {
	var configIDs []string
	for _, v := range configItems {
		configIDs = append(configIDs, v)
	}
	return configIDs
}

// ParseStsResponseStrict return key is configID，The value is an encrypted value.
// such as Service/WiseFunctionService/common.password.etcd.value/dev: ENC(key=servicekek, value=xxx)
func ParseStsResponseStrict(respBody *SensitiveConfigResponse) (map[string]string, error) {
	if len(respBody.MissingConfigItems) != 0 {
		return nil, fmt.Errorf("missing item: %s", respBody.MissingConfigItems)
	}
	sensitiveMap := make(map[string]string)
	for _, configItem := range respBody.ConfigItems {
		if configItem.ConfigValue == "" {
			return nil, fmt.Errorf("item value is empty, configID: %s", configItem.ConfigID)
		}
		sensitiveMap[configItem.ConfigID] = configItem.ConfigValue
	}
	return sensitiveMap, nil
}

// GetSensitiveConfigIDs -
func GetSensitiveConfigIDs(configIDs []string) (*SensitiveConfigResponse, error) {
	log.GetLogger().Info("[sts] start get sensitiveConfig")
	size := len(configIDs)
	httpRequest := new(remote.StsHttpRequestBuilder).SetMethod("POST").SetPath(SensitiveConfigPath).Build()
	httpClient := GetStsHTTPClient()
	var stsConfigs = SensitiveConfigResponse{}

	for i := 0; i < size; i += maxConfigIDPerRequest {
		var tmpConfigIDs []string
		for j := i; j < i+maxConfigIDPerRequest && j < size; j++ {
			tmpConfigIDs = append(tmpConfigIDs, configIDs[j])
		}
		req := &configIDsReq{
			ConfigIds: tmpConfigIDs,
		}
		requestJSONByte, err := jsoniter.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal failed: %v", err)
		}
		httpRequest.Body = requestJSONByte
		// If the config ID is incorrect, 200 is returned.
		// The config ID is in Missing Config Items. If the config ID is empty, 400 is returned.
		buf, err := doStsRequest(httpRequest, httpClient)
		if err != nil {
			return nil, err
		}

		var configs SensitiveConfigResponse
		err = jsoniter.Unmarshal(buf, &configs)
		if err != nil {
			return nil, fmt.Errorf("unmarshal failed, error: %v", err)
		}
		stsConfigs.MissingConfigItems = append(stsConfigs.MissingConfigItems, configs.MissingConfigItems...)
		stsConfigs.ConfigItems = append(stsConfigs.ConfigItems, configs.ConfigItems...)
	}
	return &stsConfigs, nil
}
