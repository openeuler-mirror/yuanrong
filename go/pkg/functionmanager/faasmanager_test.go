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

package functionmanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"plugin"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	clientv3 "go.etcd.io/etcd/client/v3"
	v1 "k8s.io/api/apps/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	"yuanrong.org/kernel/runtime/libruntime/api"
	"yuanrong.org/kernel/pkg/common/faas_common/constant"
	"yuanrong.org/kernel/pkg/common/faas_common/etcd3"
	commonType "yuanrong.org/kernel/pkg/common/faas_common/types"
	mockUtils "yuanrong.org/kernel/pkg/common/faas_common/utils"
	"yuanrong.org/kernel/pkg/functionmanager/state"
	"yuanrong.org/kernel/pkg/functionmanager/types"
	"yuanrong.org/kernel/pkg/functionmanager/utils"
	"yuanrong.org/kernel/pkg/functionmanager/vpcmanager"
)

type KvMock struct {
}

func (k *KvMock) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	// TODO implement me
	return nil, nil
}

func (k *KvMock) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	response := &clientv3.GetResponse{}
	response.Count = 10
	return response, nil
}

func (k *KvMock) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return nil, fmt.Errorf("error")
}

func (k *KvMock) Compact(ctx context.Context, rev int64, opts ...clientv3.CompactOption) (*clientv3.CompactResponse,
	error) {
	// TODO implement me
	panic("implement me")
}

func (k *KvMock) Do(ctx context.Context, op clientv3.Op) (clientv3.OpResponse, error) {
	// TODO implement me
	panic("implement me")
}

func (k *KvMock) Txn(ctx context.Context) clientv3.Txn {
	// TODO implement me
	panic("implement me")
}

func TestNewFaaSManager(t *testing.T) {
	defer gomonkey.ApplyFunc(state.Update, func(value interface{}, tags ...string) {
	}).Reset()
	type args struct {
		sdkClient api.LibruntimeAPI
		stopCh    chan struct{}
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		patchesFunc mockUtils.PatchesFunc
	}{
		{"case1 failed to plugin open", args{}, true, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(plugin.Open, func(path string) (*plugin.Plugin, error) {
					return nil, errors.New("plugin open error")
				}),
			})
			return patches
		}},
		{"case2 InitVpcPlugin error", args{}, true, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(plugin.Open, func(path string) (*plugin.Plugin, error) {
					return &plugin.Plugin{}, nil
				}),
				gomonkey.ApplyFunc((*vpcmanager.PluginVPC).InitVpcPlugin, func(_ *vpcmanager.PluginVPC) error {
					return errors.New("InitVpcPlugin error")
				}),
			})
			return patches
		}},
		{"case3 InitKubeClient error", args{}, true, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(plugin.Open, func(path string) (*plugin.Plugin, error) {
					return &plugin.Plugin{}, nil
				}),
				gomonkey.ApplyFunc((*vpcmanager.PluginVPC).InitVpcPlugin, func(_ *vpcmanager.PluginVPC) error {
					return nil
				}),
				gomonkey.ApplyFunc(utils.InitKubeClient, func() error {
					return errors.New("InitKubeClient error")
				}),
			})
			return patches
		}},
		{"case3 InitKubeClient error", args{}, true, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(plugin.Open, func(path string) (*plugin.Plugin, error) {
					return &plugin.Plugin{}, nil
				}),
				gomonkey.ApplyFunc((*vpcmanager.PluginVPC).InitVpcPlugin, func(_ *vpcmanager.PluginVPC) error {
					return nil
				}),
				gomonkey.ApplyFunc(utils.InitKubeClient, func() error {
					return errors.New("InitKubeClient error")
				}),
			})
			return patches
		}},
		{"case4 succeed to NewFaaSManager", args{}, false, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(plugin.Open, func(path string) (*plugin.Plugin, error) {
					return &plugin.Plugin{}, nil
				}),
				gomonkey.ApplyFunc((*vpcmanager.PluginVPC).InitVpcPlugin, func(_ *vpcmanager.PluginVPC) error {
					return nil
				}),
				gomonkey.ApplyFunc(utils.InitKubeClient, func() error {
					return nil
				}),
			})
			return patches
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches := tt.patchesFunc()
			_, err := NewFaaSManagerLibruntime(tt.args.sdkClient, tt.args.stopCh)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFaaSManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			patches.ResetAll()
		})
	}
}

