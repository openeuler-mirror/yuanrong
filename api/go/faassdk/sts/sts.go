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

// Package sts used for init sts
package sts

import (
	"os"

	"github.com/magiconair/properties"
	"huawei.com/wisesecurity/sts-sdk/pkg/stsgoapi"

	"yuanrong.org/kernel/runtime/faassdk/types"
	"yuanrong.org/kernel/runtime/libruntime/common/faas/logger"
	"yuanrong.org/kernel/runtime/libruntime/common/logger/config"
)

// EnvSTSEnable flag
const EnvSTSEnable = "STS_ENABLE"
const fileMode = 0640

// InitStsSDK - Configure sts go sdk
func InitStsSDK(serverCfg types.StsServerConfig) error {
	initStsSdkLog()
	logger.GetLogger().Infof("finished to init sts sdk log")
	stsProperties := properties.LoadMap(
		map[string]string{
			"sts.server.domain":     serverCfg.Domain,
			"sts.config.path":       serverCfg.Path,
			"sts.connect.timeout":   "20000",
			"sts.handshake.timeout": "20000",
		},
	)
	err := stsgoapi.InitWith(*stsProperties)
	return err
}

func initStsSdkLog() {
	coreInfo, err := config.GetCoreInfoFromEnv()
	if err != nil {
		coreInfo = config.GetDefaultCoreInfo()
	}
	stsSdkLogFilePath := coreInfo.FilePath + "/sts.sdk.log"
	stsgoapi.SetLogFile(stsSdkLogFilePath)
	file, err := os.OpenFile(stsSdkLogFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fileMode)
	if err != nil {
		logger.GetLogger().Errorf("failed to open stsSdkLogFile")
		return
	}
	defer file.Close()
	return
}
