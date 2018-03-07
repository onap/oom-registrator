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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewMSBAgent(t *testing.T) {
	url := urlPrefix + "/health"
	handler := func(res http.ResponseWriter, req *http.Request) {
		if url != req.URL.String() {
			t.Errorf("newMSBAgent() health check url should be %s, not %s", url, req.URL)
		} else {
			res.WriteHeader(200)
			res.Header().Set("Content-Type", "application/xml")
			fmt.Fprintln(res, "regist success")
		}
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	_, err := newMSBAgent(server.URL)

	if err != nil {
		t.Errorf("newMSBAgent() error")
	}
}

func TestServiceAnnotation2ServiceUnit(t *testing.T) {
	// nil test
	unitNil := ServiceAnnotation2ServiceUnit(nil)
	if unitNil != nil {
		t.Errorf("ServiceAnnotation2ServiceUnit input nil, expect nil the result is not nil")
	}

	// not nil test
	sa := ServiceAnnotation{
		IP:          "127.0.0.1",
		Port:        "80",
		ServiceName: "saTest",
		Version:     "v1",
		URL:         "http://localhost:80/msb",
		Protocol:    "http",
		LBPolicy:    "random",
		VisualRange: "all",
		Path:        "/path",
		EnableSSL:   true,
	}
	unit := ServiceAnnotation2ServiceUnit(&sa)

	if unit.Name != sa.ServiceName || unit.Version != sa.Version || unit.URL != sa.URL || unit.Protocol != sa.Protocol || unit.LBPolicy != sa.LBPolicy || unit.LBPolicy != sa.LBPolicy || unit.Path != sa.Path || unit.EnableSSL != sa.EnableSSL || unit.Instances[0].ServiceIP != sa.IP || unit.Instances[0].ServicePort != sa.Port {
		t.Errorf("ServiceAnnotation2ServiceUnit error")
	}
}
