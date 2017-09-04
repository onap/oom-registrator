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
package main

import (
	"log"
	"strings"
	"sync"
)

type MSBWorker interface {
	AddService(string, string)
	RemoveService(string)
	AddPod(string, string)
	RemovePod(string)
}

type MSBAgentWorker struct {
	sync.Mutex
	MSBWorker
	agent Client
}

func newMSBAgentWorker(client Client) *MSBAgentWorker {
	return &MSBAgentWorker{
		agent: client,
	}
}

func (client *MSBAgentWorker) AddService(ip, sInfo string) {
	client.Lock()
	defer client.Unlock()

	if ip == "" || sInfo == "" {
		log.Println("Service Info is not valid for AddService")
		return
	}

	client.agent.Register(mergeIP(ip, sInfo))
}

func (client *MSBAgentWorker) RemoveService(ip, sInfo string) {
	client.Lock()
	defer client.Unlock()

	if sInfo == "" {
		log.Println("Service Info is not valid for RemoveService")
		return
	}

	client.agent.DeRegister(mergeIP(ip, sInfo))
}

func (client *MSBAgentWorker) AddPod(ip, sInfo string) {
	client.Lock()
	defer client.Unlock()
	if ip == "" || sInfo == "" {
		log.Println("Service Info is not valid for AddPod")
		return
	}

	client.agent.Register(mergeIP(ip, sInfo))
}

func (client *MSBAgentWorker) RemovePod(ip, sInfo string) {
	client.Lock()
	defer client.Unlock()
	if sInfo == "" {
		log.Println("Service Info is not valid for RemovePod")
		return
	}

	client.agent.DeRegister(mergeIP(ip, sInfo))
}

func mergeIP(ip, sInfo string) string {
	insert := "{\"ip\":\"" + ip + "\","
	parts := strings.Split(sInfo, "{")
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += insert + parts[i]
	}
	return out
}
