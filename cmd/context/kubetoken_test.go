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
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd/api"
)

func Test_RemoveExecFromCfg(t *testing.T) {
	var tests = []struct {
		input    *okteto.Context
		expected *okteto.Context
		name     string
	}{
		{
			name:     "nil context",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty UserID",
			input:    &okteto.Context{},
			expected: &okteto.Context{},
		},
		{
			name:     "nil config",
			input:    &okteto.Context{UserID: "test-user"},
			expected: &okteto.Context{UserID: "test-user"},
		},
		{
			name:     "nil AuthInfos",
			input:    &okteto.Context{UserID: "test-user", Cfg: &api.Config{}},
			expected: &okteto.Context{UserID: "test-user", Cfg: &api.Config{}},
		},
		{
			name:     "missing user in AuthInfos",
			input:    &okteto.Context{UserID: "test-user", Cfg: &api.Config{AuthInfos: make(map[string]*api.AuthInfo)}},
			expected: &okteto.Context{UserID: "test-user", Cfg: &api.Config{AuthInfos: make(map[string]*api.AuthInfo)}},
		},
		{
			name: "Exec removed successfully",
			input: &okteto.Context{
				UserID: "test-user",
				Cfg: &api.Config{AuthInfos: map[string]*api.AuthInfo{
					"test-user": {Token: "test-token", Exec: &api.ExecConfig{Command: "test-cmd"}},
				}},
			},
			expected: &okteto.Context{
				UserID: "test-user",
				Cfg: &api.Config{AuthInfos: map[string]*api.AuthInfo{
					"test-user": {Token: "test-token", Exec: nil},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newStaticKubetokenController().updateOktetoContextExec(tt.input)

			assert.Equal(t, tt.expected, tt.input)
		})
	}
}
