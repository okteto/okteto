// Copyright 2020 The Okteto Authors
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
package cmd

import "testing"

func Test_parseURL(t *testing.T) {
	tests := []struct {
		name    string
		u       string
		want    string
		wantErr bool
	}{
		{
			name: "full",
			u:    "https://okteto.cloud.okteto.net",
			want: "https://okteto.cloud.okteto.net",
		},
		{
			name: "no-protocol",
			u:    "okteto.cloud.okteto.net",
			want: "https://okteto.cloud.okteto.net",
		},
		{
			name: "trim-slash",
			u:    "https://okteto.cloud.okteto.net/",
			want: "https://okteto.cloud.okteto.net",
		},
		{
			name: "ip",
			u:    "https://192.168.0.1",
			want: "https://192.168.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseURL(tt.u)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
