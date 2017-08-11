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
// kube_service.go
package main

import (
	"log"
	"strconv"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
	kapi "k8s.io/kubernetes/pkg/api"
)

type KubeServiceAction interface {
	BuildServiceInfoMap(*kapi.ServicePort, *map[string]string)
	BuildAgentService(*ServiceInfo, DnsInfo, *kapi.ServicePort) *consulapi.AgentServiceRegistration
	BuildAgentServiceCheck(DnsInfo, *kapi.ServicePort) *consulapi.AgentServiceCheck
}

type KubeService struct {
	KubeServiceAction
	sInfos map[string]*ServiceInfo
}

func newKubeService() *KubeService {
	return &KubeService{
		sInfos: make(map[string]*ServiceInfo),
	}
}

func (kube *KubeService) BuildServiceInfoMap(sport *kapi.ServicePort, annos *map[string]string) {
	portstr := strconv.Itoa(int(sport.Port))
	nameprefix := "SERVICE_" + portstr + "_NAME"
	tagprefix := "SERVICE_" + portstr + "_TAGS"

	for name, value := range *annos {
		if strings.HasPrefix(name, nameprefix) {
			addToServiceInfoMap(name, value, Service_NAME, &kube.sInfos)
		} else if strings.HasPrefix(name, tagprefix) {
			addToServiceInfoMap(name, value, Service_TAGS, &kube.sInfos)
		}
	}
}

func (kube *KubeService) BuildAgentService(sInfo *ServiceInfo, config DnsInfo, sport *kapi.ServicePort) *consulapi.AgentServiceRegistration {
	name := sInfo.Name
	var tagslice []string
	tagslice = strings.Split(sInfo.Tags, ",")

	for _, elem := range tagslice {
		var elemslice []string
		elemslice = strings.Split(elem, ":")
		if elemslice[0] == PROTOCOL {
			switch elemslice[1] {
			case PROTOCOL_UI:
				name = PREFIX_UI + name
			default:
				log.Println("regular protocol:", elemslice[1])
			}
			break
		}
	}

	asrID := config.BaseID + "-" + strconv.Itoa(int(sport.Port)) + "-" + name

	regPort := sport.NodePort
	if config.ServiceType == kapi.ServiceTypeClusterIP {
		regPort = sport.Port
	}

	return &consulapi.AgentServiceRegistration{
		ID:      asrID,
		Name:    name,
		Address: config.IPAddress,
		Port:    int(regPort),
		Tags:    tagslice,
	}
}

func (kube *KubeService) BuildAgentServiceCheck(config DnsInfo, sport *kapi.ServicePort) *consulapi.AgentServiceCheck {
	log.Println("Creating service check for: ", config.IPAddress, " on Port: ", sport.NodePort)
	return &consulapi.AgentServiceCheck{
		TCP:      config.IPAddress + ":" + strconv.Itoa(int(sport.NodePort)),
		Interval: "60s",
	}
}