func TestManager_ProcessSchedulerRequest(t *testing.T) {
	defer gomonkey.ApplyFunc(state.Update, func(value interface{}, tags ...string) {
	}).Reset()
	pullTriggerBytes, _ := json.Marshal(types.PullTriggerRequestInfo{PodName: "sn/trigger/test"})
	reportBytes, _ := json.Marshal(types.ReportInfo{PatPodName: "pat1", InstanceID: "abc"})
	deleteBytes, _ := json.Marshal(types.DeleteInfo{InstanceID: "abc"})
	deleteTriggerBytes, _ := json.Marshal(types.PullTriggerDeleteInfo{PodName: "sn/trigger/test"})
	type fields struct {
		patKeyList        map[string]map[string]struct{}
		VpcPlugin         vpcmanager.PluginVPC
		vpcMutex          sync.Mutex
		remoteClientLease map[string]*leaseTimer
		remoteClientList  map[string]struct{}
		clientMutex       sync.Mutex
	}
	type args struct {
		args []*api.Arg
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        int
		patchesFunc mockUtils.PatchesFunc
	}{
		{"case1 succeed to requestOpCreate", fields{patKeyList: make(map[string]map[string]struct{}, 1),
			vpcMutex: sync.Mutex{}},
			args{args: []*api.Arg{&api.Arg{Data: []byte(requestOpCreate)}, &api.Arg{Data: []byte("")},
				&api.Arg{Data: []byte("")}}},
			constant.InsReqSuccessCode, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc((*vpcmanager.PluginVPC).CreateVpcResource,
					func(_ *vpcmanager.PluginVPC, requestData []byte) (types.NATConfigure, error) {
						return types.NATConfigure{PatPodName: "pat1"}, nil
					}),
			})
			return patches
		}},
		{"case2 succeed to requestOpCreateTrigger", fields{patKeyList: make(map[string]map[string]struct{}, 1),
			vpcMutex: sync.Mutex{}},
			args{args: []*api.Arg{&api.Arg{Data: []byte(requestOpCreateTrigger)}, &api.Arg{Data: pullTriggerBytes},
				&api.Arg{Data: []byte("")}}},
			constant.InsReqSuccessCode, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc((*vpcmanager.PluginVPC).CreateVpcResource,
					func(_ *vpcmanager.PluginVPC, requestData []byte) (types.NATConfigure, error) {
						return types.NATConfigure{PatPodName: "pat1"}, nil
					}),
				gomonkey.ApplyFunc(utils.GetDeployByK8S,
					func(k8sClient kubernetes.Interface, deployName string) (*v1.Deployment, error) {
						return nil, errors.New("errors")
					}),
				gomonkey.ApplyFunc(k8serror.IsNotFound, func(err error) bool {
					return true
				}),
				gomonkey.ApplyFunc(utils.GetKubeClient, func() kubernetes.Interface {
					return &kubernetes.Clientset{}
				}),
				gomonkey.ApplyFunc(utils.CreateDeployByK8S,
					func(k8sClient kubernetes.Interface, deploy *v1.Deployment) error {
						return nil
					}),
			})
			return patches
		}},
		{"case3 succeed to requestOpReport", fields{patKeyList: make(map[string]map[string]struct{}, 1),
			vpcMutex: sync.Mutex{}},
			args{args: []*api.Arg{&api.Arg{Data: []byte(requestOpReport)}, &api.Arg{Data: reportBytes},
				&api.Arg{Data: []byte("")}}},
			constant.InsReqSuccessCode, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{})
			return patches
		}},
		{"case4 succeed to requestOpDelete", fields{patKeyList: make(map[string]map[string]struct{}, 1),
			vpcMutex: sync.Mutex{}},
			args{args: []*api.Arg{&api.Arg{Data: []byte(requestOpDelete)}, &api.Arg{Data: deleteBytes},
				&api.Arg{Data: []byte("")}}},
			constant.InsReqSuccessCode, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{})
			return patches
		}},
		{"case5 succeed to requestOpDeleteTrigger", fields{patKeyList: make(map[string]map[string]struct{}, 1),
			vpcMutex: sync.Mutex{}},
			args{args: []*api.Arg{&api.Arg{Data: []byte(requestOpDeleteTrigger)}, &api.Arg{Data: deleteTriggerBytes},
				&api.Arg{Data: []byte("")}}},
			constant.InsReqSuccessCode, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(utils.GetKubeClient, func() kubernetes.Interface {
					return &kubernetes.Clientset{}
				}),
				gomonkey.ApplyFunc(utils.GetDeployByK8S,
					func(k8sClient kubernetes.Interface, deployName string) (*v1.Deployment, error) {
						return &v1.Deployment{}, nil
					}),
				gomonkey.ApplyFunc(k8serror.IsNotFound, func(err error) bool {
					return false
				}),
				gomonkey.ApplyFunc(utils.DeleteDeployByK8S,
					func(k8sClient kubernetes.Interface, name string) error {
						return nil
					}),
			})
			return patches
		}},
		{"case6 unknow opt", fields{patKeyList: make(map[string]map[string]struct{}, 1),
			vpcMutex: sync.Mutex{}},
			args{args: []*api.Arg{&api.Arg{Data: []byte(requestOpDeleteTrigger)}, &api.Arg{Data: []byte("")}}},
			constant.UnsupportedOperationErrorCode,
			func() mockUtils.PatchSlice {
				patches := mockUtils.InitPatchSlice()
				return patches
			}},
		{"case7 failed to requestOpNewLease", fields{patKeyList: make(map[string]map[string]struct{}, 1),
			vpcMutex: sync.Mutex{}, remoteClientLease: map[string]*leaseTimer{"test-client-ID": nil},
			remoteClientList: map[string]struct{}{}, clientMutex: sync.Mutex{}},
			args{args: []*api.Arg{&api.Arg{Data: []byte(requestOpNewLease)}, &api.Arg{Data: []byte("")}}},
			constant.UnsupportedOperationErrorCode, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{})
			return patches
		}},
		{"case8 success to requestOpNewLease", fields{patKeyList: make(map[string]map[string]struct{}, 1),
			vpcMutex: sync.Mutex{}, remoteClientLease: make(map[string]*leaseTimer, 1),
			remoteClientList: map[string]struct{}{}, clientMutex: sync.Mutex{}},
			args{args: []*api.Arg{&api.Arg{Data: []byte(requestOpNewLease)}, &api.Arg{Data: []byte("test-client-ID")},
				&api.Arg{Data: []byte("")}}},
			constant.InsReqSuccessCode, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{})
			return patches
		}},
		{"case9 failed to requestOpKeepAlive", fields{patKeyList: make(map[string]map[string]struct{}, 1),
			vpcMutex: sync.Mutex{}, remoteClientLease: make(map[string]*leaseTimer, 1),
			remoteClientList: map[string]struct{}{}, clientMutex: sync.Mutex{}},
			args{args: []*api.Arg{&api.Arg{Data: []byte(requestOpKeepAlive)}, &api.Arg{Data: []byte("test-client-ID")},
				&api.Arg{Data: []byte("")}}},
			constant.UnsupportedOperationErrorCode, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{})
			return patches
		}},
		{"case10 success to requestOpKeepAlive", fields{patKeyList: make(map[string]map[string]struct{}, 1),
			vpcMutex: sync.Mutex{}, remoteClientLease: map[string]*leaseTimer{"test-client-ID": newLeaseTimer(1000)},
			remoteClientList: map[string]struct{}{}, clientMutex: sync.Mutex{}},
			args{args: []*api.Arg{&api.Arg{Data: []byte(requestOpKeepAlive)}, &api.Arg{Data: []byte("test-client-ID")},
				&api.Arg{Data: []byte("")}}},
			constant.InsReqSuccessCode, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{})
			return patches
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches := tt.patchesFunc()
			fm := &Manager{
				patKeyList:        tt.fields.patKeyList,
				VpcPlugin:         tt.fields.VpcPlugin,
				vpcMutex:          tt.fields.vpcMutex,
				remoteClientLease: tt.fields.remoteClientLease,
				remoteClientList:  tt.fields.remoteClientList,
				clientMutex:       tt.fields.clientMutex,
				queue: &queue{
					cond:       sync.NewCond(&sync.RWMutex{}),
					dirty:      map[interface{}]struct{}{},
					processing: map[interface{}]struct{}{},
				},
				leaseRenewMinute: 5,
			}
			go fm.saveStateLoop()
			libruntimeClient = &mockUtils.FakeLibruntimeSdkClient{}
			if got := fm.ProcessSchedulerRequest(tt.args.args, ""); !reflect.DeepEqual(got.Code, tt.want) {
				t.Errorf("ProcessSchedulerRequest() = %v, want %v", got, tt.want)
			}
			patches.ResetAll()
			libruntimeClient = nil
		})
	}
}

