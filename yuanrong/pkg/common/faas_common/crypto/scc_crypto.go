//go:build scc

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
	"encoding/json"
	"fmt"
	"path"
	"sync"

	corev1 "k8s.io/api/core/v1"

	"yuanrong/pkg/common/faas_common/logger/log"
	"yuanrong/pkg/common/faas_common/utils"
)

var (
	sccInitialized bool = false
	m              sync.RWMutex
)

const (
	// Aes256Cbc  -
	Aes256Cbc = "AES256_CBC"
	// Aes128Gcm  -
	Aes128Gcm = "AES128_GCM"
	// Aes256Gcm  -
	Aes256Gcm = "AES256_GCM"
	// Sm4Cbc     -
	Sm4Cbc = "SM4_CBC"
	// Sm4Ctr     -
	Sm4Ctr = "SM4_CTR"

	sccConfigDefaultPath = "/home/snuser/secret/scc"
)

// SccConfig -
type SccConfig struct {
	Enable        bool   `json:"enable" valid:"optional"`
	SecretName    string `json:"secretName" valid:"optional"`
	Algorithm     string `json:"algorithm" valid:"optional"`
	SccConfigPath string `json:"sccConfigPath" valid:"optional"`
}

// SCCInitialized -
func SCCInitialized() bool {
	m.RLock()
	defer m.RUnlock()
	return sccInitialized
}

// GetSCCAlgorithm -
func GetSCCAlgorithm(algorithm string) int {
	switch algorithm {
	case Aes256Cbc:
		return cryptoapi.ALG_AES256_CBC
	case Aes128Gcm:
		return cryptoapi.ALG_AES128_GCM
	case Aes256Gcm:
		return cryptoapi.ALG_AES256_GCM
	case Sm4Cbc:
		return cryptoapi.ALG_SM4_CBC
	case Sm4Ctr:
		return cryptoapi.ALG_SM4_CTR
	default:
		return cryptoapi.ALG_AES256_GCM
	}
}

// InitializeSCC -
func InitializeSCC(config SccConfig) error {
	m.Lock()
	defer m.Unlock()

	if !config.Enable {
		return nil
	}
	options := cryptoapi.NewSccOptions()
	sccConfigPath := config.SccConfigPath
	if sccConfigPath == "" {
		sccConfigPath = sccConfigDefaultPath
	}
	options.PrimaryKeyFile = path.Join(sccConfigPath, "primary.ks")
	options.StandbyKeyFile = path.Join(sccConfigPath, "standby.ks")
	options.LogPath = "/tmp/log/"
	options.LogFile = "scc"
	options.DefaultAlgorithm = GetSCCAlgorithm(config.Algorithm)
	options.RandomDevice = "/dev/random"
	options.EnableChangeFilePermission = 1
	cryptoapi.Finalize()
	err := cryptoapi.InitializeWithConfig(options)
	if err != nil {
		log.GetLogger().Errorf("Initialize SCC Error = [%s]", err.Error())
		return err
	}
	sccInitialized = true
	return nil
}

// FinalizeSCC -
func FinalizeSCC() {
	m.Lock()
	defer m.Unlock()
	sccInitialized = false
	cryptoapi.Finalize()
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

// SCCDecrypt -
func SCCDecrypt(cipher []byte) (string, error) {
	plain, err := cryptoapi.Decrypt(string(cipher))
	if err != nil {
		log.GetLogger().Errorf("SCC Decrypt Error = [%s]", err.Error())
		return "", err
	}

	return plain, nil
}

// GenerateSCCVolumesAndMounts -
func GenerateSCCVolumesAndMounts(secretName string, builder *utils.VolumeBuilder) (string, string, error) {
	if builder == nil {
		return "", "", fmt.Errorf("volume builder is nil")
	}
	builder.AddVolume(corev1.Volume{Name: "scc-ks",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}}})
	builder.AddVolumeMount(utils.ContainerRuntimeManager,
		corev1.VolumeMount{Name: "scc-ks", MountPath: "/home/snuser/resource/scc"})
	volumesData, err := json.Marshal(builder.Volumes)
	if err != nil {
		return "", "", err
	}
	volumesMountData, err := json.Marshal(builder.Mounts[utils.ContainerRuntimeManager])
	if err != nil {
		return "", "", err
	}
	return string(volumesData), string(volumesMountData), nil
}
