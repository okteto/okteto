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

package context

import (
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/env"
	"github.com/stretchr/testify/assert"
)

func Test_setSecrets(t *testing.T) {
	key := "key"
	expectedValue := "value"
	var tests = []struct {
		envs    map[string]string
		name    string
		secrets []env.Var
	}{
		{
			name: "create new env var from secret",
			secrets: []env.Var{
				{
					Name:  key,
					Value: expectedValue,
				},
			},
			envs: map[string]string{},
		},
		{
			name: "not overwrite env var from secret",
			secrets: []env.Var{
				{
					Name:  key,
					Value: "random-value",
				},
			},
			envs: map[string]string{
				key: expectedValue,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}
			exportPlatformVariablesToEnv(tt.secrets)
			assert.Equal(t, expectedValue, os.Getenv(key))
		})
	}
}
