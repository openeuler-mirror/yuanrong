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
package vpcmanager

import (
	"encoding/json"
	"errors"
	"plugin"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"

	mockUtils "yuanrong/pkg/common/faas_common/utils"
	"yuanrong/pkg/functionmanager/types"
)

func TestPluginVPC_CreateVpcResource(t *testing.T) {
	type args struct {
		requestData    []byte
		createResource interface{}
	}
	tests := []struct {
		name        string
		args        args
		want        types.NATConfigure
		wantErr     bool
		patchesFunc mockUtils.PatchesFunc
	}{
		{"case1 Marshal error",
			args{
				createResource: func(request []byte) ([]byte, error) {
					return []byte{}, nil
				},
			},
			types.NATConfigure{}, true, func() mockUtils.PatchSlice {
				patches := mockUtils.InitPatchSlice()
				patches.Append(mockUtils.PatchSlice{
					gomonkey.ApplyFunc(describeRequest,
						func(requestData []byte) types.VpcControllerRequester {
							return types.VpcControllerRequester{}
						}),
					gomonkey.ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
						return nil, errors.New("failed to marshel json")
					}),
				})
				return patches
			}},

		{"case2 createResource error",
			args{
				createResource: func(request []byte) ([]byte, error) {
					return nil, errors.New("test")
				},
			},
			types.NATConfigure{}, true, func() mockUtils.PatchSlice {
				patches := mockUtils.InitPatchSlice()
				patches.Append(mockUtils.PatchSlice{
					gomonkey.ApplyFunc(describeRequest,
						func(requestData []byte) types.VpcControllerRequester {
							return types.VpcControllerRequester{}
						}),
				})
				return patches
			}},

		{"case3 UnMarshal error",
			args{
				createResource: func(request []byte) ([]byte, error) {
					return []byte{}, nil
				},
			},
			types.NATConfigure{}, true, func() mockUtils.PatchSlice {
				patches := mockUtils.InitPatchSlice()
				patches.Append(mockUtils.PatchSlice{
					gomonkey.ApplyFunc(describeRequest,
						func(requestData []byte) types.VpcControllerRequester {
							return types.VpcControllerRequester{}
						}),
				})
				return patches
			}},

		{"case4 succeed to createVpcResource",
			args{
				createResource: func(request []byte) ([]byte, error) {
					tmp, _ := json.Marshal(types.NATConfigure{})
					return tmp, nil
				},
			},
			types.NATConfigure{}, false, func() mockUtils.PatchSlice {
				patches := mockUtils.InitPatchSlice()
				patches.Append(mockUtils.PatchSlice{
					gomonkey.ApplyFunc(describeRequest,
						func(requestData []byte) types.VpcControllerRequester {
							return types.VpcControllerRequester{}
						}),
				})
				return patches
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginVPC := PluginVPC{}
			pluginVPC.Plugin = &plugin.Plugin{}
			pluginVPC.createResource = tt.args.createResource
			patches := tt.patchesFunc()
			got, err := pluginVPC.CreateVpcResource(tt.args.requestData)
			if (err != nil) != tt.wantErr {
				t.Errorf("createVpcResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createVpcResource() got = %v, want %v", got, tt.want)
			}
			patches.ResetAll()
		})
	}
}

