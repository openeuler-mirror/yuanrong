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

// Package cert parsing certificate
package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// LoadCerts - parsing certificate
func LoadCerts() (*x509.CertPool, *tls.Certificate, error) {
	return nil, nil, nil
}

func parseSTSCerts(pemBlocks []*pem.Block) ([][]byte, []byte, []byte, error) {
	var certByte, keyByte []byte
	var err error
	var caBytes [][]byte
	for _, pemBlock := range pemBlocks {
		pemEncoded := pem.EncodeToMemory(pemBlock)
		if pemBlock.Type == "PRIVATE KEY" {
			keyByte = pemEncoded
		} else {
			var cert *x509.Certificate
			if cert, err = x509.ParseCertificate(pemBlock.Bytes); err != nil {
				return nil, nil, nil, err
			}
			if cert == nil {
				return nil, nil, nil, fmt.Errorf("parse certificate err: cert is empty")
			}
			if cert.IsCA {
				pemBlock.Headers = map[string]string{}
				caBytes = append(caBytes, pem.EncodeToMemory(pemBlock))
			} else {
				certByte = append(certByte, pemEncoded...)
			}
		}
	}
	if len(caBytes) == 0 {
		return caBytes, certByte, keyByte, fmt.Errorf("ca certs not exists")
	}
	if len(certByte) == 0 {
		return caBytes, certByte, keyByte, fmt.Errorf("certs not exists")
	}
	if len(keyByte) == 0 {
		return caBytes, certByte, keyByte, fmt.Errorf("private key not exists")
	}
	return caBytes, certByte, keyByte, nil
}
