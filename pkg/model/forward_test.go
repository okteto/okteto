// Copyright 2021 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"reflect"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v2"
)

func TestForward_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		data     Forward
	}{
		{
			name:     "basic",
			expected: "8080:9090",
			data:     Forward{Local: 8080, Remote: 9090},
		},
		{
			name:     "service-with-port",
			expected: "8080:svc:5214",
			data:     Forward{Local: 8080, Remote: 5214, Service: true, ServiceName: "svc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := yaml.Marshal(tt.data)
			if err != nil {
				t.Error(err)
			}

			outStr := strings.Trim(string(b), "\n")
			if outStr != tt.expected {
				t.Errorf("didn't marshal correctly. Actual '%+v', Expected '%+v'", outStr, tt.expected)
			}

		})
	}
}

func TestForward_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		expected  Forward
		expectErr bool
	}{
		{
			name:     "basic",
			data:     "8080:9090",
			expected: Forward{Local: 8080, Remote: 9090},
		},
		{
			name:     "equal",
			data:     "8080:8080",
			expected: Forward{Local: 8080, Remote: 8080},
		},
		{
			name:      "service-with-port",
			data:      "8080:svc:5214",
			expectErr: false,
			expected:  Forward{Local: 8080, Remote: 5214, Service: true, ServiceName: "svc"},
		},
		{
			name:      "bad-local-port",
			data:      "local:8080",
			expectErr: true,
		},
		{
			name:      "service-with-bad-port",
			data:      "8080:svc:bar",
			expectErr: true,
		},
		{
			name:      "too-little-parts",
			data:      "8080",
			expectErr: true,
		},
		{
			name:      "too-many-parts",
			data:      "8080:svc:8082:8019",
			expectErr: true,
		},
		{
			name:      "service-at-end",
			data:      "8080:8081:svc",
			expectErr: true,
		},
		{
			name:      "just-service",
			data:      "8080:svc",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Forward
			if err := yaml.Unmarshal([]byte(tt.data), &result); err != nil {
				if tt.expectErr {
					return
				}

				t.Fatal(err)
			}

			if tt.expectErr {
				t.Fatal("didn't got expected error")
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual '%+v', Expected '%+v'", result, tt.expected)
			}

			out, err := yaml.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}

			outStr := string(out)
			outStr = strings.TrimSuffix(outStr, "\n")

			if !reflect.DeepEqual(outStr, tt.data) {
				t.Errorf("didn't unmarshal correctly. Actual '%+v', Expected '%+v'", outStr, tt.data)
			}
		})
	}
}

func TestForward_less(t *testing.T) {
	tests := []struct {
		name string
		f    *Forward
		c    *Forward
		want bool
	}{
		{
			name: "ports-lesser",
			f:    &Forward{Local: 80},
			c:    &Forward{Local: 85},
			want: true,
		},
		{
			name: "ports-bigger",
			f:    &Forward{Local: 8080},
			c:    &Forward{Local: 85},
			want: false,
		},
		{
			name: "services",
			f:    &Forward{Service: true, ServiceName: "db", Local: 80},
			c:    &Forward{Service: true, ServiceName: "api", Local: 81},
			want: true,
		},
		{
			name: "port-lesser-than-service",
			f:    &Forward{Local: 22000},
			c:    &Forward{Service: true, ServiceName: "api", Local: 81},
			want: true,
		},
		{
			name: "service-lesser-than-port",
			f:    &Forward{Service: true, ServiceName: "api", Local: 81},
			c:    &Forward{Local: 22000},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.less(tt.c); got != tt.want {
				t.Errorf("Forward.less() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestForwardExtended_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		data     Forward
	}{
		{
			name:     "service-name",
			expected: "8080:svc:9090",
			data:     Forward{Local: 8080, Remote: 9090, Service: true, ServiceName: "svc"},
		},
		{
			name:     "service-name-and-labels",
			expected: "8080:svc:5214",
			data:     Forward{Local: 8080, Remote: 5214, Service: true, ServiceName: "svc", Labels: map[string]string{"key": "value"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := yaml.Marshal(tt.data)
			if err != nil {
				t.Error(err)
			}

			outStr := strings.Trim(string(b), "\n")
			if outStr != tt.expected {
				t.Errorf("didn't marshal correctly. Actual '%+v', Expected '%+v'", outStr, tt.expected)
			}

		})
	}
}
