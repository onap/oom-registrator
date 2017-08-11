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
// kube_work.go
package main

import (
	"encoding/json"
	"kube2consul/util/restclient"
	"log"
	"strings"
	"sync"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kselector "k8s.io/kubernetes/pkg/fields"
	klabels "k8s.io/kubernetes/pkg/labels"
)

type KubeBookKeeper interface {
	AddNode(*kapi.Node)
	RemoveNode(*kapi.Node)
	UpdateNode(*kapi.Node)
	AddService(*kapi.Service)
	RemoveService(*kapi.Service)
	UpdateService(*kapi.Service)
	AddPod(*kapi.Pod)
	RemovePod(*kapi.Pod)
	UpdatePod(*kapi.Pod)
}

type ClientBookKeeper struct {
	sync.Mutex
	KubeBookKeeper
	client      *kclient.Client
	nodes       map[string]*KubeNode
	services    map[string]*kapi.Service
	pods        map[string]*kapi.Pod
	consulQueue chan<- ConsulWork
}

type KubeNode struct {
	name        string
	readyStatus bool
	serviceIDS  map[string]string
	address     string
}

func buildServiceBaseID(nodeName string, service *kapi.Service) string {
	return nodeName + "-" + service.Name
}

func nodeReady(node *kapi.Node) bool {
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == kapi.NodeReady {
			return node.Status.Conditions[i].Status == kapi.ConditionTrue
		}
	}

	log.Println("NodeReady condition is missing from node: ", node.Name)
	return false
}

func newKubeNode() *KubeNode {
	return &KubeNode{
		name:        "",
		readyStatus: false,
		serviceIDS:  make(map[string]string),
	}
}

func newClientBookKeeper(client *kclient.Client) *ClientBookKeeper {
	return &ClientBookKeeper{
		client:   client,
		nodes:    make(map[string]*KubeNode),
		services: make(map[string]*kapi.Service),
		pods:     make(map[string]*kapi.Pod),
	}
}

func (client *ClientBookKeeper) AddNode(node *kapi.Node) {
	client.Lock()
	defer client.Unlock()
	if _, ok := client.nodes[node.Name]; ok {
		log.Printf("Node:%s already exist. skip this ADD notification.", node.Name)
		return
	}
	//Add to Node Collection
	kubeNode := newKubeNode()
	kubeNode.readyStatus = nodeReady(node)
	kubeNode.name = node.Name
	kubeNode.address = node.Status.Addresses[0].Address

	//Send request for Service Addition for node and all serviceIDS (Create Service ID here)
	if kubeNode.readyStatus {
		client.AddAllServicesToNode(kubeNode)
	}

	client.nodes[node.Name] = kubeNode
	log.Println("Added Node: ", node.Name)
}

func (client *ClientBookKeeper) AddAllServicesToNode(node *KubeNode) {
	for _, service := range client.services {
		if service.Spec.Type == kapi.ServiceTypeClusterIP {
			log.Println("ClusterIP service ignored for node binding:", service.Name)
		} else {
			client.AttachServiceToNode(node, service)
		}
	}
}

func (client *ClientBookKeeper) AttachServiceToNode(node *KubeNode, service *kapi.Service) {
	baseID := buildServiceBaseID(node.name, service)
	client.consulQueue <- ConsulWork{
		Action:  ConsulWorkAddService,
		Service: service,
		Config: DnsInfo{
			BaseID:      baseID,
			IPAddress:   node.address,
			ServiceType: service.Spec.Type,
		},
	}
	log.Println("Requesting Addition of services with Base ID: ", baseID)
	node.serviceIDS[service.Name] = baseID
}

func (client *ClientBookKeeper) RemoveNode(name string) {
	client.Lock()
	defer client.Unlock()
	if kubeNode, ok := client.nodes[name]; ok {
		//Remove All DNS for node
		client.RemoveAllServicesFromNode(kubeNode)
		//Remove Node from Collection
		delete(client.nodes, name)
	} else {
		log.Println("Attmepted to remove missing node: ", name)
	}

	log.Println("Queued Node to be removed: ", name)
}

