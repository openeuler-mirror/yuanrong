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

// Package utils -
package utils

import (
	"testing"
	"time"

	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"

	"yuanrong.org/kernel/runtime/libruntime/api"

	"yuanrong.org/kernel/pkg/common/faas_common/constant"
	"yuanrong.org/kernel/pkg/common/faas_common/resspeckey"
	commonTypes "yuanrong.org/kernel/pkg/common/faas_common/types"
	"yuanrong.org/kernel/pkg/common/faas_common/utils"
	"yuanrong.org/kernel/pkg/functionscaler/config"
	"yuanrong.org/kernel/pkg/functionscaler/types"
)

func TestIsFaaSManager(t *testing.T) {
	convey.Convey("error funcKey", t, func() {
		is := IsFaaSManager("")
		convey.So(is, convey.ShouldBeFalse)
	})
	convey.Convey("right funcKey", t, func() {
		is := IsFaaSManager("0-system-faasmanager")
		convey.So(is, convey.ShouldBeTrue)
	})
}

func TestAddAffinityCPU(t *testing.T) {
	type args struct {
		crName            string
		schedulingOptions *types.SchedulingOptions
		resSpec           *resspeckey.ResourceSpecification
		affinityType      api.AffinityType
	}
	tests := []struct {
		name string
		args args
	}{
		{"case1", args{
			crName:            "yyrk1234-0-yrservice-test-image-env-call-latest-670698364",
			schedulingOptions: &types.SchedulingOptions{},
			resSpec:           &resspeckey.ResourceSpecification{CPU: 300, Memory: 128},
			affinityType:      api.PreferredAntiAffinity,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddAffinityCPU(tt.args.crName, tt.args.schedulingOptions, tt.args.resSpec, tt.args.affinityType)
		})
	}
}

func TestAppendInstanceTypeToInstanceResource(t *testing.T) {
	convey.Convey("test AppendInstanceTypeToInstanceResource", t, func() {
		convey.Convey("resSpec is nil", func() {
			AppendInstanceTypeToInstanceResource(nil, "")
		})
		convey.Convey("success", func() {
			resSpec := &resspeckey.ResourceSpecification{}
			AppendInstanceTypeToInstanceResource(resSpec, "NPU")
			convey.So(resSpec.CustomResourcesSpec["instanceType"], convey.ShouldEqual, "NPU")
		})
	})
}

func TestGetNpuTypeAndInstanceTypeFromStr(t *testing.T) {
	convey.Convey("Test GetNpuTypeAndInstanceTypeFromStr", t, func() {
		convey.Convey("Test GetNpuTypeAndInstanceTypeFromStr", func() {
			npuType, extraType := GetNpuTypeAndInstanceTypeFromStr(`{"huawei.com/ascend-1980":1}`,
				`{"instanceType":"280t"}`)
			convey.So(npuType, convey.ShouldEqual, "huawei.com/ascend-1980")
			convey.So(extraType, convey.ShouldEqual, "280t")
		})
	})
}

func TestAddNodeSelector(t *testing.T) {
	convey.Convey("Test AddNodeSelector", t, func() {
		convey.Convey("Test AddNodeSelector", func() {
			scheduleOption := &types.SchedulingOptions{Extension: make(map[string]string)}
			AddNodeSelector(map[string]string{"aaa": "123"}, scheduleOption, &resspeckey.ResourceSpecification{})
			convey.So(scheduleOption.Extension[utils.NodeSelectorKey], convey.ShouldEqual, `{"aaa": "123"}`)
		})
	})
}

func TestGetCreateTimeout(t *testing.T) {
	convey.Convey("test GetCreateTimeout", t, func() {
		convey.Convey("get create timeout", func() {
			timeout := GetCreateTimeout(&types.FunctionSpecification{})
			convey.So(timeout, convey.ShouldEqual, defaultCreateTimeout)
			timeout = GetCreateTimeout(&types.FunctionSpecification{
				ExtendedMetaData: commonTypes.ExtendedMetaData{
					Initializer: commonTypes.Initializer{
						Timeout: 50,
					},
				},
			})
			convey.So(timeout, convey.ShouldEqual, (50+constant.CommonExtraTimeout+constant.KernelScheduleTimeout)*time.Second)
			timeout = GetCreateTimeout(&types.FunctionSpecification{
				FuncMetaData: commonTypes.FuncMetaData{
					Runtime: types.CustomContainerRuntimeType,
				},
				ExtendedMetaData: commonTypes.ExtendedMetaData{
					Initializer: commonTypes.Initializer{
						Timeout: 50,
					},
				},
			})
			convey.So(timeout, convey.ShouldEqual, (50+constant.CustomImageExtraTimeout)*time.Second)
		})
	})
}

func TestGetLeaseInterval(t *testing.T) {
	convey.Convey("test GetLeaseInterval", t, func() {
		convey.Convey("get lease interval", func() {
			config.GlobalConfig.LeaseSpan = 50
			interval := GetLeaseInterval()
			convey.So(interval, convey.ShouldEqual, 500*time.Millisecond)
			config.GlobalConfig.LeaseSpan = 200
			interval = GetLeaseInterval()
			convey.So(interval, convey.ShouldEqual, 500*time.Millisecond)
			config.GlobalConfig.LeaseSpan = 600
			interval = GetLeaseInterval()
			convey.So(interval, convey.ShouldEqual, 600*time.Millisecond)
		})
	})
}

