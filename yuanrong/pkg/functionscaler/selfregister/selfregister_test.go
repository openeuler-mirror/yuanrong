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

package selfregister

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"yuanrong/pkg/common/faas_common/etcd3"
	commontypes "yuanrong/pkg/common/faas_common/types"
	"yuanrong/pkg/functionscaler/config"
	"yuanrong/pkg/functionscaler/types"
)

func setEnv() {
	os.Setenv("CLUSTER_ID", "cluster001")
	os.Setenv("NODE_IP", "127.0.0.1")
	os.Setenv("POD_NAME", "faas-scheduler-5b5886db99-66gn8")
	os.Setenv("POD_IP", "127.0.0.1")
}

func cleanEnv() {
	os.Setenv("CLUSTER_ID", "")
	os.Setenv("NODE_IP", "")
	os.Setenv("POD_NAME", "")
	os.Setenv("POD_IP", "")
}

func TestGetServiceKeyAndValue(t *testing.T) {
	convey.Convey("test getInstanceKeyAndValue success", t, func() {
		convey.Convey("success", func() {
			setEnv()
			defer cleanEnv()
			rawGConfig := config.GlobalConfig
			config.GlobalConfig = types.Configuration{
				ModuleConfig: &types.ModuleConfig{
					ServicePort: "8888",
				},
			}
			defer func() {
				config.GlobalConfig = rawGConfig
			}()
			key, _, err := getInstanceKeyAndValue()
			convey.So(err, convey.ShouldBeNil)
			convey.So(key, convey.ShouldEqual,
				"/sn/faas-scheduler/instances/cluster001/127.0.0.1/faas-scheduler-5b5886db99-66gn8")
		})
		convey.Convey("failed", func() {
			os.Setenv("NODE_IP", "")
			_, _, err := getInstanceKeyAndValue()
			convey.So(err, convey.ShouldNotBeNil)
		})

	})
}

