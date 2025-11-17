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

// Package vpcmanager -
package vpcmanager

import (
	"encoding/json"
	"errors"
	"plugin"

	"yuanrong/pkg/common/faas_common/logger/log"
	"yuanrong/pkg/functionmanager/types"
)

// PluginVPC -
type PluginVPC struct {
	*plugin.Plugin
	createResource plugin.Symbol
	deleteResource plugin.Symbol
}

// InitVpcPlugin -
func (p *PluginVPC) InitVpcPlugin() error {
	err := p.initController()
	if err != nil {
		log.GetLogger().Errorf("failed to exec initController, error: %s", err.Error())
		return err
	}
	log.GetLogger().Infof("succeed to exec initController")
	return nil
}

func (p *PluginVPC) initController() error {
	if p.Plugin == nil {
		log.GetLogger().Errorf("pluginVpc is nil")
		return errors.New("pluginVpc is nil")
	}
	targetFunc, err := p.Plugin.Lookup("InitController")
	if err != nil {
		log.GetLogger().Errorf("failed to look up InitController, error: %s", err.Error())
		return err
	}
	if initController, ok := targetFunc.(func() error); ok {
		err = initController()
		if err != nil {
			log.GetLogger().Errorf("failed to init Controller, error: %s", err.Error())
			return err
		}
	}
	createResource, err := p.Plugin.Lookup("CreateResource")
	if err != nil {
		log.GetLogger().Errorf("failed to lookup function of CreateResource: %s", err.Error())
		return err
	}
	p.createResource = createResource
	deleteResource, err := p.Plugin.Lookup("DeleteResource")
	if err != nil {
		log.GetLogger().Errorf("failed to lookup function of DeleteResource: %s", err.Error())
		return err
	}
	p.deleteResource = deleteResource
	return nil
}

// CreateVpcResource -
func (p *PluginVPC) CreateVpcResource(requestData []byte) (types.NATConfigure, error) {
	if p.Plugin == nil {
		log.GetLogger().Errorf("pluginVpc is nil")
		return types.NATConfigure{}, errors.New("pluginVpc is nil")
	}
	patPod := types.NATConfigure{}
	if createResource, ok := p.createResource.(func(request []byte) ([]byte, error)); ok {
		request := describeRequest(requestData)
		reqInfo, err := json.Marshal(request)
		if err != nil {
			log.GetLogger().Errorf("HandleVpcFunctionInfo Marshal error: %s", err.Error())
			return types.NATConfigure{}, err
		}
		resp, err := createResource(reqInfo)
		if err != nil {
			log.GetLogger().Errorf("HandleVpcFunctionInfo createResource error: %s", err.Error())
			return types.NATConfigure{}, err
		}
		err = json.Unmarshal(resp, &patPod)
		if err != nil {
			log.GetLogger().Errorf("HandleVpcFunctionInfo Unmarshal error: %s", err.Error())
			return types.NATConfigure{}, err
		}
	} else {
		log.GetLogger().Errorf("failed to assert createResource")
		return types.NATConfigure{}, errors.New("failed to assert createResource")
	}
	return patPod, nil
}

// DeleteVpcResource -
func (p *PluginVPC) DeleteVpcResource(patPodName string) error {
	if p.Plugin == nil {
		log.GetLogger().Errorf("pluginVpc is nil")
		return errors.New("pluginVpc is nil")
	}
	if deleteResource, ok := p.deleteResource.(func(string2 string) error); ok {
		err := deleteResource(patPodName)
		if err != nil {
			log.GetLogger().Errorf("failed to delete vpc resource: %s", err.Error())
			return err
		}
	} else {
		log.GetLogger().Errorf("failed to assert deleteResource")
		return errors.New("failed to assert deleteResource")
	}
	return nil
}

// describeRequest get different requestInfo by vpcType
func describeRequest(requestData []byte) types.VpcControllerRequester {
	requestInfo := types.RequestInfo{}
	err := json.Unmarshal(requestData, &requestInfo)
	if err != nil {
		return types.VpcControllerRequester{}
	}
	vpcInfo := types.Vpc{
		Namespace: requestInfo.Namespace,
		DomainID:  requestInfo.DomainID,
	}
	delegate := types.Delegate{
		Xrole:    requestInfo.Xrole,
		AppXrole: requestInfo.AppXrole,
	}
	vpcInfo.SubnetName = requestInfo.SubnetName
	vpcInfo.VpcID = requestInfo.VpcID
	vpcInfo.SubnetID = requestInfo.SubnetID
	vpcInfo.Gateway = requestInfo.Gateway
	vpcInfo.ID = requestInfo.ID
	vpcInfo.TenantCidr = requestInfo.TenantCidr
	vpcInfo.HostVMCidr = requestInfo.HostVMCidr
	vpcInfo.VpcName = requestInfo.VpcName
	request := types.VpcControllerRequester{
		Delegate: delegate,
		Vpc:      vpcInfo,
	}
	return request
}