func Test_handleRequestOpCreate(t *testing.T) {
	convey.Convey("ProcessSchedulerRequest-handleRequestOpCreate", t, func() {
		m := &Manager{
			patKeyList:       make(map[string]map[string]struct{}),
			VpcPlugin:        vpcmanager.PluginVPC{},
			leaseRenewMinute: 5,
		}
		args := []*api.Arg{
			{
				Type: 0,
				Data: []byte(requestOpCreate),
			},
			{
				Type: 0,
				Data: []byte{},
			},
			{
				Type: 0,
				Data: []byte("trace-id"),
			},
		}
		convey.Convey("failed to create vpc pat pod, error", func() {

			request := m.ProcessSchedulerRequest(args, "")
			convey.So(request, convey.ShouldBeNil)
		})

		convey.Convey("json Marshal error", func() {
			defer gomonkey.ApplyMethod(reflect.TypeOf(&m.VpcPlugin), "CreateVpcResource",
				func(_ *vpcmanager.PluginVPC, requestData []byte) (types.NATConfigure, error) {
					return types.NATConfigure{}, nil
				}).Reset()
			defer gomonkey.ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				return nil, errors.New("json marshal error")
			}).Reset()
			request := m.ProcessSchedulerRequest(args, "")
			convey.So(request, convey.ShouldBeNil)
		})
	})
}

