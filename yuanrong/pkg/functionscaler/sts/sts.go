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
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"huawei.com/wisesecurity/sts-sdk/pkg/remote"
	"huawei.com/wisesecurity/sts-sdk/pkg/stsgoapi"

	"yuanrong/pkg/common/faas_common/snerror"
	"yuanrong/pkg/common/faas_common/statuscode"
	"yuanrong/pkg/functionscaler/config"
)

var (
	httpClient = &remote.StsKeyHttpClient{}
	once       sync.Once
)

func doStsRequest(httpRequest *remote.StsHttpRequest, httpClient *remote.StsKeyHttpClient) ([]byte, error) {
	newStsHTTPReq, _ := stsgoapi.SignRequest(httpRequest, serviceMeta)
	response, err := httpClient.SendMessage(newStsHTTPReq)
	if err != nil {
		return nil, fmt.Errorf("send message failed: %s", err.Error())
	}
	defer response.Body.Close()
	buf, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("io.ReadAll failed, error: %s, response body is %v", err.Error(), string(buf))
	}
	if response.StatusCode == http.StatusOK {
		return buf, nil
	}
	if response.StatusCode/100 == 4 { // 4xx
		errString := strings.ReplaceAll(string(buf), `"`, "")
		return nil, snerror.New(statuscode.StsConfigErrCode,
			"The requested parameter or permission is abnormal, statusCode is "+strconv.Itoa(response.StatusCode)+
				", err response is "+errString)
	}

	//  5xx... etc
	return nil, fmt.Errorf("http error, the code is %d, err response is %s", response.StatusCode, string(buf))
}

// GetStsHTTPClient create StsKeyHttpClient
func GetStsHTTPClient() *remote.StsKeyHttpClient {
	once.Do(func() {
		cfg := config.GlobalConfig
		rawHTTPClient := remote.NewInstanceOfHttpClient()
		httpClient = &remote.StsKeyHttpClient{
			StsServerHost: "http://" + cfg.RawStsConfig.MgmtServerConfig.Domain,
			MyServiceMeta: rawHTTPClient.MyServiceMeta,
			Cert:          rawHTTPClient.Cert,
			Signer:        rawHTTPClient.Signer,
			HttpClient:    rawHTTPClient.HttpClient,
			VerifyCert:    rawHTTPClient.VerifyCert,
		}
	})
	return httpClient
}
