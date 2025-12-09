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

package raw

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	saltKeySep = ":"

	shareKey = "1752F862B5176946F18D45D67E256642F115D2D6A3D77773FAF1E5874AC5211D"

	plain = "{\"key1\":\"value1\",\"key2\":\"value2\"}"

	saltKey = "R8Mi3gSG3ou4X6eY:VIQASOEBJTQT3yd4qGrpqSbLrgemB5eTaD5KRefaOcXh/r18YSwhtv0j0A=="
	plain2  = "{\"key1\":\"va1\",\"key2\":\"va2\"}"
)

func TestAesGCMDecrypt(t *testing.T) {
	shareKey2 := make([]byte, hex.DecodedLen(len(shareKey)))
	_, err := hex.Decode(shareKey2, []byte(shareKey))
	if err != nil {
		t.Errorf("%s", err)
	}

	salt, cipherBytes, err := AesGCMEncrypt(shareKey2, []byte(plain))
	fmt.Println(string(salt), string(cipherBytes))
	saltBase64 := base64.StdEncoding.EncodeToString(salt)
	cipherBase64 := base64.StdEncoding.EncodeToString(cipherBytes)
	fmt.Println(saltBase64, cipherBase64)

	if err != nil {
		t.Errorf("%s", err)
	}
	blocks1, err := AesGCMDecrypt(shareKey2, salt, cipherBytes)

	assert.Equal(t, string(blocks1), plain)
}

func TestAesGCMDecrypt2(t *testing.T) {
	shareKey2 := make([]byte, hex.DecodedLen(len(shareKey)))
	_, err := hex.Decode(shareKey2, []byte(shareKey))
	if err != nil {
		t.Errorf("%s", err)
	}

	fields := strings.Split(saltKey, saltKeySep)
	salt1, err := base64.StdEncoding.DecodeString(fields[0])
	if err != nil {
		t.Errorf("%s", err)
	}
	cipher, err := base64.StdEncoding.DecodeString(fields[1])
	blocks, err := AesGCMDecrypt(shareKey2, salt1, cipher)
	if err != nil {
		t.Errorf("%s", err)
	}

	assert.Equal(t, string(blocks), plain2)
}
