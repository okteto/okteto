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
	apiv1 "k8s.io/api/core/v1"
)

func Test_translateConfigMap(t *testing.T) {
	var tests = []struct {
		name     string
		status   string
		appName  string
		output   string
		expected *apiv1.ConfigMap
	}{
		{
			name:     "create errorStatus",
			status:   ErrorStatus,
			appName:  "test",
			output:   "test",
			expected: &apiv1.ConfigMap{},
		},
		{
			name:     "create progressing",
			status:   ProgressingStatus,
			appName:  "test",
			output:   "test",
			expected: &apiv1.ConfigMap{},
		},
		{
			name:     "create destroying",
			status:   DestroyingStatus,
			appName:  "test",
			output:   "test",
			expected: &apiv1.ConfigMap{},
		},
		{
			name:     "create deployed",
			status:   DeployedStatus,
			appName:  "test",
			output:   "test",
			expected: &apiv1.ConfigMap{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := TranslateConfigMap(tt.appName, tt.status, tt.output)
			assert.Equal(t, tt.appName, cfg.Name)
			assert.Equal(t, cfg.Data[statusField], tt.status)
			assert.Equal(t, cfg.Data[outputField], tt.output)
		})
	}
}