func (client *ClientBookKeeper) RemoveAllServicesFromNode(node *KubeNode) {
	for _, service := range client.services {
		if service.Spec.Type == kapi.ServiceTypeClusterIP {
			log.Println("ClusterIP service ignored for node unbinding:", service.Name)
		} else {
			client.DetachServiceFromNode(node, service)
		}
	}
}

func (client *ClientBookKeeper) DetachServiceFromNode(node *KubeNode, service *kapi.Service) {
	if baseID, ok := node.serviceIDS[service.Name]; ok {
		client.consulQueue <- ConsulWork{
			Action: ConsulWorkRemoveService,
			Config: DnsInfo{
				BaseID: baseID,
			},
		}

		log.Println("Requesting Removal of services with Base ID: ", baseID)
		delete(node.serviceIDS, service.Name)
	}
}

func (client *ClientBookKeeper) UpdateNode(node *kapi.Node) {
	if nodeReady(node) {
		// notReady nodes also stored in client.nodes
		client.AddAllServicesToNode(client.nodes[node.Name])
	} else {
		client.RemoveAllServicesFromNode(client.nodes[node.Name])
	}
}

func (client *ClientBookKeeper) AddService(svc *kapi.Service) {
	client.Lock()
	defer client.Unlock()
	if !isUserDefinedService(svc) {
		log.Println("Not the target, skip this ADD notification for service:", svc.Name)
		return
	}

	if _, ok := client.services[svc.Name]; ok {
		log.Printf("service:%s already exist. skip this ADD notification.", svc.Name)
		return
	}

	if kapi.IsServiceIPSet(svc) {
		if svc.Spec.Type == kapi.ServiceTypeClusterIP {
			log.Println("Adding ClusterIP service:", svc.Name)
			client.consulQueue <- ConsulWork{
				Action:  ConsulWorkAddService,
				Service: svc,
				Config: DnsInfo{
					BaseID:      svc.Name,
					IPAddress:   svc.Spec.ClusterIP,
					ServiceType: svc.Spec.Type,
				},
			}
		} else {
			//Check and use local node list first
			if len(client.nodes) > 0 {
				for _, kubeNode := range client.nodes {
					if kubeNode.readyStatus {
						client.AttachServiceToNode(kubeNode, svc)
					}
				}
			} else {
				log.Println("local node list is empty, retrieve it from the api server.")
				nodes := client.client.Nodes()
				if nodeList, err := nodes.List(kapi.ListOptions{
					LabelSelector: klabels.Everything(),
					FieldSelector: kselector.Everything(),
				}); err == nil {
					for _, node := range nodeList.Items {
						if nodeReady(&node) {
							kubeNode := newKubeNode()
							kubeNode.readyStatus = nodeReady(&node)
							kubeNode.name = node.Name
							kubeNode.address = node.Status.Addresses[0].Address
							client.AttachServiceToNode(kubeNode, svc)
						}
					}
				}
			}
		}
		client.services[svc.Name] = svc
		log.Println("Queued Service to be added: ", svc.Name)
	} else {
		// if ClusterIP is not set, do not create a DNS records
		log.Printf("Skipping dns record for headless service: %s\n", svc.Name)
	}
}

func (client *ClientBookKeeper) RemoveService(svc *kapi.Service) {
	client.Lock()
	defer client.Unlock()
	if !isUserDefinedService(svc) {
		log.Println("Not the target, skip this Remove notification for service:", svc.Name)
		return
	}

	if _, ok := client.services[svc.Name]; !ok {
		log.Printf("Service:%s not exist. skip this REMOVE notification.", svc.Name)
		return
	}

	if svc.Spec.Type == kapi.ServiceTypeClusterIP {
		log.Println("Removing ClusterIP service:", svc.Name)
		//Perform All DNS Removes
		client.consulQueue <- ConsulWork{
			Action: ConsulWorkRemoveService,
			Config: DnsInfo{
				BaseID: svc.Name,
			},
		}
	} else {
		//Check and use local node list first
		if len(client.nodes) > 0 {
			for _, kubeNode := range client.nodes {
				if kubeNode.readyStatus {
					client.DetachServiceFromNode(kubeNode, svc)
				}
			}
		} else {
			log.Println("local node list is empty, retrieve it from the api server. sync it later.")
		}
	}
	delete(client.services, svc.Name)
	log.Println("Queued Service to be removed: ", svc.Name)
}

