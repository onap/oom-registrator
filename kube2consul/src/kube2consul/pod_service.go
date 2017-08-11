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
// pod_service.go
package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
	kapi "k8s.io/kubernetes/pkg/api"
)

type PodServiceAction interface {
	BuildServiceInfoMap(*kapi.ContainerPort, *kapi.Container)
	BuildAgentService(*ServiceInfo, DnsInfo, *kapi.ContainerPort, string) *consulapi.AgentServiceRegistration
}

type PodService struct {
	PodServiceAction
	sInfos map[string]*ServiceInfo
}

func newPodService() *PodService {
	return &PodService{
		sInfos: make(map[string]*ServiceInfo),
	}
}

func (pod *PodService) BuildServiceInfoMap(cport *kapi.ContainerPort, container *kapi.Container) {
	portstr := strconv.Itoa(int(cport.ContainerPort))
	fullname := "SERVICE_" + portstr + "_NAME"
	nameprefix := "SERVICE_" + portstr + "_NAME_"
	tagprefix := "SERVICE_" + portstr + "_TAGS"
	labelprefix := "SERVICE_" + portstr + "_ROUTE_LABELS"
	mdataprefix := "SERVICE_" + portstr + "_META_DATA"
	nsprefix := "SERVICE_" + portstr + "_NAMESPACE"
	ttlprefix := "SERVICE_" + portstr + "_CHECK_TTL"
	httpprefix := "SERVICE_" + portstr + "_CHECK_HTTP"
	tcpprefix := "SERVICE_" + portstr + "_CHECK_TCP"
	cmdprefix := "SERVICE_" + portstr + "_CHECK_CMD"
	scriptprefix := "SERVICE_" + portstr + "_CHECK_SCRIPT"
	timeoutprefix := "SERVICE_" + portstr + "_CHECK_TIMEOUT"
	intervalprefix := "SERVICE_" + portstr + "_CHECK_INTERVAL"
	for _, env := range container.Env {
		if env.Name == fullname || strings.HasPrefix(env.Name, nameprefix) {
			addToServiceInfoMap(env.Name, env.Value, Service_NAME, &pod.sInfos)
		} else if strings.HasPrefix(env.Name, tagprefix) {
			addToServiceInfoMap(env.Name, env.Value, Service_TAGS, &pod.sInfos)
		} else if strings.HasPrefix(env.Name, labelprefix) {
			addToServiceInfoMap(env.Name, env.Value, Service_LABELS, &pod.sInfos)
		} else if strings.HasPrefix(env.Name, mdataprefix) {
			addToServiceInfoMap(env.Name, env.Value, Service_MDATA, &pod.sInfos)
		} else if strings.HasPrefix(env.Name, nsprefix) {
			addToServiceInfoMap(env.Name, env.Value, Service_NS, &pod.sInfos)			
		} else if strings.HasPrefix(env.Name, ttlprefix) && env.Value != "" {
			addToServiceInfoMap(env.Name, env.Value, Service_TTL, &pod.sInfos)
		} else if strings.HasPrefix(env.Name, httpprefix) && env.Value != "" {
			addToServiceInfoMap(env.Name, env.Value, Service_HTTP, &pod.sInfos)
		} else if strings.HasPrefix(env.Name, tcpprefix) && env.Value != "" {
			addToServiceInfoMap(env.Name, env.Value, Service_TCP, &pod.sInfos)
		} else if strings.HasPrefix(env.Name, cmdprefix) && env.Value != "" {
			addToServiceInfoMap(env.Name, env.Value, Service_CMD, &pod.sInfos)
		} else if strings.HasPrefix(env.Name, scriptprefix) && env.Value != "" {
			addToServiceInfoMap(env.Name, env.Value, Service_SCRIPT, &pod.sInfos)
		} else if strings.HasPrefix(env.Name, timeoutprefix) && env.Value != "" {
			addToServiceInfoMap(env.Name, env.Value, Service_TIMEOUT, &pod.sInfos)
		} else if strings.HasPrefix(env.Name, intervalprefix) && env.Value != "" {
			addToServiceInfoMap(env.Name, env.Value, Service_INTERVAL, &pod.sInfos)
		}
	}
}

