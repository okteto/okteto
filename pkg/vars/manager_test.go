// Copyright 2024 The Okteto Authors
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

package vars

import (
	"os"
	"testing"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/stretchr/testify/assert"
)

type fakeVarManager struct{}

func (*fakeVarManager) Set(key, value string) error {
	return os.Setenv(key, value)
}
func (*fakeVarManager) MaskVar(value string) {
	oktetoLog.AddMaskedWord(value)
}
func (*fakeVarManager) WarningLogf(format string, args ...interface{}) {
	oktetoLog.Warning(format, args...)
}

func TestVarManagerDoesNotExportToOsEnv(t *testing.T) {
	varManager := NewVarsManager(&fakeVarManager{})

	varManager.AddLocalVar("MY_VAR", "local-value")
	assert.Equal(t, "", os.Getenv("MY_VAR"))
	assert.Equal(t, "local-value", varManager.GetIncLocal("MY_VAR"))
}

func TestVarManagerRespectsVarTypePriority(t *testing.T) {
	varManager := NewVarsManager(&fakeVarManager{})

	varManager.AddBuiltInVar("MY_VAR", "built-in-value")
	assert.Equal(t, "built-in-value", varManager.GetIncLocal("MY_VAR"))

	varManager.AddLocalVar("MY_VAR", "local-value")
	assert.Equal(t, "local-value", varManager.GetIncLocal("MY_VAR"))

	varManager.AddFlagVar("MY_VAR", "flag-value")

	// local vars have higher priority of flag vars
	assert.Equal(t, "local-value", varManager.GetIncLocal("MY_VAR"))

	// but if we exclude local vars, flag vars are returned
	assert.Equal(t, "flag-value", varManager.GetExcLocal("MY_VAR"))
}

func TestGetIncLocal(t *testing.T) {
	tests := []struct {
		name          string
		getVarManager func() *Manager
		find          string
		expected      string
	}{
		{
			name: "empty var manager - var not found",
			getVarManager: func() *Manager {
				varManager := NewVarsManager(&fakeVarManager{})
				return varManager
			},
			find:     "MY_VAR",
			expected: "",
		},
		{
			name: "local var loaded in var manager - var found",
			getVarManager: func() *Manager {
				varManager := NewVarsManager(&fakeVarManager{})
				varManager.AddLocalVar("MY_VAR", "my-value")
				return varManager
			},
			find:     "MY_VAR",
			expected: "my-value",
		},
		{
			name: "flag var loaded in var manager - var found",
			getVarManager: func() *Manager {
				varManager := NewVarsManager(&fakeVarManager{})
				varManager.AddFlagVar("MY_VAR", "my-value")
				return varManager
			},
			find:     "MY_VAR",
			expected: "my-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varManager := tt.getVarManager()
			got := varManager.GetIncLocal(tt.find)
			assert.Equal(t, tt.expected, got)
		})

	}
}

func TestGetExcLocal(t *testing.T) {
	tests := []struct {
		name          string
		getVarManager func() *Manager
		find          string
		expected      string
	}{
		{
			name: "empty var manager - var not found",
			getVarManager: func() *Manager {
				varManager := NewVarsManager(&fakeVarManager{})
				return varManager
			},
			find:     "MY_VAR",
			expected: "",
		},
		{
			name: "local var loaded in var manager - var found",
			getVarManager: func() *Manager {
				varManager := NewVarsManager(&fakeVarManager{})
				varManager.AddLocalVar("MY_VAR", "my-value")
				return varManager
			},
			find:     "MY_VAR",
			expected: "",
		},
		{
			name: "flag var loaded in var manager - var found",
			getVarManager: func() *Manager {
				varManager := NewVarsManager(&fakeVarManager{})
				varManager.AddFlagVar("MY_VAR", "my-value")
				return varManager
			},
			find:     "MY_VAR",
			expected: "my-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varManager := tt.getVarManager()
			got := varManager.GetExcLocal(tt.find)
			assert.Equal(t, tt.expected, got)
		})

	}
}