func (client *ClientBookKeeper) UpdateService(svc *kapi.Service) {
	if !isUserDefinedService(svc) {
		log.Println("Not the target, skip this Update notification for service:", svc.Name)
		return
	}

	client.RemoveService(svc)
	client.AddService(svc)
}

func (client *ClientBookKeeper) AddPod(pod *kapi.Pod) {
	client.Lock()
	defer client.Unlock()
	if !isUserDefinedPod(pod) {
		log.Println("Not the target, skip this ADD notification for pod:", pod.Name)
		return
	}

	if _, ok := client.pods[pod.Name]; ok {
		log.Printf("Pod:%s already exist. skip this ADD notification.", pod.Name)
		return
	}

	//newly added Pod
	if pod.Name == "" || pod.Status.PodIP == "" {
		log.Printf("Pod:%s has neither name nor pod ip. skip this ADD notification.", pod.Name)
		addMap[pod.Name] = pod
		return
	}
	//existing Pod
	if kapi.IsPodReady(pod) {
		if *argPdmControllerUrl != "" {
			fillPdmPodIPsMap(pod)
		}
	}
	//Perform All DNS Adds
	client.consulQueue <- ConsulWork{
		Action: ConsulWorkAddPod,
		Pod:    pod,
		Config: DnsInfo{
			BaseID:    pod.Name,
			IPAddress: pod.Status.PodIP,
		},
	}
	client.pods[pod.Name] = pod
	log.Println("Queued Pod to be added: ", pod.Name)
}

func (client *ClientBookKeeper) RemovePod(pod *kapi.Pod) {
	client.Lock()
	defer client.Unlock()
	if !isUserDefinedPod(pod) {
		log.Println("Not the target, skip this Remove notification for pod:", pod.Name)
		return
	}

	if _, ok := client.pods[pod.Name]; !ok {
		log.Printf("Pod:%s not exist. skip this REMOVE notification.", pod.Name)
		return
	}
	//Perform All DNS Removes
	client.consulQueue <- ConsulWork{
		Action: ConsulWorkRemovePod,
		Config: DnsInfo{
			BaseID: pod.Name,
		},
	}
	delete(client.pods, pod.Name)
	log.Println("Queued Pod to be removed: ", pod.Name)
}

func (client *ClientBookKeeper) UpdatePod(pod *kapi.Pod) {
	if !isUserDefinedPod(pod) {
		log.Println("Not the target, skip this Update notification for pod:", pod.Name)
		return
	}

	if *argPdmControllerUrl != "" {
		fillPdmPodIPsMap(pod)
	}
	client.RemovePod(pod)
	client.AddPod(pod)
}

