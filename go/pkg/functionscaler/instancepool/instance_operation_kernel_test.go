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

package instancepool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
	"yuanrong.org/kernel/runtime/libruntime/api"

	commonconstant "yuanrong.org/kernel/pkg/common/faas_common/constant"
	"yuanrong.org/kernel/pkg/common/faas_common/localauth"
	"yuanrong.org/kernel/pkg/common/faas_common/resspeckey"
	"yuanrong.org/kernel/pkg/common/faas_common/snerror"
	"yuanrong.org/kernel/pkg/common/faas_common/statuscode"
	"yuanrong.org/kernel/pkg/common/faas_common/sts/raw"
	commonTypes "yuanrong.org/kernel/pkg/common/faas_common/types"
	mockUtils "yuanrong.org/kernel/pkg/common/faas_common/utils"
	wisecloudTypes "yuanrong.org/kernel/pkg/common/faas_common/wisecloudtool/types"
	"yuanrong.org/kernel/pkg/functionscaler/config"
	"yuanrong.org/kernel/pkg/functionscaler/signalmanager"
	"yuanrong.org/kernel/pkg/functionscaler/types"
	"yuanrong.org/kernel/pkg/functionscaler/utils"
)

func TestCreateInstanceForKernel(t *testing.T) {
	SetGlobalSdkClient(&mockUtils.FakeLibruntimeSdkClient{})
	funcSpec := &types.FunctionSpecification{
		FuncKey:          "12345678901234561234567890123456/0-system-faasExecutor/$latest",
		FuncMetaData:     commonTypes.FuncMetaData{},
		InstanceMetaData: commonTypes.InstanceMetaData{},
	}
	convey.Convey("Test CreateInstanceForKernel", t, func() {
		convey.Convey("create success", func() {
			request := createInstanceRequest{
				funcSpec: funcSpec,
			}
			instance, err := createInstanceForKernel(request)
			convey.So(err, convey.ShouldBeNil)
			convey.So(instance, convey.ShouldNotBeNil)
		})
	})
}

func TestDealWithVPCError(t *testing.T) {
	convey.Convey("TestDealWithVPCError", t, func() {
		convey.Convey("VPCXRoleNotFound", func() {
			err := errors.New(statuscode.ErrVPCXRoleNotFound.Error())
			err = dealWithVPCError(err)
			var snErr snerror.SNError
			ok := errors.As(err, &snErr)
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(snErr.Error(), convey.ShouldEqual, fmt.Sprintf("VPC can't find xrole"))
		})
		convey.Convey("ErrNoOperationalPermissionsVpc", func() {
			err := errors.New(statuscode.ErrNoOperationalPermissionsVpc.Error())
			err = dealWithVPCError(err)
			var snErr snerror.SNError
			ok := errors.As(err, &snErr)
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(snErr.Error(), convey.ShouldEqual, statuscode.ErrNoOperationalPermissionsVpc.Error())
		})
		convey.Convey("NotEnoughNIC", func() {
			err := errors.New(statuscode.ErrNoAvailableVpcPatInstance.Error())
			err = dealWithVPCError(err)
			var snErr snerror.SNError
			ok := errors.As(err, &snErr)
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(snErr.Error(), convey.ShouldEqual, fmt.Sprintf("not enough network cards"))
		})
		convey.Convey("ErrVPCNotFound", func() {
			err := errors.New(statuscode.ErrVPCNotFound.Error())
			err = dealWithVPCError(err)
			var snErr snerror.SNError
			ok := errors.As(err, &snErr)
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(snErr.Error(), convey.ShouldEqual, fmt.Sprintf("VPC item not found"))
		})
		convey.Convey("NotVPCError", func() {
			err := errors.New("not a vpc error")
			createErr := dealWithVPCError(err)
			convey.So(createErr, convey.ShouldEqual, err)
		})
	})
}

