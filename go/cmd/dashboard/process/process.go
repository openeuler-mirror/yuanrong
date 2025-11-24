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

// Package process for run dashboard server
package process

import (
	"fmt"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"yuanrong.org/kernel/pkg/common/faas_common/grpc/pb/logservice"
	"yuanrong.org/kernel/pkg/common/faas_common/logger/log"
	"yuanrong.org/kernel/pkg/dashboard/etcdcache"
	"yuanrong.org/kernel/pkg/dashboard/flags"
	"yuanrong.org/kernel/pkg/dashboard/logmanager"
	"yuanrong.org/kernel/pkg/dashboard/routers"
)

// StartGrpcServices of dashboard
func StartGrpcServices() {
	// start grpc service
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", flags.DashboardConfig.Ip, flags.DashboardConfig.GrpcPort))
	if err != nil {
		log.GetLogger().Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	logservice.RegisterLogManagerServiceServer(grpcServer, &logmanager.Server{})

	if err := grpcServer.Serve(lis); err != nil {
		log.GetLogger().Fatalf("failed to serve grpc: %s", err.Error())
	}
}

// StartDashboard - function for run dashboard server
func StartDashboard() {
	gin.SetMode(gin.ReleaseMode)
	stopCh := make(chan struct{})

	// init the etcd config first
	err := flags.InitEtcdClient()
	if err != nil {
		log.GetLogger().Fatalf("failed to init etcd, err: %s", err)
	}

	// register self
	err = flags.RegisterSelfToEtcd(stopCh)
	if err != nil {
		log.GetLogger().Fatalf("failed to register self to etcd, err: %s", err)
	}

	// start watcher
	etcdcache.StartWatchInstance(stopCh)

	// start grpc at background
	go StartGrpcServices()

	// start http, and use http as main thread
	r := routers.SetRouter()
	srv := &http.Server{
		Addr:    flags.DashboardConfig.ServerAddr,
		Handler: r,
	}
	log.GetLogger().Debugf("http://%s is running...", flags.DashboardConfig.ServerAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.GetLogger().Fatalf("srv.ListenAndServe: %s", err.Error())
	}
}