func Test_handleRequestOpCreateTrigger(t *testing.T) {
	convey.Convey("ProcessSchedulerRequest-handleRequestOpCreateTrigger", t, func() {
		m := &Manager{
			patKeyList:       make(map[string]map[string]struct{}),
			VpcPlugin:        vpcmanager.PluginVPC{},
			leaseRenewMinute: 5,
		}
		args := []*api.Arg{
			{
				Type: 0,
				Data: []byte(requestOpCreateTrigger),
			},
			{
				Type: 0,
				Data: []byte{},
			},
			{
				Type: 0,
				Data: []byte("trace-id"),
			},
		}
		convey.Convey("json.Unmarshal error", func() {
			defer gomonkey.ApplyFunc(json.Unmarshal, func(data []byte, v interface{}) error {
				return errors.New("json unmarshal error")
			}).Reset()
			request := m.ProcessSchedulerRequest(args, "")
			convey.So(request, convey.ShouldBeNil)
		})

		convey.Convey("invalid pull trigger name", func() {
			info := &types.PullTriggerRequestInfo{PodName: "podname/invalid"}
			bytes, _ := json.Marshal(info)
			args[1].Data = bytes
			request := m.ProcessSchedulerRequest(args, "")
			convey.So(request, convey.ShouldBeNil)
		})

		convey.Convey("failed to create pullTrigger vpc pat pod, error", func() {
			defer gomonkey.ApplyMethod(reflect.TypeOf(&m.VpcPlugin), "CreateVpcResource",
				func(_ *vpcmanager.PluginVPC, requestData []byte) (types.NATConfigure, error) {
					return types.NATConfigure{}, errors.New("failed to create pullTrigger vpc pat pod")
				}).Reset()
			info := &types.PullTriggerRequestInfo{PodName: "podname/runtime-manager/valid"}
			bytes, _ := json.Marshal(info)
			args[1].Data = bytes
			request := m.ProcessSchedulerRequest(args, "")
			convey.So(request, convey.ShouldBeNil)
		})

		convey.Convey("failed to Marshal PatPod", func() {
			defer gomonkey.ApplyMethod(reflect.TypeOf(&m.VpcPlugin), "CreateVpcResource",
				func(_ *vpcmanager.PluginVPC, requestData []byte) (types.NATConfigure, error) {
					return types.NATConfigure{}, nil
				}).Reset()
			info := &types.PullTriggerRequestInfo{PodName: "podname/runtime-manager/valid"}
			bytes, _ := json.Marshal(info)
			args[1].Data = bytes
			defer gomonkey.ApplyFunc(json.Marshal, func(v interface{}) ([]byte, error) {
				return nil, errors.New("json marshal error")
			}).Reset()
			request := m.ProcessSchedulerRequest(args, "")
			convey.So(request, convey.ShouldBeNil)
		})

		convey.Convey("failed to create deploy %s by k8s, error: %s", func() {
			defer gomonkey.ApplyMethod(reflect.TypeOf(&m.VpcPlugin), "CreateVpcResource",
				func(_ *vpcmanager.PluginVPC, requestData []byte) (types.NATConfigure, error) {
					return types.NATConfigure{}, nil
				}).Reset()
			defer gomonkey.ApplyFunc(utils.GetDeployByK8S,
				func(k8sClient kubernetes.Interface, deployName string) (*v1.Deployment, error) {
					return &v1.Deployment{}, k8serror.NewNotFound(schema.GroupResource{}, "runtime-manager")
				}).Reset()
			defer gomonkey.ApplyFunc(utils.CreateDeployByK8S,
				func(k8sClient kubernetes.Interface, deploy *v1.Deployment) error {
					return errors.New("create deploy error")
				}).Reset()
			info := &types.PullTriggerRequestInfo{PodName: "podname/runtime-manager/valid"}
			bytes, _ := json.Marshal(info)
			args[1].Data = bytes
			request := m.ProcessSchedulerRequest(args, "")
			convey.So(request, convey.ShouldBeNil)
		})

		convey.Convey("success", func() {
			defer gomonkey.ApplyMethod(reflect.TypeOf(&m.VpcPlugin), "CreateVpcResource",
				func(_ *vpcmanager.PluginVPC, requestData []byte) (types.NATConfigure, error) {
					return types.NATConfigure{}, nil
				}).Reset()
			defer gomonkey.ApplyFunc(utils.GetDeployByK8S,
				func(k8sClient kubernetes.Interface, deployName string) (*v1.Deployment, error) {
					return &v1.Deployment{}, nil
				}).Reset()
			info := &types.PullTriggerRequestInfo{PodName: "podname/runtime-manager/valid"}
			bytes, _ := json.Marshal(info)
			args[1].Data = bytes
			request := m.ProcessSchedulerRequest(args, "")
			convey.So(request.Code, convey.ShouldEqual, 6030)
		})
	})
}

