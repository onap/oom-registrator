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
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

const (
	urlPrefix = "/api/microservices/v1/services"
)

type Client interface {
	Register(string)
	DeRegister(string)
}

type MSBAgent struct {
	Client
	url string
}

func newMSBAgent(s string) (*MSBAgent, error) {
	healthCheckURL := s + urlPrefix + "/health"
	resp, err := http.Get(healthCheckURL)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MSB agent:%s is not available", s)
	}

	return &MSBAgent{url: s}, nil
}

func (client *MSBAgent) Register(serviceInfo string) {
	var (
		sa = &ServiceAnnotation{}
	)
	err := json.Unmarshal([]byte(serviceInfo), sa)
	if err != nil {
		log.Printf("Failed to Unmarshal serviceInfo to ServiceAnnotation:%v", err)
		return
	}

	su := ServiceAnnotation2ServiceUnit(sa)
	body, _ := json.Marshal(su)
	postURL := client.url + urlPrefix

	resp, err := http.Post(postURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("Failed to do a request:%v", err)
		return
	}

	log.Printf("Http request to register service:%s returned code:%d", su.Name, resp.StatusCode)
}

func (client *MSBAgent) DeRegister(serviceInfo string) {
	var (
		sa = &ServiceAnnotation{}
	)

	err := json.Unmarshal([]byte(serviceInfo), sa)
	if err != nil {
		log.Printf("Failed to Unmarshal serviceInfo to ServiceAnnotation:%v", err)
		return
	}

	deleteURL := client.url + urlPrefix + "/" + sa.ServiceName + "/version/" + sa.Version + "/nodes/" + sa.IP + "/" + sa.Port

	req, err := http.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		log.Printf("(deleteURL:%s) failed to NewRequest:%v", deleteURL, err)
		return
	}

	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		log.Printf("(deleteURL:%s) failed to do a request:%v", deleteURL, err)
		return
	}
	log.Printf("Http request to deregister service:%s returned code:%d", sa.ServiceName, resp.StatusCode)
}

func ServiceAnnotation2ServiceUnit(sa *ServiceAnnotation) *ServiceUnit {
	if sa == nil {
		return nil
	}

	return &ServiceUnit{
		Name:        sa.ServiceName,
		Version:     sa.Version,
		URL:         sa.URL,
		Protocol:    sa.Protocol,
		LBPolicy:    sa.LBPolicy,
		VisualRange: sa.VisualRange,
		Instances:   []InstanceUnit{{ServiceIP: sa.IP, ServicePort: sa.Port}},
	}
}