func TestExpandIncLocal(t *testing.T) {
	tests := []struct {
		expectedErr bool
		name        string
		result      string
		value       string
	}{
		{
			name:        "broken var - missing closing curly bracket",
			value:       "value-${BAR",
			result:      "",
			expectedErr: true,
		},
		{
			name:        "no-var",
			value:       "value",
			result:      "value",
			expectedErr: false,
		},
		{
			name:        "var",
			value:       "value-${BAR}-value",
			result:      "value-bar-value",
			expectedErr: false,
		},
		{
			name:        "default",
			value:       "value-${FOO:-foo}-value",
			result:      "value-foo-value",
			expectedErr: false,
		},
		{
			name:        "only bar expanded",
			value:       "${BAR}",
			result:      "bar",
			expectedErr: false,
		},
		{
			name:        "only bar not expand if empty",
			value:       "${FOO}",
			result:      "",
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varManager := NewVarsManager(&fakeVarManager{})
			varManager.AddLocalVar("BAR", "bar")
			result, err := varManager.ExpandIncLocal(tt.value)
			assert.Equal(t, tt.result, result)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExpandExcLocal(t *testing.T) {
	tests := []struct {
		expectedErr bool
		name        string
		result      string
		value       string
	}{
		{
			name:        "broken var - missing closing curly bracket",
			value:       "value-${BAR",
			result:      "",
			expectedErr: true,
		},
		{
			name:        "no-var",
			value:       "value",
			result:      "value",
			expectedErr: false,
		},
		{
			name:        "local var is ignored",
			value:       "value-${LOCAL}-value",
			result:      "value--value",
			expectedErr: false,
		},
		{
			name:        "var",
			value:       "value-${BAR}-value",
			result:      "value-bar-value",
			expectedErr: false,
		},
		{
			name:        "default",
			value:       "value-${FOO:-foo}-value",
			result:      "value-foo-value",
			expectedErr: false,
		},
		{
			name:        "only bar expanded",
			value:       "${BAR}",
			result:      "bar",
			expectedErr: false,
		},
		{
			name:        "only bar not expand if empty",
			value:       "${FOO}",
			result:      "",
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varManager := NewVarsManager(&fakeVarManager{})
			varManager.AddLocalVar("LOCAL", "bar")
			varManager.AddLocalVar("BAR", "bar")
			varManager.AddDotEnvVar("BAR", "bar")
			result, err := varManager.ExpandExcLocal(tt.value)
			assert.Equal(t, tt.result, result)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExpandExcLocalIfNotEmpty(t *testing.T) {
	tests := []struct {
		expectedErr bool
		name        string
		result      string
		value       string
	}{
		{
			name:        "broken var - missing closing curly bracket",
			value:       "value-${BAR",
			result:      "",
			expectedErr: true,
		},
		{
			name:        "no-var",
			value:       "value",
			result:      "value",
			expectedErr: false,
		},
		{
			name:        "local var is ignored",
			value:       "value-${LOCAL}-value",
			result:      "value--value",
			expectedErr: false,
		},
		{
			name:        "var",
			value:       "value-${BAR}-value",
			result:      "value-bar-value",
			expectedErr: false,
		},
		{
			name:        "default",
			value:       "value-${FOO:-foo}-value",
			result:      "value-foo-value",
			expectedErr: false,
		},
		{
			name:        "only bar expanded",
			value:       "${BAR}",
			result:      "bar",
			expectedErr: false,
		},
		{
			name:        "only bar not expand if empty",
			value:       "${FOO}",
			result:      "${FOO}",
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varManager := NewVarsManager(&fakeVarManager{})
			varManager.AddLocalVar("LOCAL", "bar")
			varManager.AddLocalVar("BAR", "bar")
			varManager.AddDotEnvVar("BAR", "bar")
			result, err := varManager.ExpandExcLocalIfNotEmpty(tt.value)
			assert.Equal(t, tt.result, result)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
