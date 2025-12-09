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

// This test file can also be used as a tool to create, encrypt and decrypt our secrets and cipher texts
package crypto

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSCCEncryptDecryptInitialized(t *testing.T) {
	var c = SccConfig{
		Enable:    true,
		Algorithm: "AES256_GCM",
	}
	ret := InitializeSCC(c)
	assert.False(t, ret)
	input := "text to encrypt"
	encrypted, err := SCCEncrypt(input)
	fmt.Printf("encrypted : %s\n", string(encrypted))
	assert.Nil(t, err)
	decrypt, err := SCCDecrypt(encrypted)
	fmt.Printf("decrypt : %s\n", decrypt)
	assert.Nil(t, err)
	assert.Equal(t, "", decrypt)
	assert.NotEqual(t, encrypted, input)
	FinalizeSCC()
}

func TestSCCEncryptDecryptNotInitialized(t *testing.T) {
	var c = SccConfig{
		Enable:    false,
		Algorithm: "AES256_GCM",
	}
	ret := InitializeSCC(c)
	assert.False(t, ret)
	input := "text to encrypt"
	encrypted, _ := SCCEncrypt(input)
	fmt.Printf("encrypted : %s\n", string(encrypted))
	decrypt, _ := SCCDecrypt(encrypted)
	fmt.Printf("decrypt : %s\n", decrypt)
	FinalizeSCC()
}

func TestSCCEncryptDecryptAlgorithms(t *testing.T) {
	var c = SccConfig{
		Enable:    true,
		Algorithm: "AES256_GCM",
	}

	algorithms := []string{"AES256_CBC", "AES128_GCM", "AES256_GCM", "SM4_CBC", "SM4_CTR", "DEFAULT"}
	for _, algo := range algorithms {
		FinalizeSCC()
		c.Algorithm = algo
		ret := InitializeSCC(c)
		assert.False(t, ret)
		input := "text to encrypt"
		encrypted, err := SCCEncrypt(input)
		fmt.Printf("encrypted : %s\n", string(encrypted))
		assert.Nil(t, err)
		decrypt, err := SCCDecrypt(encrypted)
		fmt.Printf("decrypt : %s\n", decrypt)
		assert.Nil(t, err)
		assert.Equal(t, "", decrypt)
		assert.NotEqual(t, encrypted, input)
	}
}
