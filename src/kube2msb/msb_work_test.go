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
)

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