func Test_handleRequestOpReport(t *testing.T) {
	convey.Convey("handleRequestOpReport", t, func() {
		m := &Manager{
			patKeyList:       make(map[string]map[string]struct{}),
			VpcPlugin:        vpcmanager.PluginVPC{},
			leaseRenewMinute: 5,
		}
		args := []*api.Arg{
			{
				Type: 0,
				Data: []byte(requestOpReport),
			},
			{
				Type: 0,
				Data: []byte{},
			},
			{
				Type: 0,
				Data: []byte("trace-id"),
			},
		}
		convey.Convey("failed to unmarshal report info", func() {
			defer gomonkey.ApplyFunc(json.Unmarshal, func(data []byte, v interface{}) error {
				return errors.New("json unmarshal error")
			}).Reset()
			request := m.ProcessSchedulerRequest(args, "")
			convey.So(request, convey.ShouldBeNil)
		})

		convey.Convey("", func() {
			report := &types.ReportInfo{PatPodName: "test"}
			bytes, _ := json.Marshal(report)
			args[1].Data = bytes
			m.patKeyList["test"] = make(map[string]struct{})
			request := m.ProcessSchedulerRequest(args, "")
			convey.So(request.Message, convey.ShouldEqual, "succeed add pat list")
		})
	})
}

func TestMiscellaneous(t *testing.T) {
	convey.Convey(" patKeyList  is nil", t, func() {
		m := &Manager{
			patKeyList:       nil,
			leaseRenewMinute: 5,
		}
		convey.Convey("addPatList", func() {
			m.addPatList("123", "456")
			convey.So(m.patKeyList, convey.ShouldBeNil)
		})
		convey.Convey("checkExists", func() {
			res := m.checkExists("123")
			convey.So(res, convey.ShouldBeFalse)
		})
		convey.Convey("deletePatList", func() {
			podName, needDeletePat := m.deletePatList("123")
			convey.So(podName, convey.ShouldEqual, "")
			convey.So(needDeletePat, convey.ShouldBeFalse)
		})
	})
	convey.Convey("deletePatList", t, func() {
		defer gomonkey.ApplyFunc(state.Update, func(value interface{}, tags ...string) {
		}).Reset()
		m := &Manager{
			patKeyList: map[string]map[string]struct{}{
				"patName123": map[string]struct{}{
					"instanceID123": struct{}{},
				},
			},
			leaseRenewMinute: 5,
		}
		convey.Convey("deletePatList", func() {
			podName, needDeletePat := m.deletePatList("instanceID123")
			convey.So(podName, convey.ShouldEqual, "patName123")
			convey.So(needDeletePat, convey.ShouldBeTrue)
		})
	})
}