func (pod *PodService) BuildAgentService(sInfo *ServiceInfo, config DnsInfo, cport *kapi.ContainerPort, cId string) []*consulapi.AgentServiceRegistration {
	asrs := []*consulapi.AgentServiceRegistration{}
	checks := []*consulapi.AgentServiceCheck{}

	//build Checks object and populate checks table
	var checks_in_tag string

	if len(sInfo.TTL) > 0 {
		check := new(consulapi.AgentServiceCheck)
		check.TTL = sInfo.TTL
		kv := "ttl:" + sInfo.TTL + ","
		checks_in_tag += kv
		checks = append(checks, check)
	}

	if len(sInfo.HTTP) > 0 {
		check := new(consulapi.AgentServiceCheck)
		check.HTTP = fmt.Sprintf("http://%s:%d%s", config.IPAddress, cport.ContainerPort, sInfo.HTTP)
		kv := "http:" + sInfo.HTTP + ","
		checks_in_tag += kv
		if len(sInfo.Interval) == 0 {
			check.Interval = DefaultInterval
			if strings.Contains(checks_in_tag, "interval:") {
				//do nothing
			} else {
				kv = "interval:" + DefaultInterval + ","
				checks_in_tag += kv
			}
		} else {
			check.Interval = sInfo.Interval
			if strings.Contains(checks_in_tag, "interval:") {
				//do nothing
			} else {
				kv = "interval:" + sInfo.Interval + ","
				checks_in_tag += kv
			}
		}
		if len(sInfo.Timeout) > 0 {
			check.Timeout = sInfo.Timeout
			if strings.Contains(checks_in_tag, "timeout:") {
				//do nothing
			} else {
				kv = "timeout:" + sInfo.Timeout + ","
				checks_in_tag += kv
			}
		}
		checks = append(checks, check)
	}

	if len(sInfo.TCP) > 0 {
		check := new(consulapi.AgentServiceCheck)
		check.TCP = fmt.Sprintf("%s:%d", config.IPAddress, cport.ContainerPort)
		kv := "tcp:ok,"
		checks_in_tag += kv
		if len(sInfo.Interval) == 0 {
			check.Interval = DefaultInterval
			if strings.Contains(checks_in_tag, "interval:") {
				//do nothing
			} else {
				kv = "interval:" + DefaultInterval + ","
				checks_in_tag += kv
			}
		} else {
			check.Interval = sInfo.Interval
			if strings.Contains(checks_in_tag, "interval:") {
				//do nothing
			} else {
				kv = "interval:" + sInfo.Interval + ","
				checks_in_tag += kv
			}
		}
		if len(sInfo.Timeout) > 0 {
			check.Timeout = sInfo.Timeout
			if strings.Contains(checks_in_tag, "timeout:") {
				//do nothing
			} else {
				kv = "timeout:" + sInfo.Timeout + ","
				checks_in_tag += kv
			}
		}
		checks = append(checks, check)
	}

	if len(sInfo.CMD) > 0 {
		check := new(consulapi.AgentServiceCheck)
		check.Script = fmt.Sprintf("check-cmd %s %s %s", cId, strconv.Itoa(int(cport.ContainerPort)), sInfo.CMD)
		kv := "script:" + sInfo.CMD + ","
		checks_in_tag += kv
		if len(sInfo.Interval) == 0 {
			check.Interval = DefaultInterval
			if strings.Contains(checks_in_tag, "interval:") {
				//do nothing
			} else {
				kv = "interval:" + DefaultInterval + ","
				checks_in_tag += kv
			}
		} else {
			check.Interval = sInfo.Interval
			if strings.Contains(checks_in_tag, "interval:") {
				//do nothing
			} else {
				kv = "interval:" + sInfo.Interval + ","
				checks_in_tag += kv
			}
		}
		checks = append(checks, check)
	}

	if len(sInfo.Script) > 0 {
		check := new(consulapi.AgentServiceCheck)
		withIp := strings.Replace(sInfo.Script, "$SERVICE_IP", config.IPAddress, -1)
		check.Script = strings.Replace(withIp, "$SERVICE_PORT", strconv.Itoa(int(cport.ContainerPort)), -1)
		kv := "script:" + sInfo.Script + ","
		checks_in_tag += kv
		if len(sInfo.Interval) == 0 {
			check.Interval = DefaultInterval
			if strings.Contains(checks_in_tag, "interval:") {
				//do nothing
			} else {
				kv = "interval:" + DefaultInterval + ","
				checks_in_tag += kv
			}
		} else {
			check.Interval = sInfo.Interval
			if strings.Contains(checks_in_tag, "interval:") {
				//do nothing
			} else {
				kv = "interval:" + sInfo.Interval + ","
				checks_in_tag += kv
			}
		}
		if len(sInfo.Timeout) > 0 {
			check.Timeout = sInfo.Timeout
			if strings.Contains(checks_in_tag, "timeout:") {
				//do nothing
			} else {
				kv = "timeout:" + sInfo.Timeout + ","
				checks_in_tag += kv
			}
		}
		checks = append(checks, check)
	}

	//remove trailing ","
	if len(checks_in_tag) != 0 {
		checks_in_tag = strings.TrimRight(checks_in_tag, ",")
	}

	//build other talbes in tags
	var (
		base_in_tag     string
		lb_in_tag       string
		labels_in_tag   = sInfo.Labels
		metadata_in_tag = sInfo.MData
		ns_in_tag       = sInfo.NS
	)

	ts := strings.Split(sInfo.Tags, ",")
	for _, elem := range ts {
		if strings.Contains(elem, NETWORK_PLANE_TYPE) || strings.Contains(elem, VISUAL_RANGE) {
			kv := "," + elem
			labels_in_tag += kv
			continue
		}

		if strings.Contains(elem, LB_POLICY) || strings.Contains(elem, LB_SVR_PARAMS) {
			kv := "," + elem
			lb_in_tag += kv
			continue
		}

		kv := "," + elem
		base_in_tag += kv
	}

	//remove leading ","
	if len(base_in_tag) != 0 {
		base_in_tag = strings.TrimLeft(base_in_tag, ",")
	}

	if len(lb_in_tag) != 0 {
		lb_in_tag = strings.TrimLeft(lb_in_tag, ",")
	}

	if len(labels_in_tag) != 0 {
		labels_in_tag = strings.TrimLeft(labels_in_tag, ",")
	}

	//build tables in tag
	var tagslice []string
	if len(base_in_tag) != 0 {
		tagslice = append(tagslice, buildTableInJsonFormat(Table_BASE, base_in_tag))
	}

	if len(lb_in_tag) != 0 {
		tagslice = append(tagslice, buildTableInJsonFormat(Table_LB, lb_in_tag))
	}

	if len(labels_in_tag) == 0 {
		tagslice = append(tagslice, DefaultLabels)
	} else {
		if !strings.Contains(labels_in_tag, "visualRange") {
			labels_in_tag += ",visualRange:1"
		}	
		tagslice = append(tagslice, buildTableInJsonFormat(Table_LABELS, labels_in_tag))
	}

	if len(metadata_in_tag) != 0 {
		tagslice = append(tagslice, buildTableInJsonFormat(Table_META_DATA, metadata_in_tag))
	}

	if len(ns_in_tag) != 0 {
		tagslice = append(tagslice, "\"ns\":{\"namespace\":\"" + ns_in_tag + "\"}")
	}
	
	if len(checks_in_tag) != 0 {
		tagslice = append(tagslice, buildTableInJsonFormat(Table_CHECKS, checks_in_tag))
	}

	//append "-<namespace>"
	name := sInfo.Name
	if len(ns_in_tag) != 0 {
		name = name + "-" + ns_in_tag
	}	

	//handle IUI service
//	for _, elem := range ts {
//		var elemslice []string
//		elemslice = strings.Split(elem, ":")
//		if elemslice[0] == PROTOCOL {
//			switch elemslice[1] {
//			case PROTOCOL_UI:
//				name = PREFIX_UI + name
//			default:
//				log.Println("regular protocol:", elemslice[1])
//			}
//			break
//		}
//	}

	if len(sInfo.IPs) == 0 {
		serviceID := config.BaseID + "-" + strconv.Itoa(int(cport.ContainerPort)) + "-" + name
		asr := &consulapi.AgentServiceRegistration{
			ID:      serviceID,
			Name:    name,
			Address: config.IPAddress,
			Port:    int(cport.ContainerPort),
			Tags:    tagslice,
			Checks:  checks,
		}
		asrs = append(asrs, asr)
		return asrs
	} else {
		for _, ip := range sInfo.IPs {
			serviceID := config.BaseID + "-" + strconv.Itoa(int(cport.ContainerPort)) + "-" + name + "-" + ip.NetworkPlane
			addr := ip.IPAddress
			tagslice_with_single_network_plane_type := refillTagsliceWithSingleNetworkPlaneType(tagslice, labels_in_tag, ip.NetworkPlane)
			asr := &consulapi.AgentServiceRegistration{
				ID:      serviceID,
				Name:    name,
				Address: addr,
				Port:    int(cport.ContainerPort),
				Tags:    tagslice_with_single_network_plane_type,
				Checks:  checks,
			}
			asrs = append(asrs, asr)
		}
		return asrs
	}
}

