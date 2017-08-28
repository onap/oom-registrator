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
	KubeWorkAddService    KubeWorkAction = "AddService"
	KubeWorkRemoveService KubeWorkAction = "RemoveService"
	KubeWorkUpdateService KubeWorkAction = "UpdateService"
	KubeWorkAddPod        KubeWorkAction = "AddPod"
	KubeWorkRemovePod     KubeWorkAction = "RemovePod"
	KubeWorkUpdatePod     KubeWorkAction = "UpdatePod"
)

type KubeWork struct {
	Action  KubeWorkAction
	Service *kapi.Service
	Pod     *kapi.Pod
}

type MSBWorkAction string

const (
	MSBWorkAddService    MSBWorkAction = "AddService"
	MSBWorkRemoveService MSBWorkAction = "RemoveService"
	MSBWorkAddPod        MSBWorkAction = "AddPod"
	MSBWorkRemovePod     MSBWorkAction = "RemovePod"
)

type MSBWork struct {
	Action      MSBWorkAction
	ServiceInfo string
	IPAddress   string
}

const serviceKey = "msb.onap.org/service-info"

type ServiceUnit struct {
	Name             string         `json:"serviceName,omitempty"`
	Version          string         `json:"version"`
	URL              string         `json:"url"`
	Protocol         string         `json:"protocol"`
	VisualRange      string         `json:"visualRange"`
	LBPolicy         string         `json:"lb_policy"`
	PublishPort      string         `json:"publish_port"`
	Namespace        string         `json:"namespace"`
	NWPlaneType      string         `json:"network_plane_type"`
	Host             string         `json:"host"`
	SubDomain        string         `json:"subdomain,omitempty"`
	Path             string         `json:"path"`
	Instances        []InstanceUnit `json:"nodes"`
	Metadata         []MetaUnit     `json:"metadata"`
	Labels           []string       `json:"labels"`
	SwaggerURL       string         `json:"swagger_url,omitempty"`
	IsManual         bool           `json:"is_manual"`
	EnableSSL        bool           `json:"enable_ssl"`
	EnableTLS        bool           `json:"enable_tls"`
	EnableReferMatch string         `json:"enable_refer_match"`
	ProxyRule        Rules          `json:"proxy_rule,omitempty"`
	RateLimiting     RateLimit      `json:"rate_limiting,omitempty"`
}

type InstanceUnit struct {
	ServiceIP      string `json:"ip,omitempty"`
	ServicePort    string `json:"port,omitempty"`
	LBServerParams string `json:"lb_server_params,omitempty"`
	CheckType      string `json:"checkType,omitempty"`
	CheckURL       string `json:"checkUrl,omitempty"`
	CheckInterval  string `json:"checkInterval,omitempty"`
	CheckTTL       string `json:"ttl,omitempty"`
	CheckTimeOut   string `json:"checkTimeOut,omitempty"`
	HaRole         string `json:"ha_role,omitempty"`
	ServiceID      string `json:"nodeId,omitempty"`
	ServiceStatus  string `json:"status,omitempty"`
	APPVersion     string `json:"appversion,omitempty"`
}

type MetaUnit struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Rules struct {
	HTTPProxy   HTTPProxyRule   `json:"http_proxy,omitempty"`
	StreamProxy StreamProxyRule `json:"stream_proxy,omitempty"`
}

type HTTPProxyRule struct {
	SendTimeout string `json:"send_timeout,omitempty"`
	ReadTimeout string `json:"read_timeout,omitempty"`
}

type StreamProxyRule struct {
	ProxyTimeout  string `json:"proxy_timeout,omitempty"`
	ProxyResponse string `json:"proxy_responses,omitempty"`
}

type RateLimit struct {
	LimitReq LimitRequest `json:"limit_req,omitempty"`
}

type LimitRequest struct {
	Rate  string `json:"rate,omitempty"`
	Burst string `json:"burst,omitempty"`
}

type ServiceAnnotation struct {
	IP          string `json:"ip,omitempty"`
	Port        string `json:"port,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
	Version     string `json:"version,omitempty"`
	URL         string `json:"url,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	LBPolicy    string `json:"lb_policy,omitempty"`
	VisualRange string `json:"visualRange,omitempty"`
}
