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

// Package vpcmanager -
package vpcmanager

import "k8s.io/api/core/v1"

// Type volume type
type Type int

const (
	cipherVolumeType Type = iota
	sslVolumeType
	kafkaVolumeType
	authVolumeType
	etcdVolumeType
	redisVolumeType
)

func getTypes() []Type {
	return []Type{cipherVolumeType, sslVolumeType, kafkaVolumeType, authVolumeType, etcdVolumeType, redisVolumeType}
}

func (t Type) keyToPath() []v1.KeyToPath {
	return [...][]v1.KeyToPath{cipherKeyToPath, sslKeyToPath, kafkaKeyToPath, authKeyToPath, etcdKeyToPath,
		redisKeyToPath}[t]
}

func (t Type) secretName() string {
	return [...]string{cipherSecretName, sslSecretName, kafkaSecretName, authSecretName, etcdSecretName,
		redisSecretName}[t]
}

func (t Type) name() string {
	return [...]string{cipherName, sslName, kafkaName, authName, etcdName, redisName}[t]
}