func Test_handleLeaseTimeout(t *testing.T) {
	convey.Convey("handleLeaseTimeout", t, func() {
		m := &Manager{
			patKeyList:        make(map[string]map[string]struct{}),
			VpcPlugin:         vpcmanager.PluginVPC{},
			vpcMutex:          sync.Mutex{},
			remoteClientLease: map[string]*leaseTimer{},
			remoteClientList:  map[string]struct{}{},
			clientMutex:       sync.Mutex{},
			queue: &queue{
				cond:       sync.NewCond(&sync.RWMutex{}),
				dirty:      map[interface{}]struct{}{},
				processing: map[interface{}]struct{}{},
			},
			leaseRenewMinute: 5,
		}
		go m.saveStateLoop()
		args := []*api.Arg{
			{
				Type: 0,
				Data: []byte(requestOpNewLease),
			},
			{
				Type: 0,
				Data: []byte("test-client-ID"),
			},
			{
				Type: 0,
				Data: []byte("trace-id"),
			},
		}
		convey.Convey("handle client lease timeout", func() {
			defer gomonkey.ApplyFunc(state.Update, func(value interface{}, tags ...string) {}).Reset()
			libruntimeClient = &mockUtils.FakeLibruntimeSdkClient{}
			request := m.ProcessSchedulerRequest(args, "test-trace-ID")
			convey.So(request.Message, convey.ShouldEqual, "Succeed to create lease")
			convey.So(len(m.remoteClientList), convey.ShouldEqual, 1)
			convey.So(m.remoteClientLease["test-client-ID"], convey.ShouldNotBeNil)
			m.remoteClientLease["test-client-ID"].Timer.Reset(1 * time.Second)
			time.Sleep(2 * time.Second)
			convey.So(len(m.remoteClientList), convey.ShouldEqual, 0)
			convey.So(len(m.remoteClientLease), convey.ShouldEqual, 0)
			libruntimeClient = nil
		})
	})
}

func Test_handleDelLease(t *testing.T) {
	convey.Convey("handleDelLease", t, func() {
		m := &Manager{
			patKeyList:        make(map[string]map[string]struct{}),
			VpcPlugin:         vpcmanager.PluginVPC{},
			vpcMutex:          sync.Mutex{},
			remoteClientLease: map[string]*leaseTimer{"test-client-ID": newLeaseTimer(1000)},
			remoteClientList:  map[string]struct{}{"test-client-ID": {}},
			clientMutex:       sync.Mutex{},
			queue: &queue{
				cond:       sync.NewCond(&sync.RWMutex{}),
				dirty:      map[interface{}]struct{}{},
				processing: map[interface{}]struct{}{},
			},
			leaseRenewMinute: 5,
		}
		go m.saveStateLoop()
		args := []*api.Arg{
			{
				Type: 0,
				Data: []byte(requestOpDelLease),
			},
			{
				Type: 0,
				Data: []byte("test-client-ID"),
			},
			{
				Type: 0,
				Data: []byte("trace-id"),
			},
		}
		libruntimeClient = &mockUtils.FakeLibruntimeSdkClient{}
		convey.Convey("delete no exited lease", func() {
			args[1].Data = []byte("test-client-ID-error")
			defer gomonkey.ApplyFunc(state.Update, func(value interface{}, tags ...string) {}).Reset()
			request := m.ProcessSchedulerRequest(args, "test-trace-ID")
			convey.So(request.Message, convey.ShouldEqual, "Succeed to delete lease")
			convey.So(len(m.remoteClientList), convey.ShouldEqual, 1)
			convey.So(len(m.remoteClientLease), convey.ShouldEqual, 1)
		})
		convey.Convey("success to delete lease", func() {
			defer gomonkey.ApplyFunc(state.Update, func(value interface{}, tags ...string) {}).Reset()
			request := m.ProcessSchedulerRequest(args, "test-trace-ID")
			convey.So(request.Message, convey.ShouldEqual, "Succeed to delete lease")
			convey.So(len(m.remoteClientList), convey.ShouldEqual, 0)
			convey.So(len(m.remoteClientLease), convey.ShouldEqual, 0)
		})
		libruntimeClient = nil
	})
}

