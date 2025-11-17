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

const (
	cipherName = "cipher"
	sslName    = "ssl"
	kafkaName  = "kafkacert"
	authName   = "auth"
	etcdName   = "etcd"
	redisName  = "redisdb"
)

const (
	cipherSecretName = "cff-cipher-ssl-secret"
	sslSecretName    = "cff-cipher-ssl-secret"
	kafkaSecretName  = "cff-kafka-secret"
	authSecretName   = "cff-common-secret"
	etcdSecretName   = "cff-etcd-secret"
	redisSecretName  = "cff-rdb-secret"
)

var (
	secretMode      = int32(defaultSecretMode)
	cipherKeyToPath = []v1.KeyToPath{
		{
			Key:  "root.key",
			Path: "root.key",
			Mode: &secretMode,
		},
		{
			Key:  "common_shared.key",
			Path: "common_shared.key",
			Mode: &secretMode,
		},
	}
	sslKeyToPath = []v1.KeyToPath{
		{
			Key:  "ca.crt",
			Path: "trust.cer",
			Mode: &secretMode,
		},
		{
			Key:  "tls.crt",
			Path: "server.cer",
			Mode: &secretMode,
		},
		{
			Key:  "tls.key.pwd",
			Path: "server_key.pem",
			Mode: &secretMode,
		},
		{
			Key:  "pwd",
			Path: "cert_pwd",
			Mode: &secretMode,
		},
	}
	kafkaKeyToPath = []v1.KeyToPath{
		{
			Key:  "tls.crt",
			Path: "kafka.cer",
			Mode: &secretMode,
		},
		{
			Key:  "tls.key",
			Path: "kafka.key",
			Mode: &secretMode,
		},
		{
			Key:  "tls.ca",
			Path: "kafka.ca",
			Mode: &secretMode,
		},
	}
	authKeyToPath = []v1.KeyToPath{
		{
			Key:  "internal_sign_ak",
			Path: "internal_sign_ak",
			Mode: &secretMode,
		},
		{
			Key:  "internal_sign_sk",
			Path: "internal_sign_sk",
			Mode: &secretMode,
		},
	}
	etcdKeyToPath = []v1.KeyToPath{
		{
			Key:  "appversion",
			Path: "appversion.json",
			Mode: &secretMode,
		},
	}
	redisKeyToPath = []v1.KeyToPath{
		{
			Key:  "appversion",
			Path: "appversion.json",
			Mode: &secretMode,
		},
	}
)
