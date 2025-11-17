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

// Package common for tools
package common

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/magiconair/properties"
	"github.com/smartystreets/goconvey/convey"

	"huawei.com/wisesecurity/sts-sdk/pkg/stsgoapi"
)

func TestGetConfig(t *testing.T) {
	convey.Convey(
		"Test Get config", t, func() {
			convey.Convey(
				"Test get config success", func() {
					conf := GetConfig()
					convey.So(conf.LogPath, convey.ShouldBeEmpty)
				},
			)
		},
	)
}

func TestLoadSTSConfig(t *testing.T) {
	convey.Convey(
		"Test loadSTSConfig", t, func() {
			convey.Convey(
				"Test loadSTSConfig when configPath == \"\"", func() {
					convey.So(func() {
						loadSTSConfig("")
					}, convey.ShouldNotPanic)
				},
			)
			file, _ := os.Create("config.json")
			convey.Convey(
				"Test loadSTSConfig when json.Unmarshal error", func() {
					convey.So(func() {
						loadSTSConfig("config.json")
					}, convey.ShouldNotPanic)
				},
			)
			c := &GlobalConfig{
				RawStsConfig: StsConfig{
					StsEnable: false,
					ServerConfig: ServerConfig{
						Domain: "244",
						Path:   "244",
					},
					SensitiveConfigs: SensitiveConfigs{
						Auth: map[string]string{
							"enableIam": "false",
						},
					},
				},
			}
			bytes, _ := json.Marshal(c)
			file.Write(bytes)
			convey.Convey(
				"Test loadSTSConfig when c.RawStsConfig.StsEnable == false", func() {
					convey.So(func() {
						loadSTSConfig("config.json")
					}, convey.ShouldNotPanic)
				},
			)
			c.RawStsConfig.StsEnable = true
			bytes, _ = json.Marshal(c)
			file, _ = os.OpenFile("config.json", os.O_WRONLY|os.O_TRUNC, 0644)
			file.Write(bytes)
			convey.Convey(
				"Test loadSTSConfig when stsgoapi.InitWith error", func() {
					convey.So(func() {
						loadSTSConfig("config.json")
					}, convey.ShouldNotPanic)
				},
			)

			convey.Convey(
				"Test enableIam is false", func() {
					convey.So(func() {
						defer gomonkey.ApplyFunc(stsgoapi.InitWith, func(property properties.Properties) error {
							return nil
						}).Reset()
						loadSTSConfig("config.json")
					}, convey.ShouldNotPanic)
				},
			)

			file.Close()
			os.Remove("config.json")
		},
	)
}
