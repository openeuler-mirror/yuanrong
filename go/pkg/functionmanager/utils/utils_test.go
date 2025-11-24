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
package utils

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"

	"yuanrong.org/kernel/pkg/common/faas_common/types"
)

func TestGenerateErrorResponse(t *testing.T) {
	type args struct {
		errorCode    int
		errorMessage string
	}
	var a args
	a.errorCode = 0
	a.errorMessage = "0"
	resp := &types.CallHandlerResponse{
		Code:    0,
		Message: "0",
	}
	tests := []struct {
		name string
		args args
		want *types.CallHandlerResponse
	}{
		{"case1", a, resp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateErrorResponse(tt.args.errorCode, tt.args.errorMessage); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateErrorResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateSuccessResponse(t *testing.T) {
	type args struct {
		code    int
		message string
	}
	var a args
	a.code = 0
	a.message = "0"
	resp := &types.CallHandlerResponse{
		Code:    0,
		Message: "0",
	}
	tests := []struct {
		name string
		args args
		want *types.CallHandlerResponse
	}{
		{"case1", a, resp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateSuccessResponse(tt.args.code, tt.args.message); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateSuccessResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitKubeClient(t *testing.T) {
	kubeClient = nil
	convey.Convey("failed to get config", t, func() {
		defer gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
			return nil, fmt.Errorf("get config error")
		}).Reset()
		InitKubeClient()
		convey.So(kubeClient, convey.ShouldBeNil)
	})
	defer gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}).Reset()

	convey.Convey("failed to get config", t, func() {
		defer gomonkey.ApplyFunc(kubernetes.NewForConfig, func(c *rest.Config) (*kubernetes.Clientset, error) {
			return nil, fmt.Errorf("get client error")
		}).Reset()
		InitKubeClient()
		convey.So(kubeClient, convey.ShouldBeNil)
	})
	defer gomonkey.ApplyFunc(kubernetes.NewForConfig, func(c *rest.Config) (*kubernetes.Clientset, error) {
		return &kubernetes.Clientset{}, nil
	}).Reset()
	convey.Convey("init success", t, func() {
		InitKubeClient()
		convey.So(GetKubeClient(), convey.ShouldNotBeNil)
	})
}

func TestGetDeployByK8S(t *testing.T) {
	convey.Convey("kubeClient is nil", t, func() {
		_, err := GetDeployByK8S(nil, "deployName")
		convey.So(err, convey.ShouldEqual, ErrK8SClientNil)
	})

	fakeDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deployment",
			Namespace: "default",
		},
	}
	clientset := fake.NewSimpleClientset(fakeDeployment)
	clientset.PrependReactor("get", "deployments",
		func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
			getAction := action.(clienttesting.GetAction)
			fakeDeployment.Name = getAction.GetName()
			return true, fakeDeployment, nil
		})

	convey.Convey("GetDeployByK8S success", t, func() {
		_, err := GetDeployByK8S(clientset, "deployName")
		convey.So(err, convey.ShouldBeNil)
	})
}

func TestCreateDeployByK8S(t *testing.T) {
	convey.Convey("kubeClient is nil", t, func() {
		err := CreateDeployByK8S(nil, &v1.Deployment{})
		convey.So(err, convey.ShouldEqual, ErrK8SClientNil)
	})

	clientset := fake.NewSimpleClientset()
	clientset.PrependReactor("create", "deployments",
		func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
			createAction := action.(clienttesting.CreateAction)
			if createAction.GetObject().(*appsv1.Deployment).Name == "my-deployment" {
				return true, nil, errors.New("mock error")
			} else {
				fakeDeployment := createAction.GetObject().(*appsv1.Deployment)
				fakeDeployment.Name = "fake-deployment"
				return true, fakeDeployment, nil
			}
		})

	convey.Convey("create error", t, func() {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-deployment",
				Namespace: "default",
			},
		}
		err := CreateDeployByK8S(clientset, deployment)
		convey.So(err, convey.ShouldNotBeNil)
	})

	convey.Convey("create success", t, func() {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-deployment",
				Namespace: "default",
			},
		}
		err := CreateDeployByK8S(clientset, deployment)
		convey.So(err, convey.ShouldBeNil)
	})

}

func TestDeleteDeployByK8S(t *testing.T) {
	convey.Convey("kubeClient is nil", t, func() {
		err := DeleteDeployByK8S(nil, "123")
		convey.So(err, convey.ShouldEqual, ErrK8SClientNil)
	})

	clientset := fake.NewSimpleClientset()
	clientset.PrependReactor("delete", "deployments",
		func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
			deleteAction := action.(clienttesting.DeleteAction)
			if deleteAction.GetName() == "my-deployment" {
				return true, nil, errors.New("mock error")
			} else {
				return true, nil, nil
			}
		})
	convey.Convey("kubeClient is nil", t, func() {
		err := DeleteDeployByK8S(clientset, "other-deployment")
		convey.So(err, convey.ShouldBeNil)
	})
}
