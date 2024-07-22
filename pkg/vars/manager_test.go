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

//type fakeEnvManager struct {
//	t           *testing.T
//	maskedWords []string
//	logs        []string
//}

//func (*fakeEnvManager) LookupEnv(key string) (string, bool) {
//	return os.LookupEnv(key)
//}
//func (e *fakeEnvManager) SetEnv(key, value string) error {
//	e.t.Setenv(key, value)
//	return nil
//}
//func (e *fakeEnvManager) MaskVar(value string) {
//	e.maskedWords = append(e.maskedWords, value)
//}
//func (e *fakeEnvManager) WarningLogf(msg string, _ ...interface{}) {
//	e.logs = append(e.logs, msg)
//}
//func (*fakeEnvManager) WarnVarsPrecedence() {}
//
//func newFakeEnvManager(t *testing.T) *fakeEnvManager {
//	return &fakeEnvManager{
//		t: t,
//	}
//}

//func varExists(key string) bool {
//	_, exists := os.LookupEnv(key)
//	return exists
//}

//func Test_EnvManager(t *testing.T) {
//	fakeEnvManager := newFakeEnvManager(t)
//
//	t.Run("empty env manager", func(t *testing.T) {
//		envManager := NewVarsManager(fakeEnvManager)
//		err := envManager.export()
//		assert.NoError(t, err)
//
//		var emptyGroup []env.Var
//
//		envManager.AddVars(emptyGroup, OktetoVariableTypeLocal)
//		err = envManager.export()
//		assert.NoError(t, err)
//	})
//
//	t.Run("add multiple groups and lookup var with higher priority successfully", func(t *testing.T) {
//		fakeGroupVarsFromPlatform := []env.Var{
//			{
//				Name:  "TEST_VAR_1",
//				Value: "platform-value1",
//			},
//			{
//				Name:  "TEST_VAR_2",
//				Value: "platform-value2",
//			},
//		}
//
//		fakeGroupVarsFromManifest := []env.Var{
//			{
//				Name:  "TEST_VAR_1",
//				Value: "manifest-value1",
//			},
//		}
//
//		fakeGroupVarsFromLoal := []env.Var{
//			{
//				Name:  "TEST_VAR_1",
//				Value: "local-value1",
//			},
//		}
//
//		fakeGroupVarsFromFlag := []env.Var{
//			{
//				Name:  "TEST_VAR_1",
//				Value: "flag-value1",
//			},
//		}
//
//		// making sure these vars are not set
//		assert.Equal(t, false, varExists("TEST_VAR_1"))
//		assert.Equal(t, false, varExists("TEST_VAR_2"))
//		assert.Equal(t, false, varExists("TEST_VAR_3"))
//
//		envManager := NewVarsManager(fakeEnvManager)
//		envManager.AddVars(fakeGroupVarsFromPlatform, PriorityVarFromPlatform)
//		assert.NoError(t, envManager.export())
//		assert.Equal(t, "platform-value1", os.Getenv("TEST_VAR_1"))
//
//		envManager.AddVars(fakeGroupVarsFromManifest, PriorityVarFromManifest)
//
//		// until we export, the value stays the same
//		assert.Equal(t, "platform-value1", os.Getenv("TEST_VAR_1"))
//
//		assert.NoError(t, envManager.export())
//		assert.Equal(t, "manifest-value1", os.Getenv("TEST_VAR_1"))
//
//		envManager.AddVars(fakeGroupVarsFromLoal, OktetoVariableTypeLocal)
//		assert.NoError(t, envManager.export())
//		assert.Equal(t, "local-value1", os.Getenv("TEST_VAR_1"))
//
//		envManager.AddVars(fakeGroupVarsFromFlag, OktetoVariableTypeFlag)
//		assert.NoError(t, envManager.export())
//		assert.Equal(t, "flag-value1", os.Getenv("TEST_VAR_1"))
//
//		// no other groups override the value
//		assert.Equal(t, "platform-value2", os.Getenv("TEST_VAR_2"))
//
//		// make sure values are obfuscated
//		// note: currently local vars are not obsucated so "local-value1" should not be in the list
//		expectedMaskedValues := []string{"platform-value1", "platform-value2", "manifest-value1", "flag-value1"}
//		assert.ElementsMatch(t, expectedMaskedValues, fakeEnvManager.maskedWords, "Masked words should match expected values")
//	})
//}
//
//func Test_Expand(t *testing.T) {
//	fakeEnvManager := NewVarsManager(newFakeEnvManager(t)
//
//	groupLocalVars := Group{
//		Vars: []Var{
//			{
//				Name:  "TEST_VAR_1",
//				Value: "local-value1",
//			},
//		}
//	}
//
//	fakeEnvManager.A
//}
//
//func Test_ExpandEnv(t *testing.T) {
//	t.Setenv("BAR", "bar")
//	tests := []struct {
//		expectedErr error
//		name        string
//		result      string
//		value       string
//	}{
//		{
//			name:        "broken var - missing closing curly bracket",
//			value:       "value-${BAR",
//			result:      "",
//			expectedErr: &VarExpansionErr{},
//		},
//		{
//			name:        "no-var",
//			value:       "value",
//			result:      "value",
//			expectedErr: nil,
//		},
//		{
//			name:        "var",
//			value:       "value-${BAR}-value",
//			result:      "value-bar-value",
//			expectedErr: nil,
//		},
//		{
//			name:        "default",
//			value:       "value-${FOO:-foo}-value",
//			result:      "value-foo-value",
//			expectedErr: nil,
//		},
//		{
//			name:        "only bar expanded",
//			value:       "${BAR}",
//			result:      "bar",
//			expectedErr: nil,
//		},
//		{
//			name:        "only bar not expand if empty",
//			value:       "${FOO}",
//			result:      "",
//			expectedErr: nil,
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			result, err := ExpandEnv(tt.value)
//			assert.Equal(t, tt.result, result)
//			if tt.expectedErr != nil {
//				assert.ErrorAs(t, err, tt.expectedErr)
//			} else {
//				assert.NoError(t, err)
//			}
//		})
//	}
//}

//func Test_ExpandEnvIfNotEmpty(t *testing.T) {
//	t.Setenv("BAR", "bar")
//	tests := []struct {
//		expectedErr error
//		name        string
//		result      string
//		value       string
//	}{
//		{
//			name:        "broken var - missing closing curly bracket",
//			value:       "value-${BAR",
//			result:      "",
//			expectedErr: &VarExpansionErr{},
//		},
//		{
//			name:        "no-var",
//			value:       "value",
//			result:      "value",
//			expectedErr: nil,
//		},
//		{
//			name:        "var",
//			value:       "value-${BAR}-value",
//			result:      "value-bar-value",
//			expectedErr: nil,
//		},
//		{
//			name:        "default",
//			value:       "value-${FOO:-foo}-value",
//			result:      "value-foo-value",
//			expectedErr: nil,
//		},
//		{
//			name:        "only bar expanded",
//			value:       "${BAR}",
//			result:      "bar",
//			expectedErr: nil,
//		},
//		{
//			name:        "only bar not expand if empty",
//			value:       "${FOO}",
//			result:      "${FOO}",
//			expectedErr: nil,
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			result, err := ExpandEnvIfNotEmpty(tt.value)
//			assert.Equal(t, tt.result, result)
//			if tt.expectedErr != nil {
//				assert.ErrorAs(t, err, tt.expectedErr)
//			} else {
//				assert.NoError(t, err)
//			}
//		})
//	}
//}
