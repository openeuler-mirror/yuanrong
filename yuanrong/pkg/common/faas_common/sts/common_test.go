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

// Package sts -
package sts

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/magiconair/properties"
	"github.com/smartystreets/goconvey/convey"
	"huawei.com/wisesecurity/sts-sdk/pkg/cloudsoa"
	"huawei.com/wisesecurity/sts-sdk/pkg/stsgoapi"

	"yuanrong/pkg/common/faas_common/sts/raw"
	"yuanrong/pkg/common/faas_common/tls"
	"yuanrong/pkg/common/faas_common/utils"
	mockUtils "yuanrong/pkg/common/faas_common/utils"
)

func TestGenerateSecretVolumeMounts(t *testing.T) {
	type args struct {
		systemFunctionName string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"case1 faasshceduler generate", args{systemFunctionName: FaaSSchedulerName}, false},
		{"case2 faasfrontend generate", args{systemFunctionName: FaasfrontendName}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			build := utils.NewVolumeBuilder()
			_, err := GenerateSecretVolumeMounts(tt.args.systemFunctionName, build)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateSecretVolumeMounts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestInitStsSDK(t *testing.T) {
	type args struct {
		serverCfg raw.ServerConfig
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		patchesFunc mockUtils.PatchesFunc
	}{
		{"case1", args{serverCfg: raw.ServerConfig{}}, false, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(stsgoapi.InitWith, func(property properties.Properties) error { return nil })})
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc((*cloudsoa.AESCryptorBuilder).Builder, func(
					_ *cloudsoa.AESCryptorBuilder) (*cloudsoa.AESCryptor, error) {
					return nil, nil
				})})
			return patches
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches := tt.patchesFunc()
			if err := InitStsSDK(tt.args.serverCfg); (err != nil) != tt.wantErr {
				t.Errorf("InitStsSDK() error = %v, wantErr %v", err, tt.wantErr)
			}
			patches.ResetAll()
		})
	}
}

func TestCustomKeyProvider(t *testing.T) {
	convey.Convey("test custom key provider", t, func() {
		provider := NewCustomKeyProvider("aaa", []byte("bbb"))
		convey.So(provider, convey.ShouldNotBeNil)
		key, i, err := provider.GetKey(cloudsoa.KeyLabel{})
		convey.So(string(key), convey.ShouldEqual, "bbb")
		convey.So(i, convey.ShouldEqual, 0)
		convey.So(err, convey.ShouldBeNil)
		key, err = provider.GetKeyWithVersion(cloudsoa.KeyLabel{}, 0)
		convey.So(string(key), convey.ShouldEqual, "bbb")
		convey.So(err, convey.ShouldBeNil)
		convey.So(provider.GetName(), convey.ShouldEqual, "aaa")
	})
}

func TestGenerateHTTPSAndLocalSecretVolumeMounts(t *testing.T) {
	convey.Convey("TestGenerateHTTPSAndLocalSecretVolumeMounts", t, func() {
		httpsConfig := tls.InternalHTTPSConfig{}
		volumeData, volumeMountData, err := GenerateHTTPSAndLocalSecretVolumeMounts(httpsConfig, nil)
		convey.So(volumeData, convey.ShouldEqual, "")
		convey.So(volumeMountData, convey.ShouldEqual, "")
		convey.So(err, convey.ShouldNotBeNil)

		volumeData, volumeMountData, err = GenerateHTTPSAndLocalSecretVolumeMounts(httpsConfig, utils.NewVolumeBuilder())
		convey.So(err, convey.ShouldBeNil)
	})
}