func TestPluginVPC_DeleteVpcResource(t *testing.T) {
	type args struct {
		patPodName     string
		deleteResource interface{}
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		patchesFunc mockUtils.PatchesFunc
	}{
		{"case1 deleteResource error",
			args{
				deleteResource: func(request string) error {
					return errors.New("test")
				},
			}, true, func() mockUtils.PatchSlice {
				patches := mockUtils.InitPatchSlice()
				return patches
			}},

		{"case2 check deleteResource error",
			args{
				deleteResource: func(request []byte) error {
					return errors.New("test")
				},
			}, true, func() mockUtils.PatchSlice {
				patches := mockUtils.InitPatchSlice()
				return patches
			}},

		{"case3 succeed to deleteResource",
			args{
				deleteResource: func(request string) error {
					return nil
				},
			}, false, func() mockUtils.PatchSlice {
				patches := mockUtils.InitPatchSlice()
				return patches
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginVPC := PluginVPC{}
			pluginVPC.Plugin = &plugin.Plugin{}
			pluginVPC.deleteResource = tt.args.deleteResource
			patches := tt.patchesFunc()
			err := pluginVPC.DeleteVpcResource(tt.args.patPodName)
			if (err != nil) != tt.wantErr {
				t.Errorf("createVpcResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			patches.ResetAll()
		})
	}
}

var (
	requestInfo = `{
		"id": "id",
		"domain_id": "domain_id",
		"namespace": "namespace",
		"vpc_name": "vpc_name",
		"vpc_id": "vpc_id",
		"subnet_name": "subnet_name",
		"subnet_id": "subnet_id",
		"tenant_cidr": "tenant_cidr",
		"host_vm_cidr": "host_vm_cidr",
		"gateway": "gateway",
		"xrole": "xrole",
		"app_xrole": "app_xrole"
	}`
)

func Test_describeRequest(t *testing.T) {
	type args struct {
		requestData []byte
	}
	var a args
	a.requestData = []byte(requestInfo)
	vpcInfo := types.Vpc{
		ID:         "id",
		DomainID:   "domain_id",
		Namespace:  "namespace",
		VpcName:    "vpc_name",
		VpcID:      "vpc_id",
		SubnetName: "subnet_name",
		SubnetID:   "subnet_id",
		TenantCidr: "tenant_cidr",
		HostVMCidr: "host_vm_cidr",
		Gateway:    "gateway",
	}
	delegate := types.Delegate{
		Xrole:    "xrole",
		AppXrole: "app_xrole",
	}
	request := types.VpcControllerRequester{
		TraceID:  "",
		Delegate: delegate,
		Vpc:      vpcInfo,
	}
	tests := []struct {
		name string
		args args
		want types.VpcControllerRequester
	}{
		{"case1", a, request},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := describeRequest(tt.args.requestData); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("describeRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitController(t *testing.T) {
	p := &PluginVPC{}
	convey.Convey(" patKeyList  is nil", t, func() {
		err := p.InitVpcPlugin()
		convey.So(err, convey.ShouldResemble, errors.New("pluginVpc is nil"))
	})
	defer gomonkey.ApplyMethod(reflect.TypeOf(&plugin.Plugin{}), "Lookup",
		func(_ *plugin.Plugin, symName string) (plugin.Symbol, error) {
			if symName == "InitController" {
				return func() error {
					return nil
				}, nil
			}
			return func() {}, nil
		}).Reset()

	p.Plugin = &plugin.Plugin{}

	convey.Convey(" patKeyList  is nil", t, func() {
		err := p.InitVpcPlugin()
		convey.So(err, convey.ShouldBeNil)
	})
}

func TestPluginVPC_initController(t *testing.T) {
	convey.Convey("initController", t, func() {
		p := &PluginVPC{Plugin: &plugin.Plugin{}}
		convey.Convey("failed to look up InitController", func() {
			err := p.initController()
			convey.So(err, convey.ShouldBeError)
		})
		convey.Convey("failed to init Controller", func() {
			defer gomonkey.ApplyMethod(reflect.TypeOf(&plugin.Plugin{}), "Lookup",
				func(_ *plugin.Plugin, symName string) (plugin.Symbol, error) {
					f := func() error {
						return errors.New("failed to init Controller")
					}
					return f, nil
				}).Reset()
			err := p.initController()
			convey.So(err, convey.ShouldBeError)
		})

		convey.Convey("failed to lookup function of CreateResource", func() {
			defer gomonkey.ApplyMethod(reflect.TypeOf(&plugin.Plugin{}), "Lookup",
				func(_ *plugin.Plugin, symName string) (plugin.Symbol, error) {
					if symName == "InitController" {
						f := func() error {
							return nil
						}
						return f, nil
					}
					return nil, errors.New("not found CreateResource")
				}).Reset()
			err := p.initController()
			convey.So(err, convey.ShouldBeError)
		})

		convey.Convey("failed to lookup function of DeleteResource", func() {
			defer gomonkey.ApplyMethod(reflect.TypeOf(&plugin.Plugin{}), "Lookup",
				func(_ *plugin.Plugin, symName string) (plugin.Symbol, error) {
					if symName == "InitController" {
						f := func() error {
							return nil
						}
						return f, nil
					}
					if symName == "CreateResource" {
						f := func() {}
						return f, nil
					}
					return nil, errors.New("not found DeleteResource")
				}).Reset()
			err := p.initController()
			convey.So(err, convey.ShouldBeError)
		})
	})

}