func TestManager_handleLeaseEvent(t *testing.T) {
	libruntimeClient = &mockUtils.FakeLibruntimeSdkClient{}
	convey.Convey("test handleLeaseEvent", t, func() {
		convey.Convey("baseline", func() {
			p := gomonkey.ApplyFunc(etcd3.GetRouterEtcdClient, func() *etcd3.EtcdClient {
				return &etcd3.EtcdClient{Client: &clientv3.Client{KV: &KvMock{}}}
			})
			defer p.Reset()
			ch := make(chan struct{})
			fm := &Manager{
				patKeyList:        make(map[string]map[string]struct{}, 1),
				vpcMutex:          sync.Mutex{},
				remoteClientLease: make(map[string]*leaseTimer),
				remoteClientList:  map[string]struct{}{},
				clientMutex:       sync.Mutex{},
				stopCh:            ch,
				queue: &queue{
					cond:       sync.NewCond(&sync.RWMutex{}),
					dirty:      map[interface{}]struct{}{},
					processing: map[interface{}]struct{}{},
				},
				leaseRenewMinute: 5,
			}
			go fm.saveStateLoop()
			e := &commonType.LeaseEvent{
				Type:           constant.NewLease,
				RemoteClientID: "123456",
				Timestamp:      time.Now().Unix(),
				TraceID:        "abc",
			}
			marshal, _ := json.Marshal(e)
			event := &etcd3.Event{
				Type:      0,
				Key:       "/sn/lease/123456",
				Value:     marshal,
				PrevValue: nil,
				Rev:       0,
			}
			fm.handleLeaseEvent(event)
			convey.So(fm.remoteClientLease["123456"], convey.ShouldNotBeNil)

			e = &commonType.LeaseEvent{
				Type:           constant.DelLease,
				RemoteClientID: "123456",
				Timestamp:      time.Now().Unix(),
				TraceID:        "abcd",
			}
			marshal, _ = json.Marshal(e)
			event = &etcd3.Event{
				Type:      0,
				Key:       "/sn/lease/123456",
				Value:     marshal,
				PrevValue: nil,
				Rev:       0,
			}
			fm.handleLeaseEvent(event)
			convey.So(fm.remoteClientLease["123456"], convey.ShouldBeNil)

			e = &commonType.LeaseEvent{
				Type:           constant.KeepAlive,
				RemoteClientID: "123456",
				Timestamp:      time.Now().Unix(),
				TraceID:        "abcde",
			}
			marshal, _ = json.Marshal(e)
			event = &etcd3.Event{
				Type:      0,
				Key:       "/sn/lease/123456",
				Value:     marshal,
				PrevValue: nil,
				Rev:       0,
			}
			fm.handleLeaseEvent(event)
			time.Sleep(100 * time.Millisecond)
			convey.So(fm.remoteClientLease["123456"], convey.ShouldNotBeNil)

			e = &commonType.LeaseEvent{
				Type:           constant.KeepAlive,
				RemoteClientID: "123456",
				Timestamp:      time.Now().Unix(),
				TraceID:        "abcdef",
			}
			marshal, _ = json.Marshal(e)
			event = &etcd3.Event{
				Type:      0,
				Key:       "/sn/lease/123456",
				Value:     marshal,
				PrevValue: nil,
				Rev:       0,
			}
			fm.handleLeaseEvent(event)
			convey.So(fm.remoteClientLease["123456"], convey.ShouldNotBeNil)
		})

		convey.Convey("event is nil", func() {
			ch := make(chan struct{})
			fm := &Manager{
				patKeyList:        make(map[string]map[string]struct{}, 1),
				vpcMutex:          sync.Mutex{},
				remoteClientLease: make(map[string]*leaseTimer),
				remoteClientList:  map[string]struct{}{},
				clientMutex:       sync.Mutex{},
				stopCh:            ch,
				queue: &queue{
					cond:       sync.NewCond(&sync.RWMutex{}),
					dirty:      map[interface{}]struct{}{},
					processing: map[interface{}]struct{}{},
				},
				leaseRenewMinute: 5,
			}
			go fm.saveStateLoop()
			fm.handleLeaseEvent(nil)
			convey.So(len(fm.remoteClientLease), convey.ShouldEqual, 0)
		})

		convey.Convey("unmarshal failed", func() {
			ch := make(chan struct{})
			fm := &Manager{
				patKeyList:        make(map[string]map[string]struct{}, 1),
				vpcMutex:          sync.Mutex{},
				remoteClientLease: make(map[string]*leaseTimer),
				remoteClientList:  map[string]struct{}{},
				clientMutex:       sync.Mutex{},
				stopCh:            ch,
				queue: &queue{
					cond:       sync.NewCond(&sync.RWMutex{}),
					dirty:      map[interface{}]struct{}{},
					processing: map[interface{}]struct{}{},
				},
				leaseRenewMinute: 5,
			}
			go fm.saveStateLoop()
			event := &etcd3.Event{
				Type:      0,
				Key:       "/sn/lease/123456",
				Value:     []byte("aaa"),
				PrevValue: nil,
				Rev:       0,
			}
			fm.handleLeaseEvent(event)
			convey.So(len(fm.remoteClientLease), convey.ShouldEqual, 0)
		})

		convey.Convey("error type", func() {
			ch := make(chan struct{})
			fm := &Manager{
				patKeyList:        make(map[string]map[string]struct{}, 1),
				vpcMutex:          sync.Mutex{},
				remoteClientLease: make(map[string]*leaseTimer),
				remoteClientList:  map[string]struct{}{},
				clientMutex:       sync.Mutex{},
				stopCh:            ch,
				queue: &queue{
					cond:       sync.NewCond(&sync.RWMutex{}),
					dirty:      map[interface{}]struct{}{},
					processing: map[interface{}]struct{}{},
				},
				leaseRenewMinute: 5,
			}
			go fm.saveStateLoop()
			e := &commonType.LeaseEvent{
				Type:           "aaa",
				RemoteClientID: "123456",
				Timestamp:      time.Now().Unix(),
				TraceID:        "abcdef",
			}
			marshal, _ := json.Marshal(e)
			event := &etcd3.Event{
				Type:      0,
				Key:       "/sn/lease/123456",
				Value:     marshal,
				PrevValue: nil,
				Rev:       0,
			}
			fm.handleLeaseEvent(event)
			convey.So(len(fm.remoteClientLease), convey.ShouldEqual, 0)
		})

	})
	libruntimeClient = nil
}

