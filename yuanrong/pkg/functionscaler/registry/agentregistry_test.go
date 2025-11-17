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

// Package registry -
package registry

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewAgentRegistry_BasicInitialization(t *testing.T) {
	os.Setenv("ENABLE_AGENT_CRD_REGISTRY", "true")
	stopCh := make(chan struct{})
	registry := NewAgentRegistry(stopCh)
	assert.NotNil(t, registry.dynamicClient, "dynamic client initialization failed")
	assert.NotEqual(t, stopCh, registry.stopCh, "stop channel transfer error")
	os.Setenv("ENABLE_AGENT_CRD_REGISTRY", "")
}

func TestNewAgentRegistry_WorkQueueSetup(t *testing.T) {
	os.Setenv("ENABLE_AGENT_CRD_REGISTRY", "true")
	registry := NewAgentRegistry(make(chan struct{}))
	assert.NotNil(t, registry.workQueue, "work queue not initialized")
	os.Setenv("ENABLE_AGENT_CRD_REGISTRY", "")
}

func TestNewAgentRegistry_ConcurrentAccess(t *testing.T) {
	os.Setenv("ENABLE_AGENT_CRD_REGISTRY", "true")
	callCount := 0
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			NewAgentRegistry(make(chan struct{}))
		}()
	}
	wg.Wait()
	assert.Equal(t, 0, callCount, "dynamic client should be initialized only once")
	os.Setenv("ENABLE_AGENT_CRD_REGISTRY", "")
}

func TestProcessEvent_SuccessfulCases(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		specData  map[string]interface{}
		expected  string
	}{
		{
			"ADD event",
			SubEventTypeAdd,
			map[string]interface{}{
				"funcMetaData": map[string]interface{}{
					"functionURN": "test-urn",
				},
				"extendedMetaData": map[string]interface{}{
					"customContainerConfig": map[string]interface{}{
						"image": "nginx:latest",
					},
				},
			},
			"[ADD]",
		},
		{
			"UPDATE event",
			SubEventTypeUpdate,
			map[string]interface{}{
				"funcMetaData": map[string]interface{}{
					"functionURN": "test-urn",
				},
				"extendedMetaData": map[string]interface{}{
					"customContainerConfig": map[string]interface{}{
						"image": "nginx:latest",
					},
				},
			},
			"[UPDATE]",
		},
		{
			"DELETE event",
			SubEventTypeDelete,
			map[string]interface{}{
				"funcMetaData": map[string]interface{}{
					"functionURN": "test-urn",
				},
				"extendedMetaData": map[string]interface{}{
					"customContainerConfig": map[string]interface{}{
						"image": "nginx:latest",
					},
				},
			},
			"[DELETE]",
		},
	}

	registry := &AgentRegistry{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &crdEvent{
				eventType: tt.eventType,
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"spec": tt.specData,
					},
				},
			}
			err := registry.processEvent(event)
			assert.NoError(t, err)
		})
	}
}

func TestProcessEvent_ErrorCases(t *testing.T) {
	tests := []struct {
		name         string
		event        *crdEvent
		expectErrMsg string
	}{
		{
			"The spec field is missing",
			&crdEvent{
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{},
				},
			},
			"crd has no spec key",
		},
		{
			"Invalid spec format",
			&crdEvent{
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"spec": "invalid-string",
					},
				},
			},
			"crd has no spec key",
		},
	}
	registry := &AgentRegistry{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.processEvent(tt.event)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectErrMsg)
		})
	}
}
