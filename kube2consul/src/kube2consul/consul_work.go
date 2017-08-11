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
// consul_work.go
package main

import (
	"log"
	"strings"
	"sync"

	consulapi "github.com/hashicorp/consul/api"
	kapi "k8s.io/kubernetes/pkg/api"
)

type ConsulWorker interface {
	AddService(config DnsInfo, service *kapi.Service)
	RemoveService(config DnsInfo)
	AddPod(config DnsInfo, pod *kapi.Pod)
	RemovePod(config DnsInfo)
	SyncDNS()
}

type ConsulAgentWorker struct {
	sync.Mutex
	ConsulWorker
	ids   map[string][]*consulapi.AgentServiceRegistration
	agent *consulapi.Client
}

func newConsulAgentWorker(client *consulapi.Client) *ConsulAgentWorker {
	return &ConsulAgentWorker{
		agent: client,
		ids:   make(map[string][]*consulapi.AgentServiceRegistration),
	}
}

func (client *ConsulAgentWorker) AddService(config DnsInfo, service *kapi.Service) {
	client.Lock()
	defer client.Unlock()
	log.Println("Starting Add Service for: ", config.BaseID)

	if config.IPAddress == "" || config.BaseID == "" {
		log.Println("Service Info is not valid for AddService")
		return
	}

	annos := service.ObjectMeta.Annotations
	//only register service with annotations
	if len(annos) == 0 {
		log.Println("no Annotation defined for this service:", service.Name)
		return
	}

	for _, port := range service.Spec.Ports {
		kubeService := newKubeService()
		kubeService.BuildServiceInfoMap(&port, &annos)

		if len(kubeService.sInfos) == 0 {
			log.Println("no Service defined for this port:", port.Port)
			continue
		}

		//remove service with no name
		for id, s := range kubeService.sInfos {
			if len(s.Name) == 0 {
				if id == "0" {
					log.Printf("SERVICE_%d_NAME not set, ignore this service.", port.Port)
				} else {
					log.Printf("SERVICE_%d_NAME_%s not set, ignore this service.", port.Port, id)
				}
				delete(kubeService.sInfos, id)
			}
		}

		//test again
		if len(kubeService.sInfos) == 0 {
			log.Println("no Service defined for this port:", port.Port)
			continue
		}

		for _, s := range kubeService.sInfos {
			asr := kubeService.BuildAgentService(s, config, &port)
			if *argChecks && port.Protocol == "TCP" && config.ServiceType != kapi.ServiceTypeClusterIP {
				//Create Check if neeeded
				asr.Check = kubeService.BuildAgentServiceCheck(config, &port)
			}

			if client.agent != nil {
				//Registers with DNS
				if err := client.agent.Agent().ServiceRegister(asr); err != nil {
					log.Println("Error creating service record: ", asr.ID)
				}
			}

			//Add to IDS
			client.ids[config.BaseID] = append(client.ids[config.BaseID], asr)
		}
	}
	log.Println("Added Service: ", service.Name)
}

func (client *ConsulAgentWorker) RemoveService(config DnsInfo) {
	client.Lock()
	defer client.Unlock()
	if ids, ok := client.ids[config.BaseID]; ok {
		for _, asr := range ids {
			if client.agent != nil {
				if err := client.agent.Agent().ServiceDeregister(asr.ID); err != nil {
					log.Println("Error removing service: ", err)
				}
			}
		}
		delete(client.ids, config.BaseID)
	} else {
		log.Println("Requested to remove non-existant BaseID DNS of:", config.BaseID)
	}
}