func (client *ClientBookKeeper) Sync() {
	nodes := client.client.Nodes()
	if nodeList, err := nodes.List(kapi.ListOptions{
		LabelSelector: klabels.Everything(),
		FieldSelector: kselector.Everything(),
	}); err == nil {
		for name, _ := range client.nodes {
			if !containsNodeName(name, nodeList) {
				log.Printf("Bookkeeper has node: %s that does not exist in api server", name)
				client.RemoveNode(name)
			}
		}
	}

	svcs := client.client.Services(kapi.NamespaceAll)
	if svcList, err := svcs.List(kapi.ListOptions{
		LabelSelector: klabels.Everything(),
		FieldSelector: kselector.Everything(),
	}); err == nil {
		for name, svc := range client.services {
			if !containsSvcName(name, svcList) {
				log.Printf("Bookkeeper has service: %s that does not exist in api server", name)
				client.RemoveService(svc)
			}
		}
	}

	pods := client.client.Pods(kapi.NamespaceAll)
	if podList, err := pods.List(kapi.ListOptions{
		LabelSelector: klabels.Everything(),
		FieldSelector: kselector.Everything(),
	}); err == nil {
		for name, pod := range client.pods {
			if !containsPodName(name, podList) {
				log.Printf("Bookkeeper has pod: %s that does not exist in api server", name)
				if _, ok := deleteMap[pod.Name]; ok {
					log.Println("Missing remove notification for Pod: ", pod.Name)
					delete(deleteMap, pod.Name)
				}
				client.RemovePod(pod)
			}
		}
		//It was observed that the kube notification may lost after the restarting of kube master.
		//Currently this is only done for Pod. same for Node and Service will be done when required.
		for _, pod := range podList.Items {
			if _, ok := client.pods[pod.ObjectMeta.Name]; !ok {
				if kapi.IsPodReady(&pod) {
					log.Println("Missing add/update notification for Pod: ", pod.ObjectMeta.Name)
					client.AddPod(&pod)
				}
			}
		}
	}
}

func containsPodName(name string, pods *kapi.PodList) bool {
	for _, pod := range pods.Items {
		if pod.ObjectMeta.Name == name {
			return true
		}
	}
	return false
}

func containsSvcName(name string, svcs *kapi.ServiceList) bool {
	for _, svc := range svcs.Items {
		if svc.ObjectMeta.Name == name {
			return true
		}
	}
	return false
}

func containsNodeName(name string, nodes *kapi.NodeList) bool {
	for _, node := range nodes.Items {
		if node.ObjectMeta.Name == name {
			return true
		}
	}
	return false
}

func isUserDefinedPod(pod *kapi.Pod) bool {
	for _, c := range pod.Spec.Containers {
		for _, e := range c.Env {
			if strings.HasPrefix(e.Name, "SERVICE_") {
				if strings.Contains(e.Name, "_NAME") {
					return true
				}
			}
		}
	}
	return false
}

func isUserDefinedService(svc *kapi.Service) bool {
	for name, _ := range svc.ObjectMeta.Annotations {
		if strings.HasPrefix(name, "SERVICE_") {
			if strings.Contains(name, "_NAME") {
				return true
			}
		}
	}
	return false
}

func fillPdmPodIPsMap(pod *kapi.Pod) {
	base := *argPdmControllerUrl
	resUrl := podUrl + "/" + pod.Namespace + "/pods"
	rclient := restclient.NewRESTClient(base, resUrl, pod.Name)
	//try 10 times at maximum
	for i := 0; i < 10; i++ {
		log.Printf("try REST get the PDM PodIP for pod:%s at %d time..", pod.Name, i)
		buf, err := rclient.Get()
		if err != nil {
			log.Printf("request PDM PodIP info for Pod:%s with error:%v", pod.Name, err)
			time.Sleep(6 * 1e9) //sleep for 6 secs
			continue
		}
		podIPInfo := PdmPodIPInfo{}
		json.Unmarshal(buf, &podIPInfo)
		IPInfo := podIPInfo.Pod
		IPs := IPInfo.IPs
		if len(IPs) == 0 {
			log.Printf("request PDM PodIP info for Pod:%s return empty ip list.", pod.Name)
			return
		} else {
			for _, ip := range IPs {
				if ip.IPAddress == "" {
					log.Printf("request PDM PodIP info for Pod:%s return empty ip.", pod.Name)
					return
				}
			}
			pdmPodIPsMap[pod.Name] = IPs
		}
		log.Println("successfully REST get the PDM PodIP for pod:", pod.Name)
		return
	}

	log.Println("failed to REST get the PDM PodIP for pod:", pod.Name)
}
