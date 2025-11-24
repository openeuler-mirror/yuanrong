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

// Package types -
package types

import (
	"yuanrong.org/kernel/pkg/common/faas_common/alarm"
	"yuanrong.org/kernel/pkg/common/faas_common/crypto"
	"yuanrong.org/kernel/pkg/common/faas_common/etcd3"
)

const (
	// KillSignalVal Kill instances of job
	KillSignalVal = 2 // Kill instances of job
)

// ManagerConfig is the config used by faas frontend function
type ManagerConfig struct {
	HTTPSEnable          bool             `json:"httpsEnable" valid:"optional"`
	FunctionCapability   int              `json:"functionCapability" valid:"optional"`
	AuthenticationEnable bool             `json:"authenticationEnable" valid:"optional"`
	LeaseRenewMinute     int              `json:"leaseRenewMinute" valid:"optional"`
	RouterEtcd           etcd3.EtcdConfig `json:"routerEtcd" valid:"optional"`
	MetaEtcd             etcd3.EtcdConfig `json:"metaEtcd" valid:"optional"`
	AlarmConfig          alarm.Config     `json:"alarmConfig" valid:"optional"`
	SccConfig            crypto.SccConfig `json:"sccConfig" valid:"optional"`
}

// NATConfigure include nat configure info for function-agent
type NATConfigure struct {
	ContainerCidr  string              `json:"containerCidr"`
	HostVMCidr     string              `json:"hostVmCidr"`
	PatContainerIP string              `json:"patContainerIP"`
	PatVMIP        string              `json:"patVmIP"`
	PatPortIP      string              `json:"patPortIP"`
	PatMacAddr     string              `json:"patMacAddr"`
	PatGateway     string              `json:"patGateway"`
	PatPodName     string              `json:"patPodName"`
	TenantCidr     string              `json:"tenantCidr"`
	NatSubnetList  map[string][]string `json:"natSubnetList"`
	IsDeleted      bool                `json:"isDeleted"`
	IsNewCreated   bool                `json:"isNewCreated"`
}

// VpcControllerRequester define request message for vpc_controller
type VpcControllerRequester struct {
	TraceID  string   `json:"trace_id"`
	Delegate Delegate `json:"delegate"`
	Vpc      Vpc      `json:"vpc"`
}

// Delegate include Xrole and AppXrole
type Delegate struct {
	Xrole    string `json:"xrole,omitempty"`
	AppXrole string `json:"app_xrole,omitempty"`
}

// Vpc include info of function vpc
type Vpc struct {
	ID         string `json:"id,omitempty"`
	DomainID   string `json:"domain_id,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	VpcName    string `json:"vpc_name,omitempty"`
	VpcID      string `json:"vpc_id,omitempty"`
	SubnetName string `json:"subnet_name,omitempty"`
	SubnetID   string `json:"subnet_id,omitempty"`
	TenantCidr string `json:"tenant_cidr,omitempty"`
	HostVMCidr string `json:"host_vm_cidr,omitempty"`
	Gateway    string `json:"gateway,omitempty"`
	Xrole      string `json:"xrole,omitempty"`
}

// RequestInfo include info of request Option Create
type RequestInfo struct {
	ID         string `json:"id,omitempty"`
	DomainID   string `json:"domain_id,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	VpcName    string `json:"vpc_name,omitempty"`
	VpcID      string `json:"vpc_id,omitempty"`
	SubnetName string `json:"subnet_name,omitempty"`
	SubnetID   string `json:"subnet_id,omitempty"`
	TenantCidr string `json:"tenant_cidr,omitempty"`
	HostVMCidr string `json:"host_vm_cidr,omitempty"`
	Gateway    string `json:"gateway,omitempty"`
	Xrole      string `json:"xrole,omitempty"`
	AppXrole   string `json:"app_xrole,omitempty"`
}

// PullTriggerRequestInfo include info of pullTrigger Option Create
type PullTriggerRequestInfo struct {
	PodName       string `json:"pod_name"`
	Image         string `json:"image"`
	DomainID      string `json:"domain_id,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	VpcName       string `json:"vpc_name,omitempty"`
	VpcID         string `json:"vpc_id,omitempty"`
	SubnetName    string `json:"subnet_name,omitempty"`
	SubnetID      string `json:"subnet_id,omitempty"`
	TenantCidr    string `json:"tenant_cidr,omitempty"`
	HostVMCidr    string `json:"host_vm_cidr,omitempty"`
	ContainerCidr string `json:"container_cidr"`
	Gateway       string `json:"gateway,omitempty"`
	Xrole         string `json:"xrole,omitempty"`
	AppXrole      string `json:"app_xrole,omitempty"`
}

// ReportInfo include info of request Option report
type ReportInfo struct {
	PatPodName string `json:"patPodName,omitempty"`
	InstanceID string `json:"instanceID,omitempty"`
}

// DeleteInfo include info of request Option delete
type DeleteInfo struct {
	InstanceID string `json:"instanceID,omitempty"`
}

// PullTriggerDeleteInfo include info of pullTrigger Option delete
type PullTriggerDeleteInfo struct {
	PodName string `json:"pod_name,omitempty"`
}