func (client *ConsulAgentWorker) AddPod(config DnsInfo, pod *kapi.Pod) {
	client.Lock()
	defer client.Unlock()
	log.Println("Starting Add Pod for: ", config.BaseID)

	//double check if have Pod IPs so far in Paas mode
	if *argPdmControllerUrl != "" {
		if _, ok := pdmPodIPsMap[pod.Name]; !ok {
			log.Println("In Paas mode, We don't have Pod IPs to register with for Pod: ", pod.Name)
			return
		}
	}

	containerIdMap := make(map[string]string)
	buildContainerIdMap(pod, &containerIdMap)

	for _, c := range pod.Spec.Containers {

		for _, p := range c.Ports {
			podService := newPodService()
			podService.BuildServiceInfoMap(&p, &c)
			if len(podService.sInfos) == 0 {
				log.Println("no Service defined for this port:", p.ContainerPort)
				continue
			}

			//remove service with no name
			for id, s := range podService.sInfos {
				if len(s.Name) == 0 {
					if id == "0" {
						log.Printf("SERVICE_%d_NAME not set, ignore this service.", p.ContainerPort)
					} else {
						log.Printf("SERVICE_%d_NAME_%s not set, ignore this service.", p.ContainerPort, id)
					}
					delete(podService.sInfos, id)
				}
			}

			//remove service if same service exist with different protocol
			for id, s := range podService.sInfos {
				services := []*consulapi.CatalogService{}
				sn := s.Name
				//append namespace if specified
				if len(s.NS) != 0 {
					sn = sn + "-" + s.NS
				}
				var tags []string
				var protocol string
				tags = strings.Split(s.Tags, ",")
				for _, v := range tags {
					var elems []string
					elems = strings.Split(v, ":")
					if elems[0] == PROTOCOL {
						protocol = elems[1]
						break
					}
				}
				//remove service with empty protocol
				if protocol == "" {
					delete(podService.sInfos, id)
					continue
				}
				
				protocol_field := "\"protocol\":\"" + protocol + "\""
				
				//query consul
				if client.agent != nil {
					same_protocol := false
					services, _, _ = client.agent.Catalog().Service(sn, "", nil)
					if services == nil || len(services) == 0 {
						continue
					}
					for _, v := range services[0].ServiceTags {
						if strings.Contains(v, protocol_field) {
							same_protocol = true
							break
						}
					}
					
					if !same_protocol {
						log.Printf("same service with different protocol already exist, ignore service:%s, protocol:%s", sn, protocol)
						delete(podService.sInfos, id)
					}
				}
			}			

			//remove service with no network plane type in tags
			if *argPdmControllerUrl != "" {
				for id, s := range podService.sInfos {
					if len(s.Tags) == 0 {
						if id == "0" {
							log.Printf("SERVICE_%d_TAGS not set, ignore this service in Paas mode.", p.ContainerPort)
						} else {
							log.Printf("SERVICE_%d_TAGS_%s not set, ignore this service in Paas mode.", p.ContainerPort, id)
						}
						delete(podService.sInfos, id)
					} else {
						var tags []string
						tags = strings.Split(s.Tags, ",")
						for _, v := range tags {
							var elems []string
							elems = strings.Split(v, ":")
							//this is saved in Tags, later will be moved to Labels??
							if elems[0] == NETWORK_PLANE_TYPE {
								types := strings.Split(elems[1], "|")
								for _, t := range types {
									ip := getIPFromPodIPsMap(t, pod.Name)
									if ip == "" {
										log.Printf("User defined network type:%s has no assigned ip for Pod:%s", t, pod.Name)
									} else {
										log.Printf("Found ip:%s for network type:%s", ip, t)
										s.IPs = append(s.IPs, PodIP{
											NetworkPlane: t,
											IPAddress:    ip,
										})
									}
								}
							}
						}

						if len(s.IPs) == 0 {
							log.Printf("In Paas mode, no IP assigned for Pod:ContainerPort->%s:%d", pod.Name, p.ContainerPort)
							delete(podService.sInfos, id)
						}
					}
				}
			}

			//test again
			if len(podService.sInfos) == 0 {
				log.Println("no Service defined for this port:", p.ContainerPort)
				continue
			}

			for _, s := range podService.sInfos {
				asrs := podService.BuildAgentService(s, config, &p, containerIdMap[c.Name])
				if client.agent != nil {
					for _, asr := range asrs {
						//Registers with DNS
						if err := client.agent.Agent().ServiceRegister(asr); err != nil {
							log.Println("Error creating service record: ", asr.ID)
						} else {
							log.Printf("Added service:instance ->%s:%s", asr.Name, asr.ID)
						}
						client.ids[config.BaseID] = append(client.ids[config.BaseID], asr)
					}
				} else {
					log.Println("Consul client is not available.")
				}
			}
		}
	}
	log.Println("Added Pod: ", pod.Name)
}

func (client *ConsulAgentWorker) RemovePod(config DnsInfo) {
	client.Lock()
	defer client.Unlock()
	if ids, ok := client.ids[config.BaseID]; ok {
		for _, asr := range ids {
			if client.agent != nil {
				if err := client.agent.Agent().ServiceDeregister(asr.ID); err != nil {
					log.Println("Error removing service: ", err)
				} else {
					log.Printf("removed service -> %s:%d", asr.Name, asr.Port)
				}
			}
		}
		delete(client.ids, config.BaseID)
		log.Println("Removed Pod: ", config.BaseID)
	} else {
		log.Println("Requested to remove non-existant BaseID DNS of:", config.BaseID)
	}
}

func (client *ConsulAgentWorker) SyncDNS() {
	if client.agent != nil {
		if services, err := client.agent.Agent().Services(); err == nil {
			for _, registered := range client.ids {
				for _, service := range registered {
					if !containsServiceId(service.ID, services) {
						log.Println("Regregistering missing service ID: ", service.ID)
						client.agent.Agent().ServiceRegister(service)
					}
				}
			}
			for _, service := range services {
				found := false
				for _, registered := range client.ids {
					for _, svc := range registered {
						if service.ID == svc.ID {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				if !found {
					log.Println("Degregistering dead service ID: ", service.ID)
					client.agent.Agent().ServiceDeregister(service.ID)
				}
			}
		} else {
			log.Println("Error retreiving services from consul during sync: ", err)
		}
	}
}

func buildContainerIdMap(pod *kapi.Pod, cmap *map[string]string) {
	for _, s := range pod.Status.ContainerStatuses {
		id := strings.TrimLeft(s.ContainerID, "docker://")
		(*cmap)[s.Name] = id
	}
}

func containsServiceId(id string, services map[string]*consulapi.AgentService) bool {
	for _, service := range services {
		if service.ID == id {
			return true
		}
	}
	return false
}

func getIPFromPodIPsMap(ptype string, podname string) string {
	ips := pdmPodIPsMap[podname]
	for _, v := range ips {
		if v.NetworkPlane == ptype {
			return v.IPAddress
		}
	}
	return ""
}
