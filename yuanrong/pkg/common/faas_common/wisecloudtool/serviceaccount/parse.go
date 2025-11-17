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

// Package serviceaccount sign http request by jwttoken
package serviceaccount

import (
	"crypto/tls"
	"fmt"

	"github.com/json-iterator/go"

	"huawei.com/wisesecurity/sts-sdk/pkg/stsgoapi"

	"yuanrong/pkg/common/faas_common/wisecloudtool/types"
)

// ParseServiceAccount -
func ParseServiceAccount(serviceAccountKeyStr string) (*types.ServiceAccount, error) {
	if len(serviceAccountKeyStr) <= 0 {
		return nil, fmt.Errorf("serviceAccountKeyStr is empty")
	}

	decryptedByte, err := stsgoapi.DecryptSensitiveConfig(serviceAccountKeyStr)
	if err != nil {
		return nil, fmt.Errorf("decrypt service account key failed")
	}
	serviceAccount := &types.ServiceAccount{}
	err = jsoniter.Unmarshal(decryptedByte, &serviceAccount)
	if err != nil {
		return nil, fmt.Errorf("unmarshal service account key failed, err: %s", err.Error())
	}
	return serviceAccount, nil
}

// ParseTlsCipherSuites -
func ParseTlsCipherSuites(tlsCipherSuitesStrs []string) ([]uint16, error) {
	if len(tlsCipherSuitesStrs) <= 0 {
		return nil, fmt.Errorf("tlsCipherSuitesStr is empty")
	}

	return cipherSuitesID(cipherSuitesFromName(tlsCipherSuitesStrs)), nil
}

func cipherSuitesFromName(names []string) []*tls.CipherSuite {
	m := make(map[string]*tls.CipherSuite, len(tls.CipherSuites()))
	for _, cipher := range tls.CipherSuites() {
		m[cipher.Name] = cipher
	}

	r := make([]*tls.CipherSuite, 0)
	for _, n := range names {
		if _, ok := m[n]; ok {
			r = append(r, m[n])
		}
	}
	return r
}

func cipherSuitesID(cs []*tls.CipherSuite) []uint16 {
	ids := make([]uint16, 0)
	for _, value := range cs {
		ids = append(ids, value.ID)
	}
	return ids
}
