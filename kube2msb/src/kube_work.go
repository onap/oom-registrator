package main

import (
	"log"
	"sync"

	kapi "k8s.io/kubernetes/pkg/api"
)

type KubeBookKeeper interface {
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
	services map[string]*kapi.Service
	pods     map[string]*kapi.Pod
	msbQueue chan<- MSBWork
}

func newClientBookKeeper() *ClientBookKeeper {
	return &ClientBookKeeper{
		services: make(map[string]*kapi.Service),
		pods:     make(map[string]*kapi.Pod),
	}
}

func (client *ClientBookKeeper) AddService(svc *kapi.Service) {
	client.Lock()
	defer client.Unlock()
	if _, ok := svc.ObjectMeta.Annotations[serviceKey]; !ok {
		log.Println("Not the target, skip this ADD notification for service:", svc.Name)
		return
	}

	if _, ok := client.services[svc.Name]; ok {
		log.Printf("service:%s already exist. skip this ADD notification.", svc.Name)
		return
	}

	if kapi.IsServiceIPSet(svc) {
		if svc.Spec.Type == kapi.ServiceTypeClusterIP || svc.Spec.Type == kapi.ServiceTypeNodePort {
			log.Printf("Adding %s service:%s", svc.Spec.Type, svc.Name)
			client.msbQueue <- MSBWork{
				Action:      MSBWorkAddService,
				ServiceInfo: svc.ObjectMeta.Annotations[serviceKey],
				IPAddress:   svc.Spec.ClusterIP,
			}
		} else if svc.Spec.Type == kapi.ServiceTypeLoadBalancer {
			log.Println("Adding LoadBalancerIP service:", svc.Name)
			client.msbQueue <- MSBWork{
				Action:      MSBWorkAddService,
				ServiceInfo: svc.ObjectMeta.Annotations[serviceKey],
				IPAddress:   svc.Spec.LoadBalancerIP,
			}
		} else {
			log.Printf("Service Type:%s for Service:%s is not supported", svc.Spec.Type, svc.Name)
			return
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
	if _, ok := svc.ObjectMeta.Annotations[serviceKey]; !ok {
		log.Println("Not the target, skip this Remove notification for service:", svc.Name)
		return
	}

	if _, ok := client.services[svc.Name]; !ok {
		log.Printf("Service:%s not exist. skip this REMOVE notification.", svc.Name)
		return
	}

	if svc.Spec.Type == kapi.ServiceTypeClusterIP || svc.Spec.Type == kapi.ServiceTypeNodePort {
		log.Printf("Removing %s service:%s", svc.Spec.Type, svc.Name)
		//Perform All DNS Removes
		client.msbQueue <- MSBWork{
			Action:      MSBWorkRemoveService,
			ServiceInfo: svc.ObjectMeta.Annotations[serviceKey],
			IPAddress:   svc.Spec.ClusterIP,
		}
	} else if svc.Spec.Type == kapi.ServiceTypeLoadBalancer {
		log.Println("Removing LoadBalancerIP service:", svc.Name)
		client.msbQueue <- MSBWork{
			Action:      MSBWorkRemoveService,
			ServiceInfo: svc.ObjectMeta.Annotations[serviceKey],
			IPAddress:   svc.Spec.LoadBalancerIP,
		}
	} else {
		log.Printf("Service Type:%s for Service:%s is not supported", svc.Spec.Type, svc.Name)
		return
	}
	delete(client.services, svc.Name)
	log.Println("Queued Service to be removed: ", svc.Name)
}

func (client *ClientBookKeeper) UpdateService(svc *kapi.Service) {
	if _, ok := svc.ObjectMeta.Annotations[serviceKey]; !ok {
		log.Println("Not the target, skip this Update notification for service:", svc.Name)
		return
	}

	client.RemoveService(svc)
	client.AddService(svc)
}

func (client *ClientBookKeeper) AddPod(pod *kapi.Pod) {
	client.Lock()
	defer client.Unlock()
	if _, ok := pod.Annotations[serviceKey]; !ok {
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

	//Perform All DNS Adds
	client.msbQueue <- MSBWork{
		Action:      MSBWorkAddPod,
		ServiceInfo: pod.Annotations[serviceKey],
		IPAddress:   pod.Status.PodIP,
	}
	client.pods[pod.Name] = pod
	log.Println("Queued Pod to be added: ", pod.Name)
}

func (client *ClientBookKeeper) RemovePod(pod *kapi.Pod) {
	client.Lock()
	defer client.Unlock()
	if _, ok := pod.Annotations[serviceKey]; !ok {
		log.Println("Not the target, skip this Remove notification for pod:", pod.Name)
		return
	}

	if _, ok := client.pods[pod.Name]; !ok {
		log.Printf("Pod:%s not exist. skip this REMOVE notification.", pod.Name)
		return
	}
	//Perform All DNS Removes
	client.msbQueue <- MSBWork{
		Action:      MSBWorkRemovePod,
		ServiceInfo: pod.Annotations[serviceKey],
		IPAddress:   pod.Status.PodIP,
	}
	delete(client.pods, pod.Name)
	log.Println("Queued Pod to be removed: ", pod.Name)
}

func (client *ClientBookKeeper) UpdatePod(pod *kapi.Pod) {
	if _, ok := pod.Annotations[serviceKey]; !ok {
		log.Println("Not the target, skip this Update notification for pod:", pod.Name)
		return
	}

	client.RemovePod(pod)
	client.AddPod(pod)
}
