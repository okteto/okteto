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

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_translateConfigMap(t *testing.T) {
	var tests = []struct {
		name    string
		status  string
		appName string
		output  string
	}{
		{
			name:    "create errorStatus",
			status:  ErrorStatus,
			appName: "test",
			output:  "test",
		},
		{
			name:    "create progressing",
			status:  ProgressingStatus,
			appName: "test",
			output:  "test",
		},
		{
			name:    "create destroying",
			status:  DestroyingStatus,
			appName: "test",
			output:  "test",
		},
		{
			name:    "create deployed",
			status:  DeployedStatus,
			appName: "test",
			output:  "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &CfgData{
				Status: tt.status,
				Output: tt.output,
			}
			cfg := TranslateConfigMap(tt.appName, data)
			assert.Equal(t, tt.appName, cfg.Name)
			assert.Equal(t, cfg.Data[statusField], tt.status)
		})
	}
}
