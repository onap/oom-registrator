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
