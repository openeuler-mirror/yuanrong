//go:build cryptoapi
// +build cryptoapi

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

// Package crypto for auth
package crypto

import (
	"cryptoapi"
	"fmt"
	"path"

	"yuanrong/pkg/common/faas_common/logger/log"
)

// SCCInitialized -
func SCCInitialized() bool {
	m.RLock()
	defer m.RUnlock()
	return sccInitialized
}

// GetSCCAlgorithm -
func GetSCCAlgorithm(algorithm string) int {
	switch algorithm {
	case Aes128Gcm:
		return cryptoapi.ALG_AES128_GCM
	case Aes256Gcm:
		return cryptoapi.ALG_AES256_GCM
	case Aes256Cbc:
		return cryptoapi.ALG_AES256_CBC
	case Sm4Cbc:
		return cryptoapi.ALG_SM4_CBC
	case Sm4Ctr:
		return cryptoapi.ALG_SM4_CTR
	default:
		return cryptoapi.ALG_AES256_GCM
	}
}

// InitializeSCC -
func InitializeSCC(config SccConfig) bool {
	m.Lock()
	defer m.Unlock()
	if !config.Enable {
		return true
	}
	options := cryptoapi.NewSccOptions()
	const configPath = "/home/sn/resource/scc"
	sccConfigPath := config.SccConfigPath
	if sccConfigPath == "" {
		sccConfigPath = configPath
	}
	options.PrimaryKeyFile = path.Join(sccConfigPath, "primary.ks")
	options.StandbyKeyFile = path.Join(sccConfigPath, "standby.ks")
	options.LogPath = "/tmp/log/"
	options.LogFile = "scc"
	options.DefaultAlgorithm = GetSCCAlgorithm(config.Algorithm)
	options.RandomDevice = "/dev/random"
	options.EnableChangeFilePermission = 0
	cryptoapi.Finalize()
	err := cryptoapi.InitializeWithConfig(options)
	if err != nil {
		fmt.Printf("failed to initialize crypto, Error = [%s]\n", err.Error())
		log.GetLogger().Errorf("Initialize SCC Error = [%s]", err.Error())
		return false
	}
	sccInitialized = true
	return true
}

// FinalizeSCC -
func FinalizeSCC() {
	m.Lock()
	defer m.Unlock()
	sccInitialized = false
	cryptoapi.Finalize()
}

// SCCDecrypt -
func SCCDecrypt(cipher []byte) (string, error) {
	plain, err := cryptoapi.Decrypt(string(cipher))
	if err != nil {
		log.GetLogger().Errorf("SCC Decrypt Error = [%s]", err.Error())
		return "", err
	}

	return plain, nil
}

// SCCEncrypt -
func SCCEncrypt(plainInput string) ([]byte, error) {
	cipher, err := cryptoapi.Encrypt(plainInput)
	if err != nil {
		log.GetLogger().Errorf("SCC Encrypt Error = [%s]", err.Error())
		return nil, err
	}

	return []byte(cipher), nil
}
