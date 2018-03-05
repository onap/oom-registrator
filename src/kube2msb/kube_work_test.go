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