func addToServiceInfoMap(name, value, flag string, smap *map[string]*ServiceInfo) {
	segs := strings.Split(name, "_")
	switch flag {
	case Service_NAME:
		if strings.ContainsAny(value, ".") {
			log.Printf("Warning:service name %s contains dot", value)
			value = strings.Replace(value, ".", "-", -1)
			log.Printf("Warning:service name has been modified to %s", value)
		}	
		if len(segs) == 3 {
			if _, ok := (*smap)["0"]; ok {
				(*smap)["0"].Name = value
			} else {
				sInfo := &ServiceInfo{
					Name: value,
				}
				(*smap)["0"] = sInfo
			}
		} else if len(segs) == 4 {
			id := segs[3]
			if _, ok := (*smap)[id]; ok {
				(*smap)[id].Name = value
			} else {
				sInfo := &ServiceInfo{
					Name: value,
				}
				(*smap)[id] = sInfo
			}
		}
	case Service_TAGS:
		if len(segs) == 3 {
			if _, ok := (*smap)["0"]; ok {
				(*smap)["0"].Tags = value
			} else {
				sInfo := &ServiceInfo{
					Tags: value,
				}
				(*smap)["0"] = sInfo
			}
		} else if len(segs) == 4 {
			id := segs[3]
			if _, ok := (*smap)[id]; ok {
				(*smap)[id].Tags = value
			} else {
				sInfo := &ServiceInfo{
					Tags: value,
				}
				(*smap)[id] = sInfo
			}
		}
	case Service_LABELS:
		if len(segs) == 4 {
			if _, ok := (*smap)["0"]; ok {
				(*smap)["0"].Labels = value
			} else {
				sInfo := &ServiceInfo{
					Labels: value,
				}
				(*smap)["0"] = sInfo
			}
		} else if len(segs) == 5 {
			id := segs[4]
			if _, ok := (*smap)[id]; ok {
				(*smap)[id].Labels = value
			} else {
				sInfo := &ServiceInfo{
					Labels: value,
				}
				(*smap)[id] = sInfo
			}
		}
	case Service_MDATA:
		if len(segs) == 4 {
			if _, ok := (*smap)["0"]; ok {
				(*smap)["0"].MData = value
			} else {
				sInfo := &ServiceInfo{
					MData: value,
				}
				(*smap)["0"] = sInfo
			}
		} else if len(segs) == 5 {
			id := segs[4]
			if _, ok := (*smap)[id]; ok {
				(*smap)[id].MData = value
			} else {
				sInfo := &ServiceInfo{
					MData: value,
				}
				(*smap)[id] = sInfo
			}
		}
	case Service_NS:
		if len(segs) == 3 {
			if _, ok := (*smap)["0"]; ok {
				(*smap)["0"].NS = value
			} else {
				sInfo := &ServiceInfo{
					NS: value,
				}
				(*smap)["0"] = sInfo
			}
		} else if len(segs) == 4 {
			id := segs[3]
			if _, ok := (*smap)[id]; ok {
				(*smap)[id].NS = value
			} else {
				sInfo := &ServiceInfo{
					NS: value,
				}
				(*smap)[id] = sInfo
			}
		}		
	case Service_TTL:
		if len(segs) == 4 {
			if _, ok := (*smap)["0"]; ok {
				(*smap)["0"].TTL = value
			} else {
				sInfo := &ServiceInfo{
					TTL: value,
				}
				(*smap)["0"] = sInfo
			}
		} else if len(segs) == 5 {
			id := segs[4]
			if _, ok := (*smap)[id]; ok {
				(*smap)[id].TTL = value
			} else {
				sInfo := &ServiceInfo{
					TTL: value,
				}
				(*smap)[id] = sInfo
			}
		}
	case Service_HTTP:
		if len(segs) == 4 {
			if _, ok := (*smap)["0"]; ok {
				(*smap)["0"].HTTP = value
			} else {
				sInfo := &ServiceInfo{
					HTTP: value,
				}
				(*smap)["0"] = sInfo
			}
		} else if len(segs) == 5 {
			id := segs[4]
			if _, ok := (*smap)[id]; ok {
				(*smap)[id].HTTP = value
			} else {
				sInfo := &ServiceInfo{
					HTTP: value,
				}
				(*smap)[id] = sInfo
			}
		}
	case Service_TCP:
		if len(segs) == 4 {
			if _, ok := (*smap)["0"]; ok {
				(*smap)["0"].TCP = value
			} else {
				sInfo := &ServiceInfo{
					TCP: value,
				}
				(*smap)["0"] = sInfo
			}
		} else if len(segs) == 5 {
			id := segs[4]
			if _, ok := (*smap)[id]; ok {
				(*smap)[id].TCP = value
			} else {
				sInfo := &ServiceInfo{
					TCP: value,
				}
				(*smap)[id] = sInfo
			}
		}
	case Service_CMD:
		if len(segs) == 4 {
			if _, ok := (*smap)["0"]; ok {
				(*smap)["0"].CMD = value
			} else {
				sInfo := &ServiceInfo{
					CMD: value,
				}
				(*smap)["0"] = sInfo
			}
		} else if len(segs) == 5 {
			id := segs[4]
			if _, ok := (*smap)[id]; ok {
				(*smap)[id].CMD = value
			} else {
				sInfo := &ServiceInfo{
					CMD: value,
				}
				(*smap)[id] = sInfo
			}
		}
	case Service_SCRIPT:
		if len(segs) == 4 {
			if _, ok := (*smap)["0"]; ok {
				(*smap)["0"].Script = value
			} else {
				sInfo := &ServiceInfo{
					Script: value,
				}
				(*smap)["0"] = sInfo
			}
		} else if len(segs) == 5 {
			id := segs[4]
			if _, ok := (*smap)[id]; ok {
				(*smap)[id].Script = value
			} else {
				sInfo := &ServiceInfo{
					Script: value,
				}
				(*smap)[id] = sInfo
			}
		}
	case Service_TIMEOUT:
		if len(segs) == 4 {
			if _, ok := (*smap)["0"]; ok {
				(*smap)["0"].Timeout = value
			} else {
				sInfo := &ServiceInfo{
					Timeout: value,
				}
				(*smap)["0"] = sInfo
			}
		} else if len(segs) == 5 {
			id := segs[4]
			if _, ok := (*smap)[id]; ok {
				(*smap)[id].Timeout = value
			} else {
				sInfo := &ServiceInfo{
					Timeout: value,
				}
				(*smap)[id] = sInfo
			}
		}
	case Service_INTERVAL:
		if len(segs) == 4 {
			if _, ok := (*smap)["0"]; ok {
				(*smap)["0"].Interval = value
			} else {
				sInfo := &ServiceInfo{
					Interval: value,
				}
				(*smap)["0"] = sInfo
			}
		} else if len(segs) == 5 {
			id := segs[4]
			if _, ok := (*smap)[id]; ok {
				(*smap)[id].Interval = value
			} else {
				sInfo := &ServiceInfo{
					Interval: value,
				}
				(*smap)[id] = sInfo
			}
		}
	default:
		log.Println("Unsupported Service Attribute: ", flag)
	}
}

