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

// Package common prepares common constants, utils and structs for collector
package common

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"yuanrong.org/kernel/pkg/common/faas_common/etcd3"
	"yuanrong.org/kernel/pkg/common/faas_common/logger/log"
)

var (
	connection *grpc.ClientConn
	once       sync.Once
)

const (
	// DefaultGrpcTimeoutS -
	DefaultGrpcTimeoutS = 5 * time.Second
)

// GetConnection get grpc connection
func GetConnection() *grpc.ClientConn {
	once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultGrpcTimeoutS)
		defer cancel()
		log.GetLogger().Infof("start connect to log manager grpc server: %s", CollectorConfigs.ManagerAddress)
		conn, err := grpc.DialContext(ctx, CollectorConfigs.ManagerAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.GetLogger().Errorf("failed to connect to log manager grpc server, error: %v", err)
			return
		}
		log.GetLogger().Infof("success to connect to log manager: %s", CollectorConfigs.ManagerAddress)
		connection = conn
	})
	return connection
}

// InitEtcdClient will
func InitEtcdClient() error {
	return etcd3.InitParam().
		WithRouteEtcdConfig(CollectorConfigs.EtcdConfig).
		WithStopCh(make(chan struct{})).
		InitClient()
}
