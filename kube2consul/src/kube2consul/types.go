/*
Copyright 2017 ZTE, Inc. and others.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// types.go
package main

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

type KubeWorkAction string

const (
	KubeWorkAddNode       KubeWorkAction = "AddNode"
	KubeWorkRemoveNode    KubeWorkAction = "RemoveNode"
	KubeWorkUpdateNode    KubeWorkAction = "UpdateNode"
	KubeWorkAddService    KubeWorkAction = "AddService"
	KubeWorkRemoveService KubeWorkAction = "RemoveService"
	KubeWorkUpdateService KubeWorkAction = "UpdateService"
	KubeWorkAddPod        KubeWorkAction = "AddPod"
	KubeWorkRemovePod     KubeWorkAction = "RemovePod"
	KubeWorkUpdatePod     KubeWorkAction = "UpdatePod"
	KubeWorkSync          KubeWorkAction = "Sync"
)

type KubeWork struct {
	Action  KubeWorkAction
	Node    *kapi.Node
	Service *kapi.Service
	Pod     *kapi.Pod
}

type ConsulWorkAction string

const (
	ConsulWorkAddService    ConsulWorkAction = "AddService"
	ConsulWorkRemoveService ConsulWorkAction = "RemoveService"
	ConsulWorkAddPod        ConsulWorkAction = "AddPod"
	ConsulWorkRemovePod     ConsulWorkAction = "RemovePod"
	ConsulWorkSyncDNS       ConsulWorkAction = "SyncDNS"
)

type ConsulWork struct {
	Action  ConsulWorkAction
	Service *kapi.Service
	Pod     *kapi.Pod
	Config  DnsInfo
}

type DnsInfo struct {
	BaseID      string
	IPAddress   string
	ServiceType kapi.ServiceType
}

type ServiceInfo struct {
	Name     string
	Tags     string
	Labels   string
	MData    string
	NS       string
	TTL      string
	HTTP     string
	TCP      string
	CMD      string
	Script   string
	Timeout  string
	Interval string
	IPs      []PodIP
}

type PdmPodIPInfo struct {
	Pod PodIPInfo `json:"pod"`
}

type PodIPInfo struct {
	Name string `json:"name"`
	IPs  []PodIP `json:"ips"`
}

type PodIP struct {
	NetworkPlane string `json:"network_plane_name"`
	IPAddress    string `json:"ip_address"`
}
