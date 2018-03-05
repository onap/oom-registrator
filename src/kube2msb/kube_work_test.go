/*
Copyright 2018 ZTE, Inc. and others.

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
package main

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
)

func TestAddServiceKube(t *testing.T) {
	client := newClientBookKeeper()
	msbWorkQueue := make(chan MSBWork, 10)
	client.msbQueue = msbWorkQueue

	// add ServiceTypeClusterIP
	serviceTypeClusterIP := createMockService("serviceTypeClusterIP", "192.168.10.10", kapi.ServiceTypeClusterIP)
	client.AddService(serviceTypeClusterIP)
	msbWorkValidate(t, msbWorkQueue, serviceTypeClusterIP, MSBWorkAddService, "192.168.10.10")

	// add ServiceTypeNodePort
	serviceTypeNodePort := createMockService("ServiceTypeNodePort", "192.168.10.11", kapi.ServiceTypeNodePort)
	client.AddService(serviceTypeNodePort)
	msbWorkValidate(t, msbWorkQueue, serviceTypeNodePort, MSBWorkAddService, "192.168.10.11")

	// add ServiceTypeLoadBalancer
	serviceTypeLoadBalancer := createMockService("ServiceTypeLoadBalancer", "192.168.10.12", kapi.ServiceTypeLoadBalancer)
	client.AddService(serviceTypeLoadBalancer)
	msbWorkValidate(t, msbWorkQueue, serviceTypeLoadBalancer, MSBWorkAddService, "192.168.10.12")

	// exception process
	// TODO ClusterIP is not set , cannot check result for there would be no return
	serviceClusterIPNotSet := &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Annotations: map[string]string{serviceKey: "clusterIp not set"},
		},
	}
	client.AddService(serviceClusterIPNotSet)

	// TODO servicekey is not set , cannot check result for there would be no return
	serviceWithoutServiceKey := &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Annotations: map[string]string{},
		},
	}
	client.AddService(serviceWithoutServiceKey)

	// TODO service already exist , cannot check result for there would be no return
	client.AddService(serviceTypeClusterIP)
}

func createMockService(name string, ip string, serviceType kapi.ServiceType) *kapi.Service {
	service := kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Annotations: map[string]string{serviceKey: name},
		},
		Spec: kapi.ServiceSpec{
			ClusterIP: ip,
			Type:      serviceType,
		},
	}
	service.Name = name

	return &service
}

func msbWorkValidate(t *testing.T, queue <-chan MSBWork, service *kapi.Service, action MSBWorkAction, ip string) {
	work := <-queue

	if work.Action != action || work.IPAddress != ip || work.ServiceInfo != service.Name {
		t.Errorf("msbWork validate error, expect %s,%s,%s to be %s %s,%s",
			work.Action, work.IPAddress, work.ServiceInfo, action, ip, service.Name)
	}
}

func removeSingleServiceTest(t *testing.T, client *ClientBookKeeper, queue <-chan MSBWork, service *kapi.Service, ip string) {
	// add service
	client.AddService(service)
	msbWorkValidate(t, queue, service, MSBWorkAddService, ip)

	// remove service
	client.RemoveService(service)
	msbWorkValidate(t, queue, service, MSBWorkRemoveService, ip)
}

func TestRemoveServiceKube(t *testing.T) {
	client := newClientBookKeeper()
	msbWorkQueue := make(chan MSBWork, 10)
	client.msbQueue = msbWorkQueue

	// exception process
	// TODO ServiceKey not set , cannot check result for there would be no return
	serviceWithoutServiceKey := kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Annotations: map[string]string{},
		},
	}
	client.RemoveService(&serviceWithoutServiceKey)

	// TODO service not exist , cannot check result for there would be no return
	serviceNotExist := createMockService("serviceTypeClusterIP", "192.168.10.9", kapi.ServiceTypeClusterIP)
	serviceNotExist.Name = "serviceNotExist"
	client.RemoveService(serviceNotExist)

	// normal process
	// ServiceTypeClusterIP
	serviceClusterIp := createMockService("serviceTypeClusterIP", "192.168.10.10", kapi.ServiceTypeClusterIP)
	removeSingleServiceTest(t, client, msbWorkQueue, serviceClusterIp, "192.168.10.10")

	// ServiceTypeNodePort
	serviceNodePort := createMockService("ServiceTypeNodePort", "192.168.10.11", kapi.ServiceTypeNodePort)
	removeSingleServiceTest(t, client, msbWorkQueue, serviceNodePort, "192.168.10.11")

	// ServiceTypeLoadBalancer
	serviceLoadBalancer := createMockService("serviceTypeClusterIP", "192.168.10.12", kapi.ServiceTypeLoadBalancer)
	serviceLoadBalancer.Spec.LoadBalancerIP = "192.168.10.12"
	removeSingleServiceTest(t, client, msbWorkQueue, serviceLoadBalancer, "192.168.10.12")
}

func TestUpdateServiceKube(t *testing.T) {
	client := newClientBookKeeper()
	msbWorkQueue := make(chan MSBWork, 10)
	client.msbQueue = msbWorkQueue

	// exception process
	// TODO ServiceKey not set , cannot check result for there would be no return
	serviceWithoutServiceKey := kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Annotations: map[string]string{},
		},
	}
	client.UpdateService(&serviceWithoutServiceKey)

	// normal process
	// update exist service
	service := createMockService("ServiceTypeNodePort", "192.168.10.11", kapi.ServiceTypeNodePort)
	client.AddService(service)
	msbWorkValidate(t, msbWorkQueue, service, MSBWorkAddService, "192.168.10.11")
	// update service info
	service.Spec.ClusterIP = "0.0.0.0"
	client.UpdateService(service)
	msbWorkValidate(t, msbWorkQueue, service, MSBWorkRemoveService, "0.0.0.0")
	msbWorkValidate(t, msbWorkQueue, service, MSBWorkAddService, "0.0.0.0")

	// update not exist service
	notExistService := createMockService("notExistService", "192.168.10.12", kapi.ServiceTypeNodePort)
	client.UpdateService(notExistService)
	msbWorkValidate(t, msbWorkQueue, notExistService, MSBWorkAddService, "192.168.10.12")
}

func createMockPod(name string, ip string) *kapi.Pod {
	pod := kapi.Pod{
		Status: kapi.PodStatus{
			PodIP: ip,
		},
	}

	pod.Name = name
	pod.Annotations = map[string]string{serviceKey: name}

	return &pod
}

func msbWorkPodValidate(t *testing.T, queue <-chan MSBWork, pod *kapi.Pod, action MSBWorkAction) {
	work := <-queue

	if work.Action != action || work.IPAddress != pod.Status.PodIP || work.ServiceInfo != pod.Name {
		t.Errorf("expect %s,%s,%s to be %s %s,%s",
			work.Action, work.IPAddress, work.ServiceInfo, action, pod.Status.PodIP, pod.Name)
	}
}

func TestAddPodKube(t *testing.T) {
	client := newClientBookKeeper()
	msbWorkQueue := make(chan MSBWork, 10)
	client.msbQueue = msbWorkQueue

	// add ServiceTypeClusterIP
	pod := createMockPod("addPodTest", "192.168.10.10")
	client.AddPod(pod)
	msbWorkPodValidate(t, msbWorkQueue, pod, MSBWorkAddPod)
	if _, ok := client.pods[pod.Name]; !ok {
		t.Errorf("add pod error, pod not exists in client.pods")
	}

	// exception process
	// TODO servicekey is not set , cannot check result for there would be no return
	podWithoutServiceKey := &kapi.Pod{}
	client.AddPod(podWithoutServiceKey)

	// TODO pod already exist , cannot check result for there would be no return
	client.AddPod(pod)

	// pod.Name == "" || pod.Status.PodIP == ""
	podWithoutName := createMockPod("", "192.168.10.10")
	client.AddPod(podWithoutName)
	if addedPod, ok := addMap[podWithoutName.Name]; !ok || addedPod.Status.PodIP != podWithoutName.Status.PodIP {
		t.Errorf("podWithoutName didnot add to addMap")
	}

	podWithoutIp := createMockPod("podWithoutIp", "")
	client.AddPod(podWithoutIp)
	if addedPod, ok := addMap[podWithoutIp.Name]; !ok || addedPod.Status.PodIP != podWithoutIp.Status.PodIP {
		t.Errorf("podWithoutIp didnot add to addMap")
	}

}

func TestRemovePodKube(t *testing.T) {
	client := newClientBookKeeper()
	msbWorkQueue := make(chan MSBWork, 10)
	client.msbQueue = msbWorkQueue

	// exception process
	// TODO ServiceKey not set , cannot check result for there would be no return
	podWithoutServiceKey := kapi.Pod{}
	client.RemovePod(&podWithoutServiceKey)

	// TODO Pod not exist , cannot check result for there would be no return
	podNotExist := createMockPod("podNotExistTest", "192.168.10.10")
	client.RemovePod(podNotExist)

	// normal process
	pod := createMockPod("removePodTest", "192.168.10.10")
	client.AddPod(pod)
	if _, ok := client.pods[pod.Name]; !ok {
		t.Errorf("add pod error, pod not exists in client.pods")
	}
	msbWorkPodValidate(t, msbWorkQueue, pod, MSBWorkAddPod)
	client.RemovePod(pod)
	msbWorkPodValidate(t, msbWorkQueue, pod, MSBWorkRemovePod)
	if _, ok := client.pods[pod.Name]; ok {
		t.Errorf("remove pod error, pod still exists in client.pods")
	}
}

func TestUpdatePodKube(t *testing.T) {
	client := newClientBookKeeper()
	msbWorkQueue := make(chan MSBWork, 10)
	client.msbQueue = msbWorkQueue

	// exception process
	// TODO ServiceKey not set , cannot check result for there would be no return
	podWithoutServiceKey := kapi.Pod{}
	client.UpdatePod(&podWithoutServiceKey)

	// normal process
	// update exist Pod
	existPod := createMockPod("mockPod", "192.168.10.11")
	client.AddPod(existPod)
	msbWorkPodValidate(t, msbWorkQueue, existPod, MSBWorkAddPod)
	if _, ok := client.pods[existPod.Name]; !ok {
		t.Errorf("add pod error, pod not exists in client.pods")
	}
	// update service info
	existPod.Status.PodIP = "0.0.0.0"
	client.UpdatePod(existPod)
	msbWorkPodValidate(t, msbWorkQueue, existPod, MSBWorkRemovePod)
	msbWorkPodValidate(t, msbWorkQueue, existPod, MSBWorkAddPod)
	if updatedExistPod, ok := client.pods[existPod.Name]; !ok || updatedExistPod.Status.PodIP != existPod.Status.PodIP {
		t.Errorf("add pod error, pod not exists in client.pods")
	}

	// update not exist service
	notExistPod := createMockPod("mockNotExistPod", "192.168.10.11")
	if _, ok := client.pods[notExistPod.Name]; ok {
		t.Errorf("mockNotExistPod should not exist before it has been added")
	}
	client.UpdatePod(notExistPod)
	msbWorkPodValidate(t, msbWorkQueue, notExistPod, MSBWorkAddPod)
	if _, ok := client.pods[notExistPod.Name]; !ok {
		t.Errorf("mockNotExistPod should not exist before it has been added")
	}
}