func buildTableInJsonFormat(table, input string) string {
	head := "\"" + table + "\":{"
	tail := "}"
	body := ""

	s := strings.Split(input, ",")
	slen := len(s)
	for _, v := range s {
		slen--
		s1 := strings.Split(v, ":")
		mstr := "\"" + strings.TrimSpace(s1[0]) + "\":"
		if strings.Contains(body, mstr) {
			if slen == 0 {
				body = strings.TrimRight(body, ",")
			}
			continue
		}
		if len(s1) == 2 {
			kv := "\"" + strings.TrimSpace(s1[0]) + "\":\"" + strings.TrimSpace(s1[1]) + "\""
			if slen != 0 {
				kv += ","
			}
			body += kv
		}
	}

	return head + body + tail
}

func refillTagsliceWithSingleNetworkPlaneType(old []string, labels, nw_type string) []string {
	var ret_tagslice []string
	var new_labels string

	for _, elem := range old {
		if strings.Contains(elem, NETWORK_PLANE_TYPE) {
			continue
		}
		ret_tagslice = append(ret_tagslice, elem)
	}

	sl := strings.Split(labels, NETWORK_PLANE_TYPE)
	sl1 := strings.SplitN(sl[1], ",", 2)
	nw_type_field := NETWORK_PLANE_TYPE + ":" + nw_type

	if len(sl1) == 2 {
		new_labels = sl[0] + nw_type_field + "," + sl1[1]
	} else {
		new_labels = sl[0] + nw_type_field
	}
	ret_tagslice = append(ret_tagslice, buildTableInJsonFormat(Table_LABELS, new_labels))

	return ret_tagslice
}