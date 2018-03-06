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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func addServiceOrPodTest(t *testing.T, addType string) {
	handler := func(res http.ResponseWriter, req *http.Request) {
		if req.Method != "POST" {
			t.Errorf("Register() request method should be 'Post' not %s", req.Method)
		} else if urlPrefix != req.URL.String() {
			t.Errorf("Register() url should be %s, not %s", urlPrefix, req.URL)
		} else {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				t.Errorf("Register() fail to read request body")
			}
			var su = ServiceUnit{}
			parseError := json.Unmarshal([]byte(body), &su)
			if parseError != nil {
				t.Errorf("Register() request body can not parse to ServiceUnit, %s", body)
				return
			} else {
				res.WriteHeader(200)
				res.Header().Set("Content-Type", "application/xml")
				fmt.Fprintln(res, "regist success")
			}
		}

	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := MSBAgentWorker{
		agent: &MSBAgent{
			url: server.URL,
		},
	}

	serviceInfo := `[{
		"port":"8080",
		"serviceName":"resgisterTest",
		"version":"v1",
		"url":"/register/test",
		"protocol":"http",
		"lb_policy":"random",
		"visualRange":"1",
		"path":"rt",
		"enable_ssl":true
	}]`

	if addType == "Service" {
		client.AddService("192.168.1.10", serviceInfo)
	} else {
		client.AddPod("192.168.1.10", serviceInfo)
	}
}

func TestAddServiceMsb(t *testing.T) {
	addServiceOrPodTest(t, "Service")
}

func TestAddPodMsb(t *testing.T) {
	addServiceOrPodTest(t, "Pod")
}

func removeServiceOrPodTest(t *testing.T, removeType string) {
	cases := []struct{ url, ip, serviceInfo string }{
		{ // Version is ""
			urlPrefix + "/resgisterTest1/version/null/nodes/192.168.1.10/8080",
			"192.168.1.10",
			`[{
				"port":"8080",
				"serviceName":"resgisterTest1",
				"version":"",
				"url":"/register/test",
				"protocol":"http",
				"lb_policy":"random",
				"visualRange":"1",
				"path":"rt",
				"enable_ssl":true
			}]`,
		}, { // version is not ""
			urlPrefix + "/resgisterTest2/version/v1/nodes/192.168.1.10/8080",
			"192.168.1.10",
			`[{
				"port":"8080",
				"serviceName":"resgisterTest2",
				"version":"v1",
				"url":"/register/test",
				"protocol":"http",
				"lb_policy":"random",
				"visualRange":"1",
				"path":"rt",
				"enable_ssl":true
			}]`,
		},
	}

	for _, c := range cases {
		handler := func(res http.ResponseWriter, req *http.Request) {
			if req.Method != "DELETE" {
				t.Errorf("DeRegister() request method should be 'DELETE' not %s", req.Method)
			} else if c.url != req.URL.String() {
				t.Errorf("DeRegister() url should be %s, not %s", c.url, req.URL)
			} else {
				res.WriteHeader(200)
				res.Header().Set("Content-Type", "application/xml")
				fmt.Fprintln(res, "deregist success")
			}

		}
		server := httptest.NewServer(http.HandlerFunc(handler))
		defer server.Close()

		client := MSBAgentWorker{
			agent: &MSBAgent{
				url: server.URL,
			},
		}

		if removeType == "Service" {
			client.RemoveService(c.ip, c.serviceInfo)
		} else if removeType == "Pod" {
			client.RemovePod(c.ip, c.serviceInfo)
		}
	}

}

func TestRemoveServiceMsb(t *testing.T) {
	removeServiceOrPodTest(t, "Service")
}

func TestMergeIP(t *testing.T) {
	cases := []struct{ ip, sInfo, want string }{
		{"127.0.0.1", "{}", "{\"ip\":\"127.0.0.1\",}"},
		{"127.0.0.1", "[{}]", "[{\"ip\":\"127.0.0.1\",}]"},
		{"127.0.0.1", "{\"name\":\"msb\"}", "{\"ip\":\"127.0.0.1\",\"name\":\"msb\"}"},
		{"127.0.0.1", "{\"name\":\"msb\", \"child\":{\"name\":\"childname\"}}",
			"{\"ip\":\"127.0.0.1\",\"name\":\"msb\", \"child\":{\"ip\":\"127.0.0.1\",\"name\":\"childname\"}}"},
	}

	for _, c := range cases {
		got := mergeIP(c.ip, c.sInfo)
		if got != c.want {
			t.Errorf("mergeIP(%ip, %sInfo) == %got, want %want", c.ip, c.sInfo, got, c.want)
		}
	}
}