func TestManager_WatchLeaseEvent(t *testing.T) {
	libruntimeClient = &mockUtils.FakeLibruntimeSdkClient{}
	convey.Convey("test watch lease event", t, func() {
		convey.Convey("baseline", func() {
			p := gomonkey.ApplyFunc(etcd3.GetRouterEtcdClient, func() *etcd3.EtcdClient {
				return &etcd3.EtcdClient{Client: &clientv3.Client{KV: &KvMock{}}}
			})
			defer p.Reset()
			p2 := gomonkey.ApplyFunc((*etcd3.EtcdWatcher).StartWatch, func(ew *etcd3.EtcdWatcher) {
				ew.ResultChan <- &etcd3.Event{
					Type:      0,
					Key:       "/sn/lease/123456",
					Value:     nil,
					PrevValue: nil,
					Rev:       0,
				}
			})
			defer p2.Reset()
			flag := false
			p3 := gomonkey.ApplyFunc((*Manager).handleLeaseEvent, func(_ *Manager) {
				flag = true
			})
			defer p3.Reset()
			ch := make(chan struct{})
			fm := &Manager{
				patKeyList:        make(map[string]map[string]struct{}, 1),
				vpcMutex:          sync.Mutex{},
				remoteClientLease: make(map[string]*leaseTimer),
				remoteClientList:  map[string]struct{}{},
				clientMutex:       sync.Mutex{},
				stopCh:            ch,
				queue: &queue{
					cond:       sync.NewCond(&sync.RWMutex{}),
					dirty:      map[interface{}]struct{}{},
					processing: map[interface{}]struct{}{},
				},
				leaseRenewMinute: 5,
			}
			go fm.saveStateLoop()
			go fm.WatchLeaseEvent()
			time.Sleep(100 * time.Millisecond)
			convey.So(flag, convey.ShouldBeTrue)
			ch <- struct{}{}
		})
	})
	libruntimeClient = nil
}

func TestManager_RecoverData(t *testing.T) {
	libruntimeClient = &mockUtils.FakeLibruntimeSdkClient{}
	convey.Convey("test recover data", t, func() {
		convey.Convey("baseline", func() {
			p := gomonkey.ApplyFunc(state.GetState, func() *state.ManagerState {
				return &state.ManagerState{
					PatKeyList: map[string]map[string]struct{}{
						"a": {"a": struct{}{}},
					},
					RemoteClientList: map[string]struct{}{"aaa": {}, "bbb": {}},
				}
			})
			defer p.Reset()
			fm := &Manager{
				patKeyList:        make(map[string]map[string]struct{}, 1),
				vpcMutex:          sync.Mutex{},
				remoteClientLease: make(map[string]*leaseTimer),
				remoteClientList:  map[string]struct{}{},
				clientMutex:       sync.Mutex{},
				stopCh:            nil,
				queue: &queue{
					cond:       sync.NewCond(&sync.RWMutex{}),
					dirty:      map[interface{}]struct{}{},
					processing: map[interface{}]struct{}{},
				},
				leaseRenewMinute: 5,
			}
			go fm.saveStateLoop()
			fm.RecoverData()
			convey.So(len(fm.remoteClientLease), convey.ShouldEqual, 2)
			convey.So(len(fm.patKeyList), convey.ShouldEqual, 1)
		})
	})
	libruntimeClient = nil
}
