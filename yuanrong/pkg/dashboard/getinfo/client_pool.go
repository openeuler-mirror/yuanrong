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

// Package getinfo for get function-master info
package getinfo

import (
	"io"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"yuanrong/pkg/common/faas_common/logger/log"
	"yuanrong/pkg/dashboard/flags"
)

// HttpClient is client pool
var HttpClient *http.Client

// PromClient is prometheus client pool
var PromClient prometheusv1.API

const reqType = "protobuf"

func init() {
	HttpClient = &http.Client{
		Timeout: 30 * time.Minute, // 连接超时时间
		Transport: &http.Transport{
			MaxIdleConns:        10,               // 最大空闲连接数
			MaxIdleConnsPerHost: 5,                // 每个主机最大空闲连接数
			MaxConnsPerHost:     10,               // 每个主机最大连接数
			IdleConnTimeout:     30 * time.Second, // 空闲连接的超时时间
			TLSHandshakeTimeout: 30 * time.Minute, // 限制TLS握手的时间
		},
	}
	InitPromClient()
}

// InitPromClient -
func InitPromClient() {
	apiClient, promClientErr := api.NewClient(api.Config{
		Address: flags.DashboardConfig.PrometheusAddr,
		Client:  HttpClient,
	})
	if promClientErr != nil {
		log.GetLogger().Errorf("failed to connect prometheus, error: %s", promClientErr.Error())
	} else {
		PromClient = prometheusv1.NewAPI(apiClient)
	}
}

func requestFunctionMaster(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", flags.DashboardConfig.FunctionMasterAddr+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Type", reqType)
	return handleRes(req)
}

func requestFrontend(method string, path string, reqBody io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, flags.DashboardConfig.FrontendAddr+path, reqBody)
	if err != nil {
		return nil, err
	}
	return handleRes(req)
}

func handleRes(req *http.Request) ([]byte, error) {
	resp, err := HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
