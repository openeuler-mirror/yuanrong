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
	"context"
	"errors"

	"k8s.io/api/apps/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"yuanrong.org/kernel/pkg/common/faas_common/logger/log"
	"yuanrong.org/kernel/pkg/common/faas_common/types"
)

const (
	defaultPodNameLen = 3
)

var (
	kubeClient kubernetes.Interface
	// ErrK8SClientNil -
	ErrK8SClientNil = errors.New("kubernetes client is nil")
)

// GenerateErrorResponse -
func GenerateErrorResponse(errorCode int, errorMessage string) *types.CallHandlerResponse {
	return &types.CallHandlerResponse{
		Code:    errorCode,
		Message: errorMessage,
	}
}

// GenerateSuccessResponse -
func GenerateSuccessResponse(code int, message string) *types.CallHandlerResponse {
	return &types.CallHandlerResponse{
		Code:    code,
		Message: message,
	}
}

// InitKubeClient initializes kubernetes client
func InitKubeClient() error {
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		log.GetLogger().Errorf("failed to get token and ca for kubernetes, error: %s", err.Error())
		return err
	}

	kubeClient, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.GetLogger().Errorf("failed to create kubernetes client, error: %s", err.Error())
		return err
	}
	log.GetLogger().Infof("succeed to create kubeClient")
	return nil
}

// GetKubeClient return kubernetes client
func GetKubeClient() kubernetes.Interface {
	return kubeClient
}

// GetDeployByK8S get deployment by k8s
func GetDeployByK8S(k8sClient kubernetes.Interface, deployName string) (*v1.Deployment, error) {
	if k8sClient == nil {
		log.GetLogger().Errorf("failed to get k8sClient")
		return nil, ErrK8SClientNil
	}
	return k8sClient.AppsV1().Deployments("default").Get(context.TODO(), deployName, metav1.GetOptions{})
}

// CreateDeployByK8S create deployment by k8s
func CreateDeployByK8S(k8sClient kubernetes.Interface, deploy *v1.Deployment) error {
	if k8sClient == nil {
		log.GetLogger().Errorf("failed to get k8sClient")
		return ErrK8SClientNil
	}
	_, err := k8sClient.AppsV1().Deployments("default").Create(context.TODO(), deploy, metav1.CreateOptions{})
	if err != nil && !k8serror.IsAlreadyExists(err) {
		log.GetLogger().Errorf("failed to create deploy %s", err.Error())
		return err
	}
	return nil
}

// DeleteDeployByK8S delete deployment by k8s
func DeleteDeployByK8S(k8sClient kubernetes.Interface, name string) error {
	if k8sClient == nil {
		log.GetLogger().Errorf("failed to get k8sClient")
		return ErrK8SClientNil
	}
	return k8sClient.AppsV1().Deployments("default").Delete(context.TODO(), name, metav1.DeleteOptions{})
}
