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

// Package localauth authenticates requests by local configmaps
package localauth

import (
	"errors"
	"sync"

	// Register aeswithkey engine
	_ "huaweicloud.com/containers/security/cbb_adapt/src/go/aeswithkey"
	"huaweicloud.com/containers/security/cbb_adapt/src/go/gcrypto"

	"yuanrong/pkg/common/faas_common/logger/log"
	"yuanrong/pkg/common/faas_common/utils"
)

var (
	gcryptoEngine gcrypto.Engine
	algorithm     = "aeswithkey"
	once          sync.Once
)

func initCrypto() error {
	engine, err := gcrypto.New(algorithm)
	if err != nil {
		log.GetLogger().Errorf("failed to initialize the crypto engine, error is %s", err.Error())
		return err
	}
	gcryptoEngine = engine
	return nil
}

// Decrypt decrypts a cypher text using a certain algorithm
func Decrypt(src string) ([]byte, error) {
	var err error
	once.Do(func() {
		err = initCrypto()
	})
	if gcryptoEngine == nil {
		return nil, err
	}

	plaintext, err := gcryptoEngine.Decrypt(0, src)
	if err != nil {
		// error message may contain some sensitive content which should not be printed
		return nil, errors.New("failed to decrypt the ciphertext")
	}
	text := []byte(plaintext)
	utils.ClearStringMemory(plaintext)
	return text, nil
}

// Encrypt encrypts a cypher text using a certain algorithm
func Encrypt(src string) (string, error) {
	var err error
	once.Do(func() {
		err = initCrypto()
	})
	if gcryptoEngine == nil {
		return "", errors.New("gcrypto engine is null")
	}

	ciperText, err := gcryptoEngine.Encrypt(0, src)
	if err != nil {
		return "", errors.New("failed to encrypt the data")
	}
	return ciperText, nil
}

// DecryptKeys decrypts a set of aKey and sKey
func DecryptKeys(inputAKey string, inputSKey string) ([]byte, []byte, error) {
	aKey, err := Decrypt(inputAKey)
	if err != nil {
		log.GetLogger().Errorf("failed to decrypt AKey, error: %s", err.Error())
		return nil, nil, err
	}
	sKey, err := Decrypt(inputSKey)
	if err != nil {
		log.GetLogger().Errorf("failed to decrypt SKey, error: %s", err.Error())
		return nil, nil, err
	}
	return aKey, sKey, nil
}
