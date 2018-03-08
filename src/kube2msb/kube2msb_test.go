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
	"os"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
)

func urlFormateValidate(t *testing.T, method string) {
	cases := []struct{ in, want string }{
		{"", ""},                                                                             // nil
		{"urlFormatError", ""},                                                               // error
		{"exmaple.com/", ""},                                                                 // Scheme == ""
		{"http://", ""},                                                                      // Host == ""
		{"http://:/", ""},                                                                    // Host == ":"
		{"http://:/msb", ""},                                                                 // Host == ":"
		{"http://192.168.1.100:8080/$var1", "http://192.168.1.100:8080/value1"},              // os.env
		{"http://192.168.1.100:8080/$var1/$var2", "http://192.168.1.100:8080/value1/value2"}, //
		{"http://192.168.1.100:8080/$var1/$var1", "http://192.168.1.100:8080/value1/value1"}, //
		{"http://192.168.1.100:8080/$var1", "http://192.168.1.100:8080/value1"},              //
		{"http://192.168.1.100:8080/msb", "http://192.168.1.100:8080/msb"},                   //
		{"http://192.168.1.100/msb", "http://192.168.1.100/msb"},                             //
		{"http://192.168.1.100/", "http://192.168.1.100/"},                                   //
		{"postgres://user:pass@host.com:5432/path?k=v#f", "postgres://user:pass@host.com:5432/path?k=v#f"},
	}

	// set os env
	os.Setenv("var1", "value1")
	os.Setenv("var2", "value2")
	os.Setenv("var3", "value3")

	for _, c := range cases {
		var got string
		if method == "getMSBUrl" {
			argMSBUrl = &c.in
			got, _ = getMSBUrl()
		} else if method == "getKubeMasterUrl" {
			argKubeMasterUrl = &c.in
			got, _ = getKubeMasterUrl()
		}

		if got != c.want {
			t.Errorf("getMSBUrl() arg %s, want %s, got %s", c.in, c.want, got)
		}
	}
}

func TestGetMSBUrl(t *testing.T) {
	urlFormateValidate(t, "getMSBUrl")
}

func TestGetKubeMasterUrl(t *testing.T) {
	urlFormateValidate(t, "getKubeMasterUrl")
}

func TestSendServiceWork(t *testing.T) {

	kubeWorkQueue := make(chan KubeWork, 1)
	serviceObj := kapi.Service{}

	cases := []KubeWorkAction{
		KubeWorkAddService,
		KubeWorkRemoveService,
		KubeWorkUpdateService,
	}

	for _, c := range cases {
		sendServiceWork(c, kubeWorkQueue, &serviceObj)
		got := <-kubeWorkQueue

		if got.Action != c {
			t.Errorf("sendServiceWork(%action, queue, service) got %gotAction", c, got.Action)
		}
	}
}

func TestSendPodWork(t *testing.T) {

	kubeWorkQueue := make(chan KubeWork, 1)
	podObj := kapi.Pod{}

	cases := []KubeWorkAction{
		KubeWorkAddPod,
		KubeWorkRemovePod,
		KubeWorkUpdatePod,
	}

	for _, c := range cases {
		sendPodWork(c, kubeWorkQueue, &podObj)
		got := <-kubeWorkQueue

		if got.Action != c {
			t.Errorf("sendPodWork(%action, queue, service) got %gotAction", c, got.Action)
		}
	}
}

func TestRunBookKeeper(t *testing.T) {
	kubeWorkQueue := make(chan KubeWork)
	msbWorkQueue := make(chan MSBWork)

	go runBookKeeper(kubeWorkQueue, msbWorkQueue)

	serviceCases := []struct {
		work      KubeWork
		ip        string
		msbAction MSBWorkAction
	}{
		{
			KubeWork{
				Action:  KubeWorkAddService,
				Service: createMockService("RunBookKeeper", "127.0.0.1", kapi.ServiceTypeClusterIP),
			},
			"127.0.0.1",
			MSBWorkAddService,
		},
		{
			KubeWork{
				Action:  KubeWorkUpdateService,
				Service: createMockService("RunBookKeeper", "127.0.0.2", kapi.ServiceTypeNodePort),
			},
			"127.0.0.2",
			MSBWorkAddService,
		},
		{
			KubeWork{
				Action:  KubeWorkRemoveService,
				Service: createMockService("RunBookKeeper", "127.0.0.3", kapi.ServiceTypeLoadBalancer),
			},
			"127.0.0.3",
			MSBWorkRemoveService,
		},
	}

	for _, c := range serviceCases {
		//		if c.work.Service.Spec.Type == kapi.ServiceTypeLoadBalancer {
		//			c.work.Service.Spec.LoadBalancerIP = "127.0.0.4"
		//			c.ip = "127.0.0.4"
		//		}
		kubeWorkQueue <- c.work

		if c.work.Action == KubeWorkUpdateService {
			msbWorkValidate(t, msbWorkQueue, c.work.Service, MSBWorkRemoveService, c.ip)
			msbWorkValidate(t, msbWorkQueue, c.work.Service, MSBWorkAddService, c.ip)
		} else {
			msbWorkValidate(t, msbWorkQueue, c.work.Service, c.msbAction, c.ip)
		}
	}

	podCases := []struct {
		work      KubeWork
		msbAction MSBWorkAction
		ip        string
	}{
		{
			KubeWork{
				Action: KubeWorkAddPod,
				Pod:    createMockPod("RunBookKeeper", "192.168.1.2"),
			},
			MSBWorkAddPod,
			"192.168.1.2",
		},
		{
			KubeWork{
				Action: KubeWorkUpdatePod,
				Pod:    createMockPod("RunBookKeeper", "192.168.1.3"),
			},
			MSBWorkAddPod,
			"",
		},
		{
			KubeWork{
				Action: KubeWorkRemovePod,
				Pod:    createMockPod("RunBookKeeper", "192.168.1.4"),
			},
			MSBWorkRemovePod,
			"192.168.1.3",
		},
	}

	for _, c := range podCases {
		kubeWorkQueue <- c.work

		if c.work.Action == KubeWorkUpdatePod {
			msbWorkPodValidate(t, msbWorkQueue, c.work.Pod, MSBWorkRemovePod, "192.168.1.2")
			msbWorkPodValidate(t, msbWorkQueue, c.work.Pod, MSBWorkAddPod, "192.168.1.3")
		} else {
			msbWorkPodValidate(t, msbWorkQueue, c.work.Pod, c.msbAction, c.ip)
		}
	}
}
