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
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"yuanrong/pkg/common/faas_common/k8sclient"
	"yuanrong/pkg/common/faas_common/logger/log"
	"yuanrong/pkg/functionscaler/types"
	"yuanrong/pkg/functionscaler/utils"
)

const (
	// CRD-related configurations are consistent with the AI cloud platform
	agentGroup     = "yr.cap.io"
	agentVersion   = "v1"
	agentResource  = "yrtasks"
	cedEventsQueue = "crdEventsQueue"

	// consistency with 1.x
	defaultMaxRetryTimes = 12
)

// AgentRegistry watches agent event of CR
type AgentRegistry struct {
	dynamicClient   dynamic.Interface
	informerFactory dynamicinformer.DynamicSharedInformerFactory
	workQueue       workqueue.RateLimitingInterface
	funcSpecs       map[string]*types.FunctionSpecification
	stopCh          <-chan struct{}
	sync.RWMutex
}

// crdEvent include eventType and obj
type crdEvent struct {
	eventType EventType
	obj       *unstructured.Unstructured
}

// NewAgentRegistry will create AgentRegistry
func NewAgentRegistry(stopCh <-chan struct{}) *AgentRegistry {
	// prevent component startup exceptions when the YAML file for deployment permissions is not configured
	if os.Getenv("ENABLE_AGENT_CRD_REGISTRY") == "" {
		return nil
	}
	dynamicClient := k8sclient.NewDynamicClient()
	// Different CR events share the same rate-limiting queue
	workQueue := workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(),
		cedEventsQueue,
	)
	agentRegistry := &AgentRegistry{
		dynamicClient:   dynamicClient,
		informerFactory: dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, time.Minute),
		workQueue:       workQueue,
		funcSpecs:       make(map[string]*types.FunctionSpecification, utils.DefaultMapSize),
		stopCh:          stopCh,
	}
	return agentRegistry
}

// RunWatcher will start CR watch process
func (ar *AgentRegistry) RunWatcher() {
	crdGVR := schema.GroupVersionResource{
		Group:    agentGroup,
		Version:  agentVersion,
		Resource: agentResource,
	}
	crdInformer := ar.informerFactory.ForResource(crdGVR).Informer()
	ar.setupEventHandlers(crdInformer)
	ar.startController(crdInformer)
}

// setupEventHandlers setup CRD Event Handlers
func (ar *AgentRegistry) setupEventHandlers(informer cache.SharedInformer) {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ar.enqueueEvent(SubEventTypeAdd, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if oldObj.(*unstructured.Unstructured).GetResourceVersion() !=
				newObj.(*unstructured.Unstructured).GetResourceVersion() {
				ar.enqueueEvent(SubEventTypeUpdate, newObj)
			}
		},
		DeleteFunc: func(obj interface{}) {
			ar.enqueueEvent(SubEventTypeDelete, obj)
		},
	})
}

// enqueueEvent handle crd event enqueue
func (ar *AgentRegistry) enqueueEvent(eventType EventType, obj interface{}) {
	unstructObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		log.GetLogger().Errorf("failed to assert crd event")
		return
	}
	ar.workQueue.Add(&crdEvent{
		eventType: eventType,
		obj:       unstructObj,
	})
}

// startController start crd Controller
func (ar *AgentRegistry) startController(informer cache.SharedInformer) {
	ctx, _ := context.WithCancel(context.Background())
	go ar.informerFactory.Start(ctx.Done())
	go ar.processQueue()
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		log.GetLogger().Warnf("failed to sync crd cache")
	}
}

// processQueue process crd event Queue
func (ar *AgentRegistry) processQueue() {
	for {
		item, shutdown := ar.workQueue.Get()
		if shutdown {
			return
		}
		event, ok := item.(*crdEvent)
		if !ok {
			log.GetLogger().Warnf("invalid crd event")
			ar.workQueue.Forget(item)
			continue
		}
		if err := ar.processEvent(event); err != nil {
			// Limited number of retries
			if ar.workQueue.NumRequeues(item) < defaultMaxRetryTimes {
				log.GetLogger().Warnf("process crd event error: %s, retry", err.Error())
				ar.workQueue.AddRateLimited(item)
			}
		} else {
			ar.workQueue.Forget(item)
		}
		ar.workQueue.Done(item)
	}
}

// processEvent process cr add update delete Event
func (ar *AgentRegistry) processEvent(event *crdEvent) error {
	spec, ok := event.obj.UnstructuredContent()["spec"].(map[string]interface{})
	if !ok {
		return errors.New("crd has no spec key")
	}

	var info types.FunctionSpecification
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(spec, &info); err != nil {
		log.GetLogger().Errorf("failed to convert crd spec: %s", err.Error())
		return err
	}

	switch event.eventType {
	case SubEventTypeAdd:
		log.GetLogger().Infof("[ADD] FunctionURN: %s, Image: %s", info.FuncMetaData.FunctionURN,
			info.ExtendedMetaData.CustomContainerConfig.Image)
	case SubEventTypeUpdate:
		log.GetLogger().Infof("[UPDATE] FunctionURN: %s, Image: %s", info.FuncMetaData.FunctionURN,
			info.ExtendedMetaData.CustomContainerConfig.Image)
	case SubEventTypeDelete:
		log.GetLogger().Infof("[DELETE] FunctionURN: %s", info.FuncMetaData.FunctionURN)
	default:
		log.GetLogger().Warnf("invalid event type")
	}
	return nil
}
