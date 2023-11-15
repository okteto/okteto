// Copyright 2023 The Okteto Authors
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

package forward

import (
	"testing"
)

func TestGlobalForward_less(t *testing.T) {
	tests := []struct {
		f    *GlobalForward
		c    *GlobalForward
		name string
		want bool
	}{
		{
			name: "ports-lesser",
			f:    &GlobalForward{Local: 80},
			c:    &GlobalForward{Local: 85},
			want: true,
		},
		{
			name: "ports-bigger",
			f:    &GlobalForward{Local: 8080},
			c:    &GlobalForward{Local: 85},
			want: false,
		},
		{
			name: "services",
			f:    &GlobalForward{ServiceName: "db", Local: 80},
			c:    &GlobalForward{ServiceName: "api", Local: 81},
			want: true,
		},
		{
			name: "port-lesser-than-service",
			f:    &GlobalForward{Local: 22000},
			c:    &GlobalForward{ServiceName: "api", Local: 81},
			want: false,
		},
		{
			name: "service-lesser-than-port",
			f:    &GlobalForward{ServiceName: "api", Local: 81},
			c:    &GlobalForward{Local: 22000},
			want: true,
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