func TestRegisterToEtcd(t *testing.T) {
	config.GlobalConfig.SchedulerDiscovery = &types.SchedulerDiscovery{RegisterMode: types.RegisterTypeSelf}
	enableRollout := config.GlobalConfig.EnableRollout
	patches := []*gomonkey.Patches{
		gomonkey.ApplyGlobalVar(&maxContendTime, 1),
		gomonkey.ApplyGlobalVar(&contendWaitInterval, 100*time.Millisecond),
		gomonkey.ApplyFunc(etcd3.GetRouterEtcdClient, func() *etcd3.EtcdClient {
			return &etcd3.EtcdClient{Client: &clientv3.Client{}}
		}),
	}
	defer func() {
		for _, p := range patches {
			p.Reset()
		}
		config.GlobalConfig.EnableRollout = enableRollout
		config.GlobalConfig.SchedulerDiscovery.RegisterMode = types.RegisterTypeSelf
	}()
	convey.Convey("test RegisterToEtcd", t, func() {
		config.GlobalConfig.SchedulerDiscovery.RegisterMode = types.RegisterTypeSelf
		config.GlobalConfig.EnableRollout = false
		convey.Convey("baseline", func() {
			p := gomonkey.ApplyFunc(getInstanceKeyAndValue, func() (string, string, error) {
				return "a", "b", nil
			})
			defer p.Reset()
			p2 := gomonkey.ApplyFunc((*etcd3.EtcdRegister).Register, func(_ *etcd3.EtcdRegister) error {
				return nil
			})
			defer p2.Reset()
			ch := make(chan struct{})
			err := RegisterToEtcd(ch)
			convey.So(err, convey.ShouldBeNil)
		})
		convey.Convey("get key failed", func() {
			p := gomonkey.ApplyFunc(getInstanceKeyAndValue, func() (string, string, error) {
				return "", "", fmt.Errorf("error")
			})
			defer p.Reset()
			ch := make(chan struct{})
			err := RegisterToEtcd(ch)
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("register failed", func() {
			p := gomonkey.ApplyFunc(getInstanceKeyAndValue, func() (string, string, error) {
				return "a", "b", nil
			})
			defer p.Reset()
			p2 := gomonkey.ApplyFunc((*etcd3.EtcdRegister).Register, func(_ *etcd3.EtcdRegister) error {
				return fmt.Errorf("error")
			})
			defer p2.Reset()
			ch := make(chan struct{})
			err := RegisterToEtcd(ch)
			convey.So(err, convey.ShouldNotBeNil)
		})
		config.GlobalConfig.SchedulerDiscovery.RegisterMode = types.RegisterTypeContend
		convey.Convey("register by contend", func() {
			patches1 := []*gomonkey.Patches{
				gomonkey.ApplyFunc((*etcd3.EtcdClient).Get, func(_ *etcd3.EtcdClient, ctxInfo etcd3.EtcdCtxInfo,
					key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
					return &clientv3.GetResponse{
						Kvs: []*mvccpb.KeyValue{
							{Key: []byte("/sn/faas-scheduler/instances/cluster1/node1/aaa"), Value: []byte("invalid value")},
							{Key: []byte("/sn/faas-scheduler/instances/cluster1/node1/bbb")},
						},
					}, nil
				}),
				gomonkey.ApplyFunc((*etcd3.EtcdClient).Put, func(_ *etcd3.EtcdClient, ctxInfo etcd3.EtcdCtxInfo,
					etcdKey string, value string, opts ...clientv3.OpOption) error {
					return nil
				}),
				gomonkey.ApplyFunc((*etcd3.EtcdClient).Delete, func(_ *etcd3.EtcdClient, ctxInfo etcd3.EtcdCtxInfo,
					etcdKey string, opts ...clientv3.OpOption) error {
					return nil
				}),
				gomonkey.ApplyFunc((*etcd3.EtcdLocker).TryLock, func(_ *etcd3.EtcdLocker, key string) error {
					return nil
				}),
				gomonkey.ApplyFunc((*etcd3.EtcdLocker).GetLockedKey, func(_ *etcd3.EtcdLocker) string {
					return "/sn/faas-scheduler/instances/cluster1/node1/aaa"
				}),
			}
			defer func() {
				for _, p := range patches1 {
					p.Reset()
				}
			}()
			SetSelfInstanceSpec(&commontypes.InstanceSpecification{})
			ch := make(chan struct{})
			err := RegisterToEtcd(ch)
			convey.So(err, convey.ShouldBeNil)
			close(ch)
			time.Sleep(200 * time.Millisecond)
		})
		convey.Convey("register by contend failed", func() {
			maxContendTime = 1
			selfLocker.LockedKey = ""
			selfInstanceSpec = nil
			var lockErr error
			patches1 := []*gomonkey.Patches{
				gomonkey.ApplyFunc((*etcd3.EtcdClient).Get, func(_ *etcd3.EtcdClient, ctxInfo etcd3.EtcdCtxInfo,
					key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
					return &clientv3.GetResponse{
						Kvs: []*mvccpb.KeyValue{
							{Key: []byte("/sn/faas-scheduler/instances/cluster1/node1/aaa"), Value: []byte("invalid value")},
							{Key: []byte("/sn/faas-scheduler/instances/cluster1/node1/bbb")},
						},
					}, nil
				}),
				gomonkey.ApplyFunc((*etcd3.EtcdClient).Put, func(_ *etcd3.EtcdClient, ctxInfo etcd3.EtcdCtxInfo,
					etcdKey string, value string, opts ...clientv3.OpOption) error {
					return nil
				}),
				gomonkey.ApplyFunc((*etcd3.EtcdClient).Delete, func(_ *etcd3.EtcdClient, ctxInfo etcd3.EtcdCtxInfo,
					etcdKey string, opts ...clientv3.OpOption) error {
					return nil
				}),
				gomonkey.ApplyFunc((*etcd3.EtcdLocker).TryLock, func(_ *etcd3.EtcdLocker, key string) error {
					return lockErr
				}),
			}
			defer func() {
				for _, p := range patches1 {
					p.Reset()
				}
			}()
			ch := make(chan struct{})
			lockErr = errors.New("some error")
			err := RegisterToEtcd(ch)
			convey.So(err, convey.ShouldNotBeNil)
			lockErr = nil
			err = RegisterToEtcd(ch)
			convey.So(err, convey.ShouldNotBeNil)
			os.Setenv(CurrentVersionEnvKey, "blue")
			config.GlobalConfig.EnableRollout = true
			err = RegisterToEtcd(ch)
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func TestPutInsSpecForInstanceKey(t *testing.T) {
	var (
		putErr    error
		lockedKey string
	)
	patches := []*gomonkey.Patches{
		gomonkey.ApplyFunc((*etcd3.EtcdClient).Put, func(_ *etcd3.EtcdClient, ctxInfo etcd3.EtcdCtxInfo,
			etcdKey string, value string, opts ...clientv3.OpOption) error {
			return putErr
		}),
		gomonkey.ApplyFunc((*etcd3.EtcdLocker).GetLockedKey, func(_ *etcd3.EtcdLocker) string {
			return lockedKey
		}),
	}
	defer func() {
		for _, p := range patches {
			p.Reset()
		}
	}()
	convey.Convey("Test PutInsSpecForInstanceKey", t, func() {
		locker := &etcd3.EtcdLocker{EtcdClient: &etcd3.EtcdClient{}}
		putErr = errors.New("some error")
		err := putInsSpecForInstanceKey(locker)
		convey.So(err, convey.ShouldNotBeNil)
		lockedKey = "testKey"
		err = putInsSpecForInstanceKey(locker)
		convey.So(err, convey.ShouldNotBeNil)
		selfInstanceSpec = &commontypes.InstanceSpecification{}
		err = putInsSpecForInstanceKey(locker)
		convey.So(err, convey.ShouldNotBeNil)
		putErr = nil
		err = putInsSpecForInstanceKey(locker)
		convey.So(err, convey.ShouldBeNil)
	})
}