func TestBuildInstanceFromInsSpec(t *testing.T) {
	convey.Convey("test BuildInstanceFromInsSpec", t, func() {
		convey.Convey("get instance IP and port", func() {
			instance := BuildInstanceFromInsSpec(&commonTypes.InstanceSpecification{
				RuntimeAddress: "1.2.3.4:1234",
			}, nil)
			convey.So(instance.InstanceIP, convey.ShouldEqual, "1.2.3.4")
			convey.So(instance.InstancePort, convey.ShouldEqual, "1234")
		})
		convey.Convey("get instance type", func() {
			instance := BuildInstanceFromInsSpec(&commonTypes.InstanceSpecification{}, nil)
			convey.So(instance.InstanceType, convey.ShouldEqual, types.InstanceTypeUnknown)
			instance = BuildInstanceFromInsSpec(&commonTypes.InstanceSpecification{
				CreateOptions: map[string]string{
					types.InstanceTypeNote: string(types.InstanceTypeReserved),
				},
			}, nil)
			convey.So(instance.InstanceType, convey.ShouldEqual, types.InstanceTypeReserved)
		})
		convey.Convey("get scheduler id", func() {
			instance := BuildInstanceFromInsSpec(&commonTypes.InstanceSpecification{
				CreateOptions: map[string]string{
					types.SchedulerIDNote: "scheduler1-permanent",
				},
			}, nil)
			convey.So(instance.Permanent, convey.ShouldEqual, true)
			convey.So(instance.CreateSchedulerID, convey.ShouldEqual, "scheduler1")
			instance = BuildInstanceFromInsSpec(&commonTypes.InstanceSpecification{
				CreateOptions: map[string]string{
					types.SchedulerIDNote: "scheduler1-temporary",
				},
			}, nil)
			convey.So(instance.Permanent, convey.ShouldEqual, false)
			convey.So(instance.CreateSchedulerID, convey.ShouldEqual, "scheduler1")
		})
		convey.Convey("get concurrentNum", func() {
			instance := BuildInstanceFromInsSpec(&commonTypes.InstanceSpecification{
				CreateOptions: map[string]string{
					types.ConcurrentNumKey: "",
				},
			}, nil)
			convey.So(instance.ConcurrentNum, convey.ShouldEqual, 0)
			instance = BuildInstanceFromInsSpec(&commonTypes.InstanceSpecification{
				CreateOptions: map[string]string{
					types.ConcurrentNumKey: "wrong data",
				},
			}, nil)
			convey.So(instance.ConcurrentNum, convey.ShouldEqual, defaultConcurrentNum)
			instance = BuildInstanceFromInsSpec(&commonTypes.InstanceSpecification{
				CreateOptions: map[string]string{
					types.ConcurrentNumKey: "50",
				},
			}, nil)
			convey.So(instance.ConcurrentNum, convey.ShouldEqual, 50)
		})
		convey.Convey("get metricLabelValue ", func() {
			instance := BuildInstanceFromInsSpec(&commonTypes.InstanceSpecification{
				Extensions: commonTypes.Extensions{},
			}, nil)
			convey.So(instance.MetricLabelValues, convey.ShouldBeEmpty)
			instance = BuildInstanceFromInsSpec(&commonTypes.InstanceSpecification{
				Extensions: commonTypes.Extensions{
					PodName:           "aaa",
					PodNamespace:      "bbb",
					PodDeploymentName: "ccc",
				},
			}, &types.FunctionSpecification{
				FuncMetaData: commonTypes.FuncMetaData{
					Name:       "111",
					TenantID:   "222",
					BusinessID: "333",
					FuncName:   "444",
				},
			})
			convey.So(len(instance.MetricLabelValues), convey.ShouldEqual, 8)
		})
	})
}

func TestCheckInstanceSessionValid(t *testing.T) {
	convey.Convey("test CheckInstanceSessionValid", t, func() {
		convey.Convey("CheckInstanceSessionValid", func() {
			res := CheckInstanceSessionValid(commonTypes.InstanceSessionConfig{
				SessionID:  "_123&0",
				SessionTTL: 10,
			})
			convey.So(res, convey.ShouldEqual, true)
			res = CheckInstanceSessionValid(commonTypes.InstanceSessionConfig{
				SessionTTL: 10,
			})
			convey.So(res, convey.ShouldEqual, false)
			res = CheckInstanceSessionValid(commonTypes.InstanceSessionConfig{
				SessionID:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				SessionTTL: 10,
			})
			convey.So(res, convey.ShouldEqual, false)
			res = CheckInstanceSessionValid(commonTypes.InstanceSessionConfig{
				SessionID:  "aaa",
				SessionTTL: 0,
			})
			convey.So(res, convey.ShouldEqual, true)
		})
	})
}

func TestGetInvokeLabelFromResKey(t *testing.T) {
	convey.Convey("test GetInvokeLabelFromResKey", t, func() {
		convey.Convey("get GetInvokeLabelFromResKey", func() {
			res := resspeckey.ResourceSpecification{
				CPU:              500,
				Memory:           1000,
				InvokeLabel:      "aaaaa",
				EphemeralStorage: 0,
			}
			label := GetInvokeLabelFromResKey(res.String())
			convey.So(label, convey.ShouldEqual, "aaaaa")
		})
	})
}

func TestIntMax(t *testing.T) {
	assert.Equal(t, IntMax(3, 1), 3)
	assert.Equal(t, IntMax(1, 3), 3)
}