func TestSetCreateOptionForDownloadData(t *testing.T) {
	convey.Convey("Test SetCreateOptionForDownloadData", t, func() {
		convey.Convey("create Opt is nil", func() {
			err := setCreateOptionForDownloadData(nil, nil)
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("SetCreateOptionForDownloadData success", func() {
			funcSpec := &types.FunctionSpecification{
				S3MetaData: commonTypes.S3MetaData{},
				FuncMetaData: commonTypes.FuncMetaData{
					Layers: []*commonTypes.Layer{{}},
				},
			}
			createOpt := map[string]string{}
			err := setCreateOptionForDownloadData(funcSpec, createOpt)
			convey.So(err, convey.ShouldBeNil)
		})
		convey.Convey(" marshal delegate download data error", func() {
			patch := ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				return []byte{}, errors.New("marshal error")
			})
			defer patch.Reset()
			funcSpec := &types.FunctionSpecification{
				S3MetaData: commonTypes.S3MetaData{},
			}
			createOpt := map[string]string{}
			err := setCreateOptionForDownloadData(funcSpec, createOpt)
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey(" marshal delegate layer download data error", func() {
			patch := ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				_, ok := v.(commonTypes.S3MetaData)
				if ok {
					return []byte{}, nil
				}
				return []byte{}, errors.New("marshal error")
			})
			defer patch.Reset()
			funcSpec := &types.FunctionSpecification{
				S3MetaData: commonTypes.S3MetaData{},
				FuncMetaData: commonTypes.FuncMetaData{
					Layers: []*commonTypes.Layer{{}},
				},
			}
			createOpt := map[string]string{}
			err := setCreateOptionForDownloadData(funcSpec, createOpt)
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func Test_setCreateOptionForLifeCycleDetached(t *testing.T) {
	createOpt := make(map[string]string)
	err := setCreateOptionForLifeCycleDetached(nil, createOpt)
	if err != nil {
		t.Errorf("do setCreateOptionForLifeCycleDetached failed, err: %s", err.Error())
		return
	}
	lifeCycle := createOpt[commonconstant.InstanceLifeCycle]
	if lifeCycle != "detached" {
		t.Errorf("do setCreateOptionForLifeCycleDetached failed, lifeCyle = %s", lifeCycle)
		return
	}
}

func Test_setCreateOptionForNuwaRuntimeInfo(t *testing.T) {
	rawGConfig := config.GlobalConfig
	config.GlobalConfig = types.Configuration{
		ClusterID: "cluster001",
		Scenario:  types.ScenarioWiseCloud,
	}
	defer func() {
		config.GlobalConfig = rawGConfig
	}()
	type args struct {
		funcSpec        *types.FunctionSpecification
		nuwaRuntimeInfo *wisecloudTypes.NuwaRuntimeInfo
		createOpt       map[string]string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{"case1 map is nil",
			args{
				funcSpec:        &types.FunctionSpecification{},
				nuwaRuntimeInfo: &wisecloudTypes.NuwaRuntimeInfo{},
				createOpt:       nil,
			},
			nil,
		},
		{
			"case2 succeeded to marshal ers workload config",
			args{
				funcSpec: &types.FunctionSpecification{},
				nuwaRuntimeInfo: &wisecloudTypes.NuwaRuntimeInfo{
					WisecloudRuntimeId:     "runtimeId",
					WisecloudSite:          "site",
					WisecloudTenantId:      "tenant",
					WisecloudApplicationId: "application",
					WisecloudServiceId:     "serviceid",
					WisecloudEnvironmentId: "environment",
					EnvLabel:               "label",
				},
				createOpt: map[string]string{},
			},
			map[string]string{"DELEGATE_NUWA_RUNTIME_INFO": `{"wisecloudRuntimeId":"runtimeId","wisecloudSite":"site","wisecloudTenantId":"tenant","wisecloudApplicationId":"application","wisecloudServiceId":"serviceid","wisecloudEnvironmentId":"environment","envLabel":"label"}`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if setCreateOptionForNuwaRuntimeInfo(tt.args.nuwaRuntimeInfo,
				tt.args.createOpt); !reflect.DeepEqual(tt.args.createOpt, tt.want) {
				t.Errorf("setCreateOptionForDelegateMount() = %v, want %v", tt.args.createOpt, tt.want)
			}
		})
	}
}

func Test_setCreateOptionForMountVolume(t *testing.T) {
	type args struct {
		funcSpec  *types.FunctionSpecification
		createOpt map[string]string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{"case1 map is nil", args{
			funcSpec:  &types.FunctionSpecification{},
			createOpt: nil,
		}, nil},
		{"case2 succeeded to marshal func mount config", args{
			funcSpec: &types.FunctionSpecification{ExtendedMetaData: commonTypes.ExtendedMetaData{
				FuncMountConfig: &commonTypes.FuncMountConfig{
					FuncMountUser: commonTypes.FuncMountUser{
						UserID:  1004,
						GroupID: 1004,
					},
					FuncMounts: []commonTypes.FuncMount{commonTypes.FuncMount{
						MountType:      "ecs",
						MountResource:  "eb4ebf7a-db82-4602-82ce-7e1e57a8ef46",
						MountSharePath: "1.1.1.1:/sharerdata",
						LocalMountPath: "/home/",
						Status:         "active",
					}},
				}}},
			createOpt: map[string]string{"test": "test"},
		}, map[string]string{"test": "test",
			"DELEGATE_MOUNT": `{"mount_user":{"user_id":1004,"user_group_id":1004},"func_mounts":[{"mount_type":"ecs","mount_resource":"eb4ebf7a-db82-4602-82ce-7e1e57a8ef46","mount_share_path":"1.1.1.1:/sharerdata","local_mount_path":"/home/","status":"active"}]}`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if setCreateOptionForDelegateMount(tt.args.funcSpec,
				tt.args.createOpt); !reflect.DeepEqual(tt.args.createOpt, tt.want) {
				t.Errorf("setCreateOptionForDelegateMount() = %v, want %v", tt.args.createOpt, tt.want)
			}
		})
	}
}

func Test_setCreateOptionForRASP(t *testing.T) {
	rawGConfig := config.GlobalConfig
	config.GlobalConfig = types.Configuration{
		Scenario: types.ScenarioWiseCloud,
	}
	defer func() {
		config.GlobalConfig = rawGConfig
	}()
	convey.Convey("test setCreateOptionForRASP", t, func() {
		convey.Convey("createOpt is nil", func() {
			funcSpec := &types.FunctionSpecification{
				ExtendedMetaData: commonTypes.ExtendedMetaData{
					RaspConfig: commonTypes.RaspConfig{
						InitImage:      "test-initImage",
						RaspImage:      "test-image",
						RaspServerIP:   "127.0.0.1",
						RaspServerPort: "8080",
					},
				},
			}
			err := setCreateOptionForRASP(funcSpec, nil)
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("initContainerAdd error", func() {
			defer ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				return nil, fmt.Errorf("marshal error")
			}).Reset()
			funcSpec := &types.FunctionSpecification{
				ExtendedMetaData: commonTypes.ExtendedMetaData{
					RaspConfig: commonTypes.RaspConfig{
						InitImage:      "test-initImage",
						RaspImage:      "test-image",
						RaspServerIP:   "127.0.0.1",
						RaspServerPort: "8080",
					},
				},
			}
			createOpt := make(map[string]string)
			err := setCreateOptionForRASP(funcSpec, createOpt)
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func TestSetCreateOptionForContainerSideCar(t *testing.T) {
	config.GlobalConfig.Scenario = types.ScenarioWiseCloud
	convey.Convey("Test SetCreateOptionForContainerSideCar", t, func() {
		convey.Convey("createOpt is nil", func() {
			err := setCreateOptionForFileBeat(nil, nil)
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("json Marshal CustomFilebeatConfig error", func() {
			patch := ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				return []byte{}, errors.New("marshal error")
			})
			defer patch.Reset()
			funcSpec := &types.FunctionSpecification{
				ExtendedMetaData: commonTypes.ExtendedMetaData{},
			}
			createOpt := map[string]string{}
			err := setCreateOptionForFileBeat(funcSpec, createOpt)
			convey.So(err, convey.ShouldNotBeNil)

		})
		convey.Convey("json Marshal config error", func() {
			patch := ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				return []byte{}, errors.New("marshal error")
			})
			defer patch.Reset()
			funcSpec := &types.FunctionSpecification{
				ExtendedMetaData: commonTypes.ExtendedMetaData{
					CustomFilebeatConfig: commonTypes.CustomFilebeatConfig{
						ImageAddress: "image",
					},
				},
			}
			createOpt := map[string]string{}
			err := setCreateOptionForFileBeat(funcSpec, createOpt)
			convey.So(err, convey.ShouldNotBeNil)

		})
		convey.Convey("success", func() {
			funcSpec := &types.FunctionSpecification{
				ExtendedMetaData: commonTypes.ExtendedMetaData{
					CustomFilebeatConfig: commonTypes.CustomFilebeatConfig{
						ImageAddress: "image",
					},
				},
			}
			createOpt := map[string]string{}
			err := setCreateOptionForFileBeat(funcSpec, createOpt)
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func Test_setCustomPodSeccompProfile(t *testing.T) {
	convey.Convey("test setCustomPodSeccompProfile", t, func() {
		rawGConfig := config.GlobalConfig
		config.GlobalConfig = types.Configuration{
			Scenario: types.ScenarioWiseCloud,
		}
		defer func() {
			config.GlobalConfig = rawGConfig
		}()
		convey.Convey("creatOpt is nil", func() {
			err := setCustomPodSeccompProfile(nil, nil)
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("runtime is not CustomContainer", func() {
			createOpt := map[string]string{}
			funcSpec := &types.FunctionSpecification{
				FuncMetaData: commonTypes.FuncMetaData{
					Runtime: "java",
				},
			}
			err := setCustomPodSeccompProfile(funcSpec, createOpt)
			convey.So(err, convey.ShouldBeNil)
		})
		convey.Convey("json Marsha data error", func() {
			defer ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				return nil, fmt.Errorf("marshal error")
			}).Reset()
			createOpt := map[string]string{}
			funcSpec := &types.FunctionSpecification{
				FuncMetaData: commonTypes.FuncMetaData{
					Runtime: types.CustomContainerRuntimeType,
				},
			}
			err := setCustomPodSeccompProfile(funcSpec, createOpt)
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("set success", func() {
			createOpt := map[string]string{}
			funcSpec := &types.FunctionSpecification{
				FuncMetaData: commonTypes.FuncMetaData{
					Runtime: types.CustomContainerRuntimeType,
				},
			}
			err := setCustomPodSeccompProfile(funcSpec, createOpt)
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func Test_setFunctionAgentInitContainer(t *testing.T) {
	convey.Convey("test setFunctionAgentInitContainer", t, func() {
		rawGConfig := config.GlobalConfig
		config.GlobalConfig = types.Configuration{
			Scenario: types.ScenarioWiseCloud,
		}
		defer func() {
			config.GlobalConfig = rawGConfig
		}()
		convey.Convey("creatOpt is nil", func() {
			err := setFunctionAgentInitContainer(nil, nil)
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("runtime is not CustomContainer", func() {
			createOpt := map[string]string{}
			funcSpec := &types.FunctionSpecification{
				FuncMetaData: commonTypes.FuncMetaData{
					Runtime: "java",
				},
			}
			err := setFunctionAgentInitContainer(funcSpec, createOpt)
			convey.So(err, convey.ShouldBeNil)
		})
		convey.Convey("json Marsha data error", func() {
			defer ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				return nil, fmt.Errorf("marshal error")
			}).Reset()
			createOpt := map[string]string{}
			funcSpec := &types.FunctionSpecification{
				FuncMetaData: commonTypes.FuncMetaData{
					Runtime: types.CustomContainerRuntimeType,
				},
			}
			err := setFunctionAgentInitContainer(funcSpec, createOpt)
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("set success", func() {
			createOpt := map[string]string{}
			funcSpec := &types.FunctionSpecification{
				FuncMetaData: commonTypes.FuncMetaData{
					Runtime: types.CustomContainerRuntimeType,
				},
			}
			err := setFunctionAgentInitContainer(funcSpec, createOpt)
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func Test_setCreateOptionForInitContainerEnv(t *testing.T) {
	convey.Convey("test setCreateOptionForInitContainerEnv", t, func() {
		funcSpec := &types.FunctionSpecification{
			ResourceMetaData: commonTypes.ResourceMetaData{
				CustomResources:     "{\"huawei.com/ascend-1980\":8}",
				CustomResourcesSpec: "{\"instanceType\":\"376T\"}",
			},
		}
		convey.Convey("d910b generate ranktable file", func() {
			createOpt := map[string]string{}

			err := setCreateOptionForInitContainerEnv(funcSpec, createOpt)
			convey.So(err, convey.ShouldBeNil)
			convey.So(strings.Contains(createOpt[commonconstant.DelegateInitEnv], "GENERATE_RANKTABLE_FILE"),
				convey.ShouldEqual, true)
			convey.So(strings.Contains(createOpt[commonconstant.DelegateInitEnv], "true"), convey.ShouldEqual, true)
		})
		convey.Convey("no generate ranktable file", func() {
			createOpt := map[string]string{}
			funcSpec.ResourceMetaData = commonTypes.ResourceMetaData{}
			err := setCreateOptionForInitContainerEnv(funcSpec, createOpt)
			convey.So(err, convey.ShouldBeNil)
			convey.So(strings.Contains(createOpt[commonconstant.DelegateInitEnv], "GENERATE_RANKTABLE_FILE"),
				convey.ShouldEqual, false)
			convey.So(strings.Contains(createOpt[commonconstant.DelegateInitEnv], "true"), convey.ShouldEqual, false)
		})
		convey.Convey("no generate ranktable file 1", func() {
			createOpt := map[string]string{}
			funcSpec.ResourceMetaData = commonTypes.ResourceMetaData{
				CustomResources:     "{\"huawei.com/ascend-1980123\":8}",
				CustomResourcesSpec: "{\"instanceType\":\"376T\"}",
			}
			err := setFunctionAgentInitContainer(funcSpec, createOpt)
			convey.So(err, convey.ShouldBeNil)
			convey.So(strings.Contains(createOpt[commonconstant.DelegateInitEnv], "GENERATE_RANKTABLE_FILE"),
				convey.ShouldEqual, false)
			convey.So(strings.Contains(createOpt[commonconstant.DelegateInitEnv], "true"), convey.ShouldEqual, false)
		})
	})
}

func Test_setCreateOptionForLabel(t *testing.T) {
	type args struct {
		funcSpec     *types.FunctionSpecification
		createOpt    map[string]string
		resSpec      *resspeckey.ResourceSpecification
		instanceType types.InstanceType
	}
	tests := []struct {
		name    string
		args    args
		wantNil bool
	}{
		{"case1 map is nil", args{
			funcSpec:  &types.FunctionSpecification{},
			createOpt: nil,
		}, true},
		{"case2 succeeded to set createOption for label", args{
			funcSpec: &types.FunctionSpecification{FuncMetaData: commonTypes.FuncMetaData{FuncName: "test",
				TenantID: "tenantID", Service: "serviceID", Version: "$latest"}},
			createOpt:    map[string]string{},
			resSpec:      &resspeckey.ResourceSpecification{CPU: 500, Memory: 500},
			instanceType: types.InstanceTypeReserved,
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setCreateOptionForLabel(tt.args.instanceType, tt.args.funcSpec, tt.args.resSpec, tt.args.createOpt)
			if !reflect.DeepEqual(tt.args.createOpt == nil, tt.wantNil) {
				t.Errorf("setCreateOptionForLabel() = %v, want %v", tt.args.createOpt, tt.wantNil)
			}
		})
	}
}

func TestSetCreateOptionForNodeAffinity(t *testing.T) {
	getMockFuncSpec := func(customResource string, customResourcesSpec string) *types.FunctionSpecification {
		return &types.FunctionSpecification{
			ResourceMetaData: commonTypes.ResourceMetaData{
				CustomResources:     customResource,
				CustomResourcesSpec: customResourcesSpec,
			},
		}
	}

	config.GlobalConfig.XpuNodeLabels = []types.XpuNodeLabel{
		types.XpuNodeLabel{
			XpuType:      "huawei.com/ascend-1980",
			InstanceType: "376T",
			NodeLabelKey: "node.kubernetes.io/instance-type",
			NodeLabelValues: []string{
				"physical.kat2ne.48xlarge.8.376t.ei.c002.ondemand",
				"physical.kat2ne.48xlarge.8.ei.pod101.ondemand",
			},
		},
		types.XpuNodeLabel{
			XpuType:      "huawei.com/ascend-1980",
			InstanceType: "",
			NodeLabelKey: "node.kubernetes.io/instance-type",
			NodeLabelValues: []string{
				"physical.kat2ne.48xlarge.8.376t.ei.c002.ondemand",
				"physical.kat2ne.48xlarge.8.ei.pod101.ondemand",
			},
		},
		types.XpuNodeLabel{
			XpuType:      "huawei.com/ascend-1980",
			InstanceType: "280T",
			NodeLabelKey: "node.kubernetes.io/instance-type",
			NodeLabelValues: []string{
				"physical.kat2e.48xlarge.8.280t.ei.c005.ondemand",
			},
		},
	}

	tests := []struct {
		name                       string
		customResource             string
		customResourcesSpec        string
		delegateNodeAffinity       string
		delegateNodeAffinityPolicy string
	}{
		{
			name:                "376T",
			customResource:      "{\"huawei.com/ascend-1980\":8}",
			customResourcesSpec: "{\"instanceType\":\"376T\"}",
			delegateNodeAffinity: "{" +
				"\"requiredDuringSchedulingIgnoredDuringExecution\": {" +
				"	\"nodeSelectorTerms\": [{" +
				"		\"matchExpressions\": [{" +
				"			\"key\": \"node.kubernetes.io/instance-type\"," +
				"			\"operator\": \"In\"," +
				"			\"values\": [\"physical.kat2ne.48xlarge.8.376t.ei.c002.ondemand\"," +
				"				\"physical.kat2ne.48xlarge.8.ei.pod101.ondemand\"]" +
				"		}]" +
				"	}]" +
				"}" +
				"}",
			delegateNodeAffinityPolicy: commonconstant.DelegateNodeAffinityPolicyAggregation,
		},
		{
			name:                "280T",
			customResource:      "{\"huawei.com/ascend-1980\":8}",
			customResourcesSpec: "{\"instanceType\":\"280T\"}",
			delegateNodeAffinity: "{" +
				"\"requiredDuringSchedulingIgnoredDuringExecution\": {" +
				"	\"nodeSelectorTerms\": [{" +
				"		\"matchExpressions\": [{" +
				"			\"key\": \"node.kubernetes.io/instance-type\"," +
				"			\"operator\": \"In\"," +
				"			\"values\": [\"physical.kat2e.48xlarge.8.280t.ei.c005.ondemand\"]" +
				"		}]" +
				"	}]" +
				"}" +
				"}",
			delegateNodeAffinityPolicy: commonconstant.DelegateNodeAffinityPolicyAggregation,
		},
		{
			name:           "376T_1",
			customResource: "{\"huawei.com/ascend-1980\":8}",
			delegateNodeAffinity: "{" +
				"\"requiredDuringSchedulingIgnoredDuringExecution\": {" +
				"	\"nodeSelectorTerms\": [{" +
				"		\"matchExpressions\": [{" +
				"			\"key\": \"node.kubernetes.io/instance-type\"," +
				"			\"operator\": \"In\"," +
				"			\"values\": [\"physical.kat2ne.48xlarge.8.376t.ei.c002.ondemand\"," +
				"				\"physical.kat2ne.48xlarge.8.ei.pod101.ondemand\"]" +
				"		}]" +
				"	}]" +
				"}" +
				"}",
			delegateNodeAffinityPolicy: commonconstant.DelegateNodeAffinityPolicyAggregation,
		},
		{
			name:                 "nil",
			customResource:       "",
			delegateNodeAffinity: "",
		},
		{
			name:                 "error",
			customResource:       "{\"instanceType\":\"28\"}",
			delegateNodeAffinity: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := make(map[string]string)
			e := setCreateOptionForAscendNPU(getMockFuncSpec(tt.customResource, tt.customResourcesSpec),
				&resspeckey.ResourceSpecification{CPU: 500, Memory: 500,
					CustomResources: map[string]int64{"huawei.com/ascend-1980": 8}}, m)
			if e != nil {
				t.Errorf("setCreateOptionForAscendNPU failed, err: %v", e)
			}
			actualNodeAffinity, ok := m["DELEGATE_NODE_AFFINITY"]
			if tt.delegateNodeAffinity == "" && !ok {
				return
			}
			if tt.delegateNodeAffinity == "" || !ok {
				t.Errorf("actual nodeAffinity is %s, expect Affinity is %s", actualNodeAffinity,
					tt.delegateNodeAffinity)
			}
			actualJson := make(map[string]interface{})
			expectJson := make(map[string]interface{})

			e1 := json.Unmarshal([]byte(actualNodeAffinity), &actualJson)
			e2 := json.Unmarshal([]byte(tt.delegateNodeAffinity), &expectJson)
			if e1 != nil || e2 != nil {
				t.Errorf("json.Unmarshal failed, err1: %v, actualNodeAffinity: %s, err2: %s, expectNodeAffinity: %s",
					e1, actualNodeAffinity, e2, tt.delegateNodeAffinity)
			}

			if !reflect.DeepEqual(actualJson, expectJson) {
				t.Errorf("deepEqual failed, actualNodeAffinity: %s, expectNodeAffinity: %s", actualNodeAffinity,
					tt.delegateNodeAffinity)
			}

			if m[commonconstant.DelegateNodeAffinityPolicy] != tt.delegateNodeAffinityPolicy {
				t.Errorf("deepEqual failed, actualNodeAffinityPolicy: %s, expectNodeAffinityPolicy: %s",
					m[commonconstant.DelegateNodeAffinityPolicy], tt.delegateNodeAffinityPolicy)
			}
			fmt.Printf("actual NodeAffinity: %s, expect NodeAffinity: %s", actualNodeAffinity, tt.delegateNodeAffinity)
		})
	}

}

func Test_PrepareCreateArguments(t *testing.T) {
	convey.Convey("test Test_PrepareCreateArguments", t, func() {
		convey.Convey("json marshal error", func() {
			defer ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				return nil, fmt.Errorf("marshal error")
			}).Reset()
			arg := prepareCreateArguments(createInstanceRequest{
				funcSpec: &types.FunctionSpecification{},
				resKey:   resspeckey.ResSpecKey{},
			})
			convey.So(arg, convey.ShouldBeNil)
		})
		convey.Convey("prepareCreateParamsData error", func() {
			defer ApplyFunc(prepareCreateParamsData, func(funcSpec *types.FunctionSpecification) ([]byte, error) {
				return nil, fmt.Errorf("prepareCreateParamsData error")
			}).Reset()
			funcSpec := &types.FunctionSpecification{
				ExtendedMetaData: commonTypes.ExtendedMetaData{
					RaspConfig: commonTypes.RaspConfig{
						InitImage:      "test-initImage",
						RaspServerIP:   "127.0.0.1",
						RaspServerPort: "8080",
					},
				},
			}
			arg := prepareCreateArguments(createInstanceRequest{
				funcSpec: funcSpec,
				resKey:   resspeckey.ResSpecKey{},
			})
			convey.So(arg, convey.ShouldBeNil)
		})
		convey.Convey("prepareSchedulerArg error", func() {
			defer ApplyFunc(prepareCreateParamsData, func(funcSpec *types.FunctionSpecification) ([]byte, error) {
				return []byte("createParamsData"), nil
			}).Reset()
			defer ApplyFunc(signalmanager.PrepareSchedulerArg, func() ([]byte, error) {
				return nil, fmt.Errorf("prepareSchedulerArg error")
			}).Reset()
			funcSpec := &types.FunctionSpecification{
				ExtendedMetaData: commonTypes.ExtendedMetaData{
					RaspConfig: commonTypes.RaspConfig{
						InitImage:      "test-initImage",
						RaspServerIP:   "127.0.0.1",
						RaspServerPort: "8080",
					},
				},
			}
			arg := prepareCreateArguments(createInstanceRequest{
				funcSpec: funcSpec,
				resKey:   resspeckey.ResSpecKey{},
			})
			convey.So(arg, convey.ShouldBeNil)
		})
		convey.Convey("prepareCustomUserArg error", func() {
			rawGConfig := config.GlobalConfig
			config.GlobalConfig = types.Configuration{
				Scenario: types.ScenarioWiseCloud,
			}
			defer func() {
				config.GlobalConfig = rawGConfig
			}()
			defer ApplyFunc(prepareCreateParamsData, func(funcSpec *types.FunctionSpecification) ([]byte, error) {
				return []byte("createParamsData"), nil
			}).Reset()
			defer ApplyFunc(signalmanager.PrepareSchedulerArg, func() ([]byte, error) {
				return []byte("prepareSchedulerArg"), nil
			}).Reset()
			defer ApplyFunc(prepareCustomUserArg, func(funcSpec *types.FunctionSpecification) ([]byte, error) {
				return nil, fmt.Errorf("prepareSchedulerArg error")
			}).Reset()
			funcSpec := &types.FunctionSpecification{
				ExtendedMetaData: commonTypes.ExtendedMetaData{
					RaspConfig: commonTypes.RaspConfig{
						InitImage:      "test-initImage",
						RaspServerIP:   "127.0.0.1",
						RaspServerPort: "8080",
					},
				},
			}
			arg := prepareCreateArguments(createInstanceRequest{
				funcSpec: funcSpec,
				resKey:   resspeckey.ResSpecKey{},
			})
			convey.So(arg, convey.ShouldNotBeNil)
		})
		convey.Convey("success", func() {
			rawGConfig := config.GlobalConfig
			config.GlobalConfig = types.Configuration{
				Scenario: types.ScenarioWiseCloud,
			}
			defer func() {
				config.GlobalConfig = rawGConfig
			}()
			defer ApplyFunc(prepareCreateParamsData, func(funcSpec *types.FunctionSpecification) ([]byte, error) {
				return []byte("createParamsData"), nil
			}).Reset()
			defer ApplyFunc(signalmanager.PrepareSchedulerArg, func() ([]byte, error) {
				return []byte("prepareSchedulerArg"), nil
			}).Reset()
			defer ApplyFunc(prepareCustomUserArg, func(funcSpec *types.FunctionSpecification) ([]byte, error) {
				return []byte("prepareCustomUserArg"), nil
			}).Reset()
			funcSpec := &types.FunctionSpecification{
				ExtendedMetaData: commonTypes.ExtendedMetaData{
					RaspConfig: commonTypes.RaspConfig{
						InitImage:      "test-initImage",
						RaspServerIP:   "127.0.0.1",
						RaspServerPort: "8080",
					},
				},
			}
			arg := prepareCreateArguments(createInstanceRequest{
				funcSpec: funcSpec,
				resKey:   resspeckey.ResSpecKey{},
			})
			convey.So(len(arg), convey.ShouldEqual, 4)
		})
	})
}

func Test_PrepareCustomUserArg(t *testing.T) {
	rawGConfig := config.GlobalConfig
	config.GlobalConfig = types.Configuration{
		RawStsConfig: raw.StsConfig{
			ServerConfig: raw.ServerConfig{},
		},
		LocalAuth: localauth.AuthConfig{},
	}
	defer func() {
		config.GlobalConfig = rawGConfig
	}()
	convey.Convey("test PrepareCustomUserArg", t, func() {
		convey.Convey("json Marshal error", func() {
			defer ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				return nil, errors.New("json marshal error")
			}).Reset()
			_, err := prepareCustomUserArg(&types.FunctionSpecification{})
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("success", func() {
			_, err := prepareCustomUserArg(&types.FunctionSpecification{})
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func Test_addInstanceCallerPodName(t *testing.T) {
	convey.Convey("test setCreateOptionForName", t, func() {
		convey.Convey("Unmarshal labels data error", func() {
			createOpt := map[string]string{
				commonconstant.DelegatePodLabels: "",
			}
			err := setCreateOptionForName("", "callerPodName", createOpt)
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("json Marsha data error", func() {
			defer ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				return nil, fmt.Errorf("marshal error")
			}).Reset()
			createOpt := map[string]string{
				commonconstant.DelegatePodLabels: "{\"funcName\":\"testcustom1024001\"}",
			}
			err := setCreateOptionForName("", "callerPodName", createOpt)
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("success", func() {
			createOpt := map[string]string{
				commonconstant.DelegatePodLabels: "{\"funcName\":\"testcustom1024001\"}",
			}
			err := setCreateOptionForName("", "callerPodName", createOpt)
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func Test_SetEphemeralStorage(t *testing.T) {
	convey.Convey("test setEphemeralStorage", t, func() {
		convey.Convey("resourcesMap is nil", func() {
			setEphemeralStorage(1, 0, nil)
			resourcesMap := make(map[string]float64)
			setEphemeralStorage(1, 1, resourcesMap)
			convey.So(resourcesMap[resourcesEphemeralStorage], convey.ShouldEqual, 1)
		})
	})
}

func TestGetNpuInstanceType(t *testing.T) {
	tests := []struct {
		name               string
		customResource     string
		customResourcesPec string
		npuType            string
		npuInstanceType    string
	}{
		{
			name:               "376T",
			customResource:     "{\"huawei.com/ascend-1980\":8}",
			customResourcesPec: "{\"instanceType\":\"376T\"}",
			npuType:            "huawei.com/ascend-1980",
			npuInstanceType:    "376T",
		},
		{
			name:               "280T",
			customResource:     "{\"huawei.com/ascend-1980\":8}",
			customResourcesPec: "{\"instanceType\":\"280T\"}",
			npuType:            "huawei.com/ascend-1980",
			npuInstanceType:    "280T",
		},
		{
			name:               "376T_1",
			customResource:     "{\"huawei.com/ascend-1980\":8}",
			customResourcesPec: "",
			npuType:            "huawei.com/ascend-1980",
			npuInstanceType:    "376T",
		},
		{
			name:               "nil",
			customResource:     "",
			customResourcesPec: "{\"instanceType\":\"280\"}",
			npuType:            "",
			npuInstanceType:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if npuType, npuInstanceType := utils.GetNpuTypeAndInstanceTypeFromStr(tt.customResource,
				tt.customResourcesPec); npuInstanceType != tt.npuInstanceType || npuType != tt.npuType {
				t.Errorf("failed, actual npuType: %s npuInstanceType: %s, expect npuType: %s npuInstanceType: %s",
					npuType, npuInstanceType, tt.npuType, tt.npuInstanceType)
			}
		})
	}
}

func TestGetNpuTypeAndInstanceType(t *testing.T) {
	tests := []struct {
		name                string
		customResource      map[string]int64
		customResourcesSpec map[string]interface{}
		npuType             string
		npuInstanceType     string
	}{
		{
			name:                "376T",
			customResource:      map[string]int64{"huawei.com/ascend-1980": 8},
			customResourcesSpec: map[string]interface{}{"instanceType": "376T"},
			npuType:             "huawei.com/ascend-1980",
			npuInstanceType:     "376T",
		},
		{
			name:                "280T",
			customResource:      map[string]int64{"huawei.com/ascend-1980": 8},
			customResourcesSpec: map[string]interface{}{"instanceType": "280T"},
			npuType:             "huawei.com/ascend-1980",
			npuInstanceType:     "280T",
		},
		{
			name:            "376T_1",
			customResource:  map[string]int64{"huawei.com/ascend-1980": 8},
			npuType:         "huawei.com/ascend-1980",
			npuInstanceType: "376T",
		},
		{
			name:            "nil",
			npuType:         "",
			npuInstanceType: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if npuType, npuInstanceType := utils.GetNpuTypeAndInstanceType(tt.customResource,
				tt.customResourcesSpec); npuType != tt.npuType || npuInstanceType != tt.npuInstanceType {
				t.Errorf("failed, actual npuType: %s npuInstanceType: %s, expect npuType: %s npuInstanceType: %s",
					npuType, npuInstanceType, tt.npuType, tt.npuInstanceType)
			}
		})
	}
}

func TestInitCustomContainerEnvForNpu(t *testing.T) {
	getMockFuncSpec := func(customResource string, customResourcesSpec string) *types.FunctionSpecification {
		return &types.FunctionSpecification{
			FuncMetaData: commonTypes.FuncMetaData{
				Name:               "0@default@zyrfunction3",
				FunctionVersionURN: "sn:cn:yrk:172120022624845016:function:0@default@zyrfunction3:1",
				FunctionURN:        "sn:cn:yrk:172120022624845016:function:0@default@zyrfunction3",
			},
			ResourceMetaData: commonTypes.ResourceMetaData{
				CustomResources:     customResource,
				CustomResourcesSpec: customResourcesSpec,
			},
		}
	}

	rawGConfig := config.GlobalConfig
	config.GlobalConfig = types.Configuration{
		RegionName: "12324234",
	}
	defer func() {
		config.GlobalConfig = rawGConfig
	}()

	tests := []struct {
		name                string
		customResource      string
		customResourcesSpec string
		nodeNpuInstanceType string
	}{
		{
			name:                "376T",
			customResource:      "{\"huawei.com/ascend-1980\":8}",
			customResourcesSpec: "{\"instanceType\":\"376T\"}",
			nodeNpuInstanceType: "376T",
		},
		{
			name:                "280T",
			customResource:      "{\"huawei.com/ascend-1980\":8}",
			customResourcesSpec: "{\"instanceType\":\"280T\"}",
			nodeNpuInstanceType: "280T",
		},
		{
			name:                "376T_1",
			customResource:      "{\"huawei.com/ascend-1980\":8}",
			customResourcesSpec: "{\"instanceType\":\"376T\"}",
			nodeNpuInstanceType: "376T",
		},
		{
			name:                "nil",
			customResource:      "",
			nodeNpuInstanceType: "",
		},
		{
			name:                "error",
			customResource:      "{\"instanceType\":\"280\"}",
			nodeNpuInstanceType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envs := initCustomContainerEnv(getMockFuncSpec(tt.customResource, tt.customResourcesSpec))
			actualNodeInstanceType := ""
			//		actualNodeInstanceTypeFromMeta := ""
			for _, env := range envs {
				if env.Name == "X_SYSTEM_NODE_INSTANCE_TYPE" {
					actualNodeInstanceType = env.Value
				}
				if env.Name == "X_SYSTEM_NODE_INSTANCE_TYPE_IN_META_CUSTOM_RESOURCE" {
					//				actualNodeInstanceTypeFromMeta = env.Value
				}
			}
			if actualNodeInstanceType != tt.nodeNpuInstanceType {
				t.Errorf("failed, actual nodeInstanceType is %s, expect nodeInstanceType is %s", actualNodeInstanceType,
					tt.nodeNpuInstanceType)
			}
		})

	}
}

func TestSetCreateOptionForVPC(t *testing.T) {
	convey.Convey("Test TestSetCreateOptionForVPC", t, func() {
		convey.Convey(" marshal network config error", func() {
			patch := ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				return []byte{}, errors.New("marshal error")
			})
			defer patch.Reset()
			natConfig := &types.NATConfigure{}
			setCreateOptionForVPC(nil, natConfig)
		})
		convey.Convey(" marshal prober config error", func() {
			patch := ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				_, ok := v.([]types.NetworkConfig)
				if ok {
					return []byte{}, nil
				}
				return []byte{}, errors.New("marshal error")
			})
			defer patch.Reset()
			natConfig := &types.NATConfigure{}
			setCreateOptionForVPC(nil, natConfig)
		})
	})
}

func TestGenerateSnErrorFromKernelError(t *testing.T) {
	convey.Convey("Test GenerateSnErrorFromKernelError", t, func() {
		convey.Convey("json Unmarshal error", func() {
			kernelErr := errors.New("code:3003,message: ")
			snError := generateSnErrorFromKernelError(kernelErr)
			convey.So(snError.Code(), convey.ShouldEqual, statuscode.InternalErrorCode)

		})
		convey.Convey("errorCode error", func() {
			initRsp := &types.ExecutorInitResponse{}
			data, _ := json.Marshal(initRsp)
			kernelErr := errors.New(fmt.Sprintf("code:3003,message: %s", string(data)))
			snError := generateSnErrorFromKernelError(kernelErr)
			convey.So(snError.Code(), convey.ShouldEqual, statuscode.InternalErrorCode)

		})
		convey.Convey("message MarshalJSON error", func() {
			initRsp := &types.ExecutorInitResponse{
				ErrorCode: "1",
				Message:   json.RawMessage{},
			}
			data, _ := json.Marshal(initRsp)
			kernelErr := errors.New(fmt.Sprintf("code:3003,message: %s", string(data)))
			snError := generateSnErrorFromKernelError(kernelErr)
			convey.So(snError.Code(), convey.ShouldEqual, statuscode.InternalErrorCode)
		})
		convey.Convey("KernelUserCodeLoadErrCode", func() {
			initRsp := &types.ExecutorInitResponse{
				ErrorCode: "2001",
			}
			data, _ := json.Marshal(initRsp)
			kernelErr := errors.New(fmt.Sprintf("code:2001,message: %s", string(data)))
			snError := generateSnErrorFromKernelError(kernelErr)
			convey.So(snError.Code(), convey.ShouldEqual, 4001)
		})
		convey.Convey("success", func() {
			initRsp := &types.ExecutorInitResponse{
				ErrorCode: "1",
			}
			data, _ := json.Marshal(initRsp)
			kernelErr := errors.New(fmt.Sprintf("code:3003,message: %s", string(data)))
			snError := generateSnErrorFromKernelError(kernelErr)
			convey.So(snError.Code(), convey.ShouldEqual, 1)
		})
	})
}

func Test_setCreateOptionForPodInitLabel(t *testing.T) {
	type args struct {
		funcSpec     *types.FunctionSpecification
		resSpec      *resspeckey.ResourceSpecification
		instanceType types.InstanceType
	}
	tests := []struct {
		name          string
		args          args
		podLabels     map[string]string
		podInitLabels map[string]string
		tenantId      string
	}{
		{"case1 map is nil",
			args{
				funcSpec: &types.FunctionSpecification{},
			},
			map[string]string{
				podLabelInstanceType: "",
				podLabelFuncName:     "",
				podLabelIsPoolPod:    "false",
				podLabelServiceID:    "",
				podLabelTenantID:     "",
				podLabelVersion:      "",
			},
			map[string]string{
				podLabelSecurityGroup: "",
			},
			"",
		},
		{"case2 succeeded to set createOption for label",
			args{
				funcSpec: &types.FunctionSpecification{FuncMetaData: commonTypes.FuncMetaData{FuncName: "test",
					TenantID: "tenantID", Service: "serviceID", Version: "$latest"},
					FuncKey: "12345678901234561234567890123456/test/$latest"},
				resSpec:      &resspeckey.ResourceSpecification{CPU: 500, Memory: 500},
				instanceType: types.InstanceTypeReserved,
			},
			map[string]string{
				podLabelInstanceType: string(types.InstanceTypeReserved),
				podLabelFuncName:     "test",
				podLabelIsPoolPod:    "false",
				podLabelServiceID:    "serviceID",
				podLabelTenantID:     "12345678901234561234567890123456",
				podLabelVersion:      "latest",
				podLabelStandard:     "500-500-fusion",
			},
			map[string]string{
				podLabelSecurityGroup: "12345678901234561234567890123456",
			},
			"12345678901234561234567890123456",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createOpt := make(map[string]string)
			err := setCreateOptionForLabel(tt.args.instanceType, tt.args.funcSpec, tt.args.resSpec, createOpt)
			if err != nil {
				t.Errorf("setCreateOptionForLabel failed, err: %s", err.Error())
				return
			}
			err = setCreateOptionForNote(tt.args.instanceType, tt.args.funcSpec, tt.args.resSpec, createOpt)
			if err != nil {
				t.Errorf("setCreateOptionForNote failed, err: %s", err.Error())
				return
			}
			if _, ok := createOpt[commonconstant.DelegatePodLabels]; !ok {
				t.Errorf("no deletegate pod labels")
				return
			}
			if _, ok := createOpt[commonconstant.DelegatePodInitLabels]; !ok {
				t.Errorf("no deletegate pod init labels")
				return
			}
			podLabels := make(map[string]string)
			podInitLabels := make(map[string]string)
			err1 := json.Unmarshal([]byte(createOpt[commonconstant.DelegatePodLabels]), &podLabels)
			err2 := json.Unmarshal([]byte(createOpt[commonconstant.DelegatePodInitLabels]), &podInitLabels)
			if err1 != nil || err2 != nil {
				t.Errorf("unmarshal failed")
				return
			}
			if !reflect.DeepEqual(podInitLabels, tt.podInitLabels) {
				t.Errorf("setCreateOptionForLabel() = %v, want %v", podInitLabels, tt.podInitLabels)
				return
			}
			if !reflect.DeepEqual(podLabels, tt.podLabels) {
				t.Errorf("setCreateOptionForLabel() = %v, want %v", podLabels, tt.podLabels)
				return
			}
			if createOpt[types.TenantID] != tt.tenantId {
				t.Errorf("setCreateOptionForLabel failed, tenantId set failed, actual: %s, expect: %s",
					createOpt[types.TenantID], tt.tenantId)
			}
		})
	}
}

func TestGetExecutorFuncKey(t *testing.T) {
	gi := &GenericInstancePool{
		FuncSpec: &types.FunctionSpecification{
			FuncMetaData: commonTypes.FuncMetaData{},
		},
	}
	convey.Convey("python3.6", t, func() {
		gi.FuncSpec.FuncMetaData.Runtime = "python3.6"
		funcKey := getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual,
			"12345678901234561234567890123456/0-system-faasExecutorPython3.6/$latest")
	})
	convey.Convey("python3.8", t, func() {
		gi.FuncSpec.FuncMetaData.Runtime = "python3.8"
		funcKey := getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual,
			"12345678901234561234567890123456/0-system-faasExecutorPython3.8/$latest")
	})
	convey.Convey("python3.9", t, func() {
		gi.FuncSpec.FuncMetaData.Runtime = "python3.9"
		funcKey := getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual,
			"12345678901234561234567890123456/0-system-faasExecutorPython3.9/$latest")
	})
	convey.Convey("python3.10", t, func() {
		gi.FuncSpec.FuncMetaData.Runtime = "python3.10"
		funcKey := getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual,
			"12345678901234561234567890123456/0-system-faasExecutorPython3.10/$latest")
	})
	convey.Convey("python3.11", t, func() {
		gi.FuncSpec.FuncMetaData.Runtime = "python3.11"
		funcKey := getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual,
			"12345678901234561234567890123456/0-system-faasExecutorPython3.11/$latest")
	})
	convey.Convey("go, http, custom", t, func() {
		gi.FuncSpec.FuncMetaData.Runtime = "go"
		funcKey := getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual, "12345678901234561234567890123456/0-system-faasExecutorGo1.x/$latest")
		gi.FuncSpec.FuncMetaData.Runtime = "http"
		funcKey = getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual, "12345678901234561234567890123456/0-system-faasExecutorGo1.x/$latest")
		gi.FuncSpec.FuncMetaData.Runtime = "custom image"
		funcKey = getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual, "12345678901234561234567890123456/0-system-faasExecutorGo1.x/$latest")
	})
	convey.Convey("java8", t, func() {
		gi.FuncSpec.FuncMetaData.Runtime = "java8"
		funcKey := getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual, "12345678901234561234567890123456/0-system-faasExecutorJava8/$latest")
	})
	convey.Convey("java11", t, func() {
		gi.FuncSpec.FuncMetaData.Runtime = "java11"
		funcKey := getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual, "12345678901234561234567890123456/0-system-faasExecutorJava11/$latest")
	})
	convey.Convey("java17", t, func() {
		gi.FuncSpec.FuncMetaData.Runtime = "java17"
		funcKey := getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual, "12345678901234561234567890123456/0-system-faasExecutorJava17/$latest")
	})
	convey.Convey("java21", t, func() {
		gi.FuncSpec.FuncMetaData.Runtime = "java21"
		funcKey := getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual, "12345678901234561234567890123456/0-system-faasExecutorJava21/$latest")
	})
	convey.Convey("posix-custom-runtime", t, func() {
		gi.FuncSpec.FuncMetaData.Runtime = "posix-custom-runtime"
		funcKey := getExecutorFuncKey(gi.FuncSpec)
		convey.So(funcKey, convey.ShouldEqual,
			"12345678901234561234567890123456/0-system-faasExecutorPosixCustom/$latest")
	})
}

func TestDeleteInstanceForKernel(t *testing.T) {
	convey.Convey("Test DeleteInstance", t, func() {
		config.GlobalConfig.InstanceOperationBackend = commonconstant.BackendTypeKernel
		funcSpec := &types.FunctionSpecification{
			FuncKey: "testFunction",
			FuncCtx: context.TODO(),
			FuncMetaData: commonTypes.FuncMetaData{
				Handler: "myHandler",
			},
			ResourceMetaData: commonTypes.ResourceMetaData{
				CPU:    500,
				Memory: 500,
			},
			InstanceMetaData: commonTypes.InstanceMetaData{
				MaxInstance:   100,
				ConcurrentNum: 100,
			},
			ExtendedMetaData: commonTypes.ExtendedMetaData{
				Initializer: commonTypes.Initializer{
					Handler: "myInitializer",
				},
			},
		}
		instance := &types.Instance{
			InstanceID: "testInstance",
		}
		convey.Convey("delete instance with global client", func() {
			SetGlobalSdkClient(&mockUtils.FakeLibruntimeSdkClient{})
			err := deleteInstanceForKernel(funcSpec, faasManagerInfo{}, instance)
			convey.So(err, convey.ShouldBeNil)
			SetGlobalSdkClient(nil)
		})
	})
}

func TestCreateInvokeOptions(t *testing.T) {
	funcSpec := &types.FunctionSpecification{
		FuncKey: "testVpcFuncKey",
		FuncMetaData: commonTypes.FuncMetaData{
			VPCTriggerImage: "vpc trigger image url",
		},
		ExtendedMetaData: commonTypes.ExtendedMetaData{},
	}
	opt := createInvokeOptions(funcSpec, &types.SchedulingOptions{}, nil, "")
	assert.NotNil(t, opt)
}

func Test_getStsServerConfig(t *testing.T) {
	convey.Convey("test getStsServerConfig", t, func() {
		convey.Convey("baseline", func() {
			funcSpec := &types.FunctionSpecification{
				StsMetaData: commonTypes.StsMetaData{
					EnableSts:    true,
					ServiceName:  "a",
					MicroService: "b",
				},
			}
			config.GlobalConfig.RawStsConfig.ServerConfig.Domain = "12345"
			config.GlobalConfig.RawStsConfig.StsDomainForRuntime = ""
			serverConfig := getStsServerConfig(funcSpec)
			convey.So(serverConfig.Domain, convey.ShouldEqual, "12345")
			convey.So(serverConfig.Path, convey.ShouldEqual, "/opt/huawei/certs/a/b/b.ini")
			config.GlobalConfig.RawStsConfig.StsDomainForRuntime = "67890"
			serverConfig = getStsServerConfig(funcSpec)
			convey.So(serverConfig.Domain, convey.ShouldEqual, "67890")
			convey.So(serverConfig.Path, convey.ShouldEqual, "/opt/huawei/certs/a/b/b.ini")
		})
	})
}

func TestCreatePATService_InvokeError(t *testing.T) {
	funcSpec := &types.FunctionSpecification{}

	faasManagerInfo := faasManagerInfo{
		funcKey:    "faas-manager-func-key",
		instanceID: "faas-manager-instance-id",
	}
	extMetaData := commonTypes.ExtendedMetaData{}
	vpcConfig := &commonTypes.VpcConfig{}

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	sdk := &mockUtils.FakeLibruntimeSdkClient{}
	SetGlobalSdkClient(sdk)
	patches.ApplyMethod(reflect.TypeOf(sdk), "InvokeByInstanceId",
		func(_ *mockUtils.FakeLibruntimeSdkClient, funcMeta api.FunctionMeta, instanceID string, args []api.Arg,
			invokeOpt api.InvokeOptions) (string, error) {
			return "", errors.New("invoke error")
		})
	natConfig, err := createPATService(funcSpec, faasManagerInfo, extMetaData, vpcConfig)

	assert.NotNil(t, err)
	assert.Nil(t, natConfig)
	assert.Equal(t, "failed to create pat service", err.Error())
	faasManagerInfo.instanceID = ""

	natConfig, err = createPATService(funcSpec, faasManagerInfo, extMetaData, vpcConfig)
	assert.Nil(t, natConfig)
	assert.Equal(t, "failed to create pat service", err.Error())
}

func TestCreatePATService_Success(t *testing.T) {
	funcSpec := &types.FunctionSpecification{
		FuncKey: "test-func-key",
	}
	faasManagerInfo := faasManagerInfo{
		funcKey:    "faas-manager-func-key",
		instanceID: "faas-manager-instance-id",
	}
	extMetaData := commonTypes.ExtendedMetaData{}
	vpcConfig := &commonTypes.VpcConfig{
		VpcID: "test-vpc-id",
	}

	patches := gomonkey.NewPatches()
	defer patches.Reset()

	sdk := &mockUtils.FakeLibruntimeSdkClient{}
	SetGlobalSdkClient(sdk)

	patches.ApplyFunc(prepareCreatePATServiceArguments,
		func(extMetaData commonTypes.ExtendedMetaData, vpcConfig *commonTypes.VpcConfig) []api.Arg {
			return []api.Arg{}
		})

	patches.ApplyFunc(utils.GenerateTraceID, func() string {
		return "test-trace-id"
	})

	patches.ApplyMethod(reflect.TypeOf(sdk), "InvokeByInstanceId",
		func(_ *mockUtils.FakeLibruntimeSdkClient, funcMeta api.FunctionMeta, instanceID string, args []api.Arg,
			invokeOpts api.InvokeOptions) (string, error) {
			return "test-obj-id", nil
		})

	patches.ApplyMethod(reflect.TypeOf(sdk), "GetAsync",
		func(_ *mockUtils.FakeLibruntimeSdkClient, objectID string, cb api.GetAsyncCallback) {
			response := &patSvcCreateResponse{
				Code:    0,
				Message: `{"containerCidr":"10.0.0.0/24"}`,
			}
			responseData, _ := json.Marshal(response)
			cb(responseData, nil)
		})

	_, err := createPATService(funcSpec, faasManagerInfo, extMetaData, vpcConfig)

	assert.Equal(t, err.Error(), "failed to create pat service")
}

func TestDeleteInstanceWithVPCSuccess(t *testing.T) {
	funcSpec := &types.FunctionSpecification{
		FuncKey: "test-func",
	}
	managerInfo := faasManagerInfo{
		funcKey:    "manager-key",
		instanceID: "manager-instance",
	}
	instance := &types.Instance{
		InstanceID: "test-instance",
	}

	t.Run("success_case", func(t *testing.T) {
		sdk := &mockUtils.FakeLibruntimeSdkClient{}
		SetGlobalSdkClient(sdk)

		var getAsyncCalled bool

		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyMethod(reflect.TypeOf(sdk), "InvokeByInstanceId",
			func(_ *mockUtils.FakeLibruntimeSdkClient, funcMeta api.FunctionMeta, instanceID string, args []api.Arg,
				invokeOpts api.InvokeOptions) (string, error) {
				return "test-obj-id", nil
			})

		patches.ApplyMethod(reflect.TypeOf(sdk), "GetAsync",
			func(_ *mockUtils.FakeLibruntimeSdkClient, objectID string, cb api.GetAsyncCallback) {
				getAsyncCalled = true
				cb([]byte("success"), nil)
			})

		deleteInstanceWithVPC(funcSpec, managerInfo, instance)

		assert.NotNil(t, getAsyncCalled, "GetAsync should be called")
	})
}

func TestDeleteInstanceWithVPC(t *testing.T) {
	funcSpec := &types.FunctionSpecification{
		FuncKey: "test-func",
	}
	managerInfo := faasManagerInfo{
		funcKey:    "manager-key",
		instanceID: "manager-instance",
	}
	instance := &types.Instance{
		InstanceID: "test-instance",
	}

	t.Run("get_async_error", func(t *testing.T) {
		sdk := &mockUtils.FakeLibruntimeSdkClient{}
		SetGlobalSdkClient(sdk)

		getAsyncCalled := false

		defer gomonkey.ApplyMethod(reflect.TypeOf(sdk), "InvokeByInstanceId",
			func(_ *mockUtils.FakeLibruntimeSdkClient, funcMeta api.FunctionMeta, instanceID string, args []api.Arg,
				invokeOpt api.InvokeOptions) (string, error) {
				return "mock-obj-id", nil
			}).Reset()

		defer gomonkey.ApplyMethod(reflect.TypeOf(sdk), "GetAsync",
			func(_ *mockUtils.FakeLibruntimeSdkClient, objectID string, cb api.GetAsyncCallback) {
				getAsyncCalled = true
				cb(nil, errors.New("get async error"))
			}).Reset()

		deleteInstanceWithVPC(funcSpec, managerInfo, instance)
		assert.NotNil(t, getAsyncCalled, "GetAsync should be called with error")
	})
}

func TestHandlePullTriggerCreate(t *testing.T) {
	funcSpec := &types.FunctionSpecification{
		FuncKey: "test-func",
		ExtendedMetaData: commonTypes.ExtendedMetaData{
			Initializer: commonTypes.Initializer{
				Handler: "myInitializer",
			},
			VpcConfig: &commonTypes.VpcConfig{},
		},
	}
	managerInfo := faasManagerInfo{
		funcKey:    "manager-key",
		instanceID: "manager-instance",
	}

	t.Run("success_case", func(t *testing.T) {
		sdk := &mockUtils.FakeLibruntimeSdkClient{}
		SetGlobalSdkClient(sdk)

		var getAsyncCalled bool
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyMethod(reflect.TypeOf(sdk), "InvokeByInstanceId",
			func(_ *mockUtils.FakeLibruntimeSdkClient, funcMeta api.FunctionMeta, instanceID string, args []api.Arg,
				invokeOpt api.InvokeOptions) (string, error) {
				return "mock-obj-id", nil
			})

		patches.ApplyMethod(reflect.TypeOf(sdk), "GetAsync",
			func(_ *mockUtils.FakeLibruntimeSdkClient, objectID string, cb api.GetAsyncCallback) {
				getAsyncCalled = true
				cb([]byte("success"), nil)
			})

		handlePullTriggerCreate(managerInfo, funcSpec)

		assert.NotNil(t, getAsyncCalled, "GetAsync should be called")
	})

	t.Run("invoke_error", func(t *testing.T) {
		sdk := &mockUtils.FakeLibruntimeSdkClient{}
		SetGlobalSdkClient(sdk)
		var getAsyncCalled bool

		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyMethod(reflect.TypeOf(sdk), "InvokeByInstanceId",
			func(_ *mockUtils.FakeLibruntimeSdkClient, funcMeta api.FunctionMeta, instanceID string, args []api.Arg,
				invokeOpt api.InvokeOptions) (string, error) {
				getAsyncCalled = true
				return "", errors.New("invoke error")
			})

		handlePullTriggerCreate(managerInfo, funcSpec)
		assert.NotNil(t, getAsyncCalled, "GetAsync should be called")
	})

	t.Run("get_async_error", func(t *testing.T) {
		sdk := &mockUtils.FakeLibruntimeSdkClient{}
		SetGlobalSdkClient(sdk)

		var getAsyncCalled bool
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyMethod(reflect.TypeOf(sdk), "InvokeByInstanceId",
			func(_ *mockUtils.FakeLibruntimeSdkClient, funcMeta api.FunctionMeta, instanceID string, args []api.Arg,
				invokeOpt api.InvokeOptions) (string, error) {
				return "mock-obj-id", nil
			})

		patches.ApplyMethod(reflect.TypeOf(sdk), "GetAsync",
			func(_ *mockUtils.FakeLibruntimeSdkClient, objectID string, cb api.GetAsyncCallback) {
				getAsyncCalled = true
				cb(nil, errors.New("get async error"))
			})

		handlePullTriggerCreate(managerInfo, funcSpec)

		assert.NotNil(t, getAsyncCalled, "GetAsync should be called")
	})
}

func TestReportInstanceWithVPC(t *testing.T) {
	tests := []struct {
		name            string
		funcSpec        *types.FunctionSpecification
		faasManagerInfo faasManagerInfo
		instance        *types.Instance
		natConfig       *types.NATConfigure
		invokeErr       error
		getAsyncErr     error
		expectGetAsync  bool
	}{
		{
			name: "successful_report",
			funcSpec: &types.FunctionSpecification{
				FuncKey: "test-func",
				ExtendedMetaData: commonTypes.ExtendedMetaData{
					Initializer: commonTypes.Initializer{
						Handler: "myInitializer",
					},
					VpcConfig: &commonTypes.VpcConfig{},
				},
			},
			faasManagerInfo: faasManagerInfo{
				funcKey:    "faas-key",
				instanceID: "instance-1",
			},
			instance: &types.Instance{
				InstanceID: "test-instance",
			},
			natConfig: &types.NATConfigure{
				PatPodName: "test-pat-pod",
			},
			invokeErr:      nil,
			getAsyncErr:    nil,
			expectGetAsync: true,
		},
		{
			name: "empty_faas_manager_info",
			funcSpec: &types.FunctionSpecification{
				FuncKey: "test-func",
			},
			faasManagerInfo: faasManagerInfo{},
			instance:        &types.Instance{},
			natConfig:       &types.NATConfigure{},
			expectGetAsync:  false,
		},
		{
			name: "invoke_error",
			funcSpec: &types.FunctionSpecification{
				FuncKey: "test-func",
			},
			faasManagerInfo: faasManagerInfo{
				funcKey:    "faas-key",
				instanceID: "instance-1",
			},
			instance: &types.Instance{},
			natConfig: &types.NATConfigure{
				PatPodName: "test-pat-pod",
			},
			invokeErr:      assert.AnError,
			expectGetAsync: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdk := &mockUtils.FakeLibruntimeSdkClient{}
			SetGlobalSdkClient(sdk)

			var getAsyncCalled bool
			patches := gomonkey.NewPatches()
			defer patches.Reset()

			patches.ApplyMethod(reflect.TypeOf(sdk), "InvokeByInstanceId",
				func(_ *mockUtils.FakeLibruntimeSdkClient, funcMeta api.FunctionMeta, instanceID string, args []api.Arg,
					invokeOpt api.InvokeOptions) (string, error) {
					if tt.invokeErr != nil {
						return "", tt.invokeErr
					}
					assert.Equal(t, tt.faasManagerInfo.funcKey, funcMeta.FuncID)
					assert.Equal(t, tt.faasManagerInfo.instanceID, instanceID)
					return "mock-obj-id", nil
				})

			patches.ApplyMethod(reflect.TypeOf(sdk), "GetAsync",
				func(_ *mockUtils.FakeLibruntimeSdkClient, objectID string, cb api.GetAsyncCallback) {
					getAsyncCalled = true
					cb([]byte("success"), tt.getAsyncErr)
				})

			reportInstanceWithVPC(tt.funcSpec, tt.faasManagerInfo, tt.instance, tt.natConfig)

			assert.NotNil(t, tt.expectGetAsync, getAsyncCalled, "GetAsync called status mismatch")
		})
	}
}

func TestHandlePullTriggerDelete(t *testing.T) {
	tests := []struct {
		name            string
		funcSpec        *types.FunctionSpecification
		faasManagerInfo faasManagerInfo
		invokeErr       error
		getAsyncErr     error
		expectGetAsync  bool
	}{
		{
			name: "successful_delete",
			funcSpec: &types.FunctionSpecification{
				FuncKey: "test-func",
			},
			faasManagerInfo: faasManagerInfo{
				funcKey:    "faas-key",
				instanceID: "instance-1",
			},
			invokeErr:      nil,
			getAsyncErr:    nil,
			expectGetAsync: true,
		},
		{
			name: "empty_faas_manager_info",
			funcSpec: &types.FunctionSpecification{
				FuncKey: "test-func",
			},
			faasManagerInfo: faasManagerInfo{},
			expectGetAsync:  false,
		},
		{
			name: "invoke_error",
			funcSpec: &types.FunctionSpecification{
				FuncKey: "test-func",
			},
			faasManagerInfo: faasManagerInfo{
				funcKey:    "faas-key",
				instanceID: "instance-1",
			},
			invokeErr:      assert.AnError,
			expectGetAsync: false,
		},
		{
			name: "get_async_error",
			funcSpec: &types.FunctionSpecification{
				FuncKey: "test-func",
			},
			faasManagerInfo: faasManagerInfo{
				funcKey:    "faas-key",
				instanceID: "instance-1",
			},
			invokeErr:      nil,
			getAsyncErr:    assert.AnError,
			expectGetAsync: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdk := &mockUtils.FakeLibruntimeSdkClient{}
			SetGlobalSdkClient(sdk)

			var getAsyncCalled bool
			patches := gomonkey.NewPatches()
			defer patches.Reset()

			patches.ApplyFunc(utils.GenerateTraceID,
				func() string {
					return "mock-trace-id"
				})

			patches.ApplyMethod(reflect.TypeOf(sdk), "InvokeByInstanceId",
				func(_ *mockUtils.FakeLibruntimeSdkClient, funcMeta api.FunctionMeta, instanceID string, args []api.Arg,
					invokeOpt api.InvokeOptions) (string, error) {
					if tt.invokeErr != nil {
						return "", tt.invokeErr
					}
					assert.Equal(t, tt.faasManagerInfo.funcKey, funcMeta.FuncID)
					assert.Equal(t, tt.faasManagerInfo.instanceID, instanceID)
					assert.Equal(t, api.PosixApi, funcMeta.Api)
					return "mock-obj-id", nil
				})

			patches.ApplyMethod(reflect.TypeOf(sdk), "GetAsync",
				func(_ *mockUtils.FakeLibruntimeSdkClient, objectID string, cb api.GetAsyncCallback) {
					getAsyncCalled = true
					cb([]byte("success"), tt.getAsyncErr)
				})

			handlePullTriggerDelete(tt.faasManagerInfo, tt.funcSpec)

			assert.NotNil(t, tt.expectGetAsync, getAsyncCalled, "GetAsync called status mismatch")
		})
	}
}

func TestHasD910b(t *testing.T) {
	var nilResSpec *resspeckey.ResourceSpecification
	assert.False(t, hasD910b(nilResSpec), "Expected false when resKey is nil")

	patches := gomonkey.NewPatches()
	defer patches.Reset()

	patches.ApplyFunc(utils.GetNpuTypeAndInstanceType,
		func(customRes map[string]int64, customResSpec map[string]interface{}) (string, string) {
			return types.AscendResourceD910B, ""
		})

	resSpecWithD910B := &resspeckey.ResourceSpecification{
		CustomResources:     map[string]int64{"mock-key": 1},
		CustomResourcesSpec: map[string]interface{}{"mock-key": "mock-value"},
	}
	assert.True(t, hasD910b(resSpecWithD910B), "Expected true when resKey has D910B resource")

	patches.Reset()
	patches.ApplyFunc(utils.GetNpuTypeAndInstanceType,
		func(customRes map[string]int64, customResSpec map[string]interface{}) (string, string) {
			return "non-D910B-resource", ""
		})

	resSpecWithoutD910B := &resspeckey.ResourceSpecification{
		CustomResources:     map[string]int64{"mock-key": 1},
		CustomResourcesSpec: map[string]interface{}{"mock-key": "mock-value"},
	}
	assert.False(t, hasD910b(resSpecWithoutD910B), "Expected false when resKey does not have D910B resource")
}
