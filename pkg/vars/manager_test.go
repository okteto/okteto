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
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type VarManagerLogger struct{}

func (VarManagerLogger) Yellow(_ string, _ ...interface{}) {}
func (VarManagerLogger) AddMaskedWord(_ string)            {}

// TestVarManagerDoesNotExportToOsEnv ensures that using var manager does not have any undesired side effects on
// the host environment variables
func TestVarManagerDoesNotExportToOsEnv(t *testing.T) {
	t.Setenv("MY_VAR", "host-value")

	varManager := NewVarsManager(&VarManagerLogger{})
	varManager.AddLocalVar("MY_VAR", "local-value")

	assert.Equal(t, "host-value", os.Getenv("MY_VAR"))
	assert.Equal(t, "local-value", varManager.Get("MY_VAR"))
}

// runCommandsInRandomOrder helps ensure that the order in which the commands are run does not affect the result
func runCommandsInRandomOrder(commands []func()) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(commands), func(i, j int) { commands[i], commands[j] = commands[j], commands[i] })
	for _, command := range commands {
		command()
	}
}

// TestBuiltInVarsPriority ensures that built-in vars have the highest priority
func TestBuiltInVarsPriority(t *testing.T) {
	varManager := NewVarsManager(&VarManagerLogger{})

	varName := "MY_VAR"

	runCommandsInRandomOrder([]func(){
		func() { varManager.AddBuiltInVar(varName, "built-in-value") },
		func() { varManager.AddFlagVar(varName, "flag-value") },
		func() { varManager.AddLocalVar(varName, "local-value") },
		func() { varManager.AddDotEnvVar(varName, "dot-env-value") },
		func() { varManager.AddAdminAndUserVar(varName, "admin-and-user-value") },
	})

	result := varManager.Get(varName)
	assert.Equal(t, "built-in-value", result)
}

// TestFlagsVarsPriority ensures that flag vars have the highest priority after built-in vars
func TestFlagsVarsPriority(t *testing.T) {
	varManager := NewVarsManager(&VarManagerLogger{})

	varName := "MY_VAR"

	runCommandsInRandomOrder([]func(){
		func() { varManager.AddFlagVar(varName, "flag-value") },
		func() { varManager.AddLocalVar(varName, "local-value") },
		func() { varManager.AddDotEnvVar(varName, "dot-env-value") },
		func() { varManager.AddAdminAndUserVar(varName, "admin-and-user-value") },
	})

	result := varManager.Get(varName)
	assert.Equal(t, "flag-value", result)
}

// TestLocalVarsPriority ensures that local vars have the highest priority after flag vars
func TestLocalVarsPriority(t *testing.T) {
	varManager := NewVarsManager(&VarManagerLogger{})

	varName := "MY_VAR"

	runCommandsInRandomOrder([]func(){
		func() { varManager.AddLocalVar(varName, "local-value") },
		func() { varManager.AddDotEnvVar(varName, "dot-env-value") },
		func() { varManager.AddAdminAndUserVar(varName, "admin-and-user-value") },
	})

	result := varManager.Get(varName)
	assert.Equal(t, "local-value", result)
}

// TestDotEnvVarsPriority ensures that dotenv vars have the highest priority after local vars
func TestDotEnvVarsPriority(t *testing.T) {
	varManager := NewVarsManager(&VarManagerLogger{})

	varName := "MY_VAR"

	runCommandsInRandomOrder([]func(){
		func() { varManager.AddDotEnvVar(varName, "dot-env-value") },
		func() { varManager.AddAdminAndUserVar(varName, "admin-and-user-value") },
	})

	result := varManager.Get(varName)
	assert.Equal(t, "dot-env-value", result)
}

// TestPriorityWithMoreComplexScenarios ensures the priority is respected even with more complex scenarios
func TestPriorityWithMoreComplexScenarios(t *testing.T) {
	varManager := NewVarsManager(&VarManagerLogger{})
	varName := "MY_VAR"

	adminVars := Group{
		Vars: []Var{
			{Name: varName, Value: "admin-value"},
		},
		Type: OktetoVariableTypeAdminAndUser,
	}
	varManager.AddGroup(adminVars)
	assert.Equal(t, "admin-value", varManager.Get(varName))

	varManager.AddDotEnvVar(varName, "dot-env-value")
	assert.Equal(t, "dot-env-value", varManager.Get(varName))

	varManager.AddLocalVar(varName, "local-value")
	assert.Equal(t, "local-value", varManager.Get(varName))

	varManager.AddFlagVar(varName, "flag-value")
	assert.Equal(t, "flag-value", varManager.Get(varName))

	varManager.AddBuiltInVar(varName, "built-in-value")
	assert.Equal(t, "built-in-value", varManager.Get(varName))
}

func TestGet(t *testing.T) {
	tests := []struct {
		name          string
		getVarManager func() *Manager
		find          string
		expected      string
	}{
		{
			name: "empty var manager - var not found",
			getVarManager: func() *Manager {
				varManager := NewVarsManager(&VarManagerLogger{})
				return varManager
			},
			find:     "MY_VAR",
			expected: "",
		},
		{
			name: "local var loaded in var manager - var found",
			getVarManager: func() *Manager {
				varManager := NewVarsManager(&VarManagerLogger{})
				varManager.AddLocalVar("MY_VAR", "my-value")
				return varManager
			},
			find:     "MY_VAR",
			expected: "my-value",
		},
		{
			name: "flag var loaded in var manager - var found",
			getVarManager: func() *Manager {
				varManager := NewVarsManager(&VarManagerLogger{})
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
			got := varManager.Get(tt.find)
			assert.Equal(t, tt.expected, got)
		})

	}
}

func TestExpand(t *testing.T) {
	tests := []struct {
		name        string
		result      string
		value       string
		expectedErr bool
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
			varManager := NewVarsManager(&VarManagerLogger{})
			varManager.AddLocalVar("BAR", "bar")
			result, err := varManager.Expand(tt.value)
			assert.Equal(t, tt.result, result)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExpandIfNotEmpty(t *testing.T) {
	tests := []struct {
		name        string
		result      string
		value       string
		expectedErr bool
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
			name:        "local var is expanded",
			value:       "value-${LOCAL}-value",
			result:      "value-bar-value",
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
			varManager := NewVarsManager(&VarManagerLogger{})
			varManager.AddLocalVar("LOCAL", "bar")
			varManager.AddLocalVar("BAR", "bar")
			varManager.AddDotEnvVar("BAR", "bar")
			result, err := varManager.ExpandIfNotEmpty(tt.value)
			assert.Equal(t, tt.result, result)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestAddVarOverridesOldValue ensured that adding a new var with the same name, but different value overrides the old value
func TestAddVarOverridesOldValue(t *testing.T) {
	varManager := NewVarsManager(&VarManagerLogger{})
	varManager.AddLocalVar("MY_VAR", "old-value")
	assert.Equal(t, "old-value", varManager.Get("MY_VAR"))
	varManager.AddLocalVar("MY_VAR", "new-value")
	assert.Equal(t, "new-value", varManager.Get("MY_VAR"))
}

// TestGetAll ensures that the method returns all okteto variables excluding local variables, respecting the priority
func TestGetAll(t *testing.T) {
	t.Run("adding vars as groups", func(t *testing.T) {
		varManager := NewVarsManager(&VarManagerLogger{})

		// host environment variables should not affect the var manager unless they are loaded accordingly
		t.Setenv("MY_VAR", "host-local-value")
		assert.Equal(t, []string{}, varManager.GetAll())

		adminAndUserVars := Group{
			Vars: []Var{
				{Name: "MY_VAR", Value: "admin-value"},
			},
			Type: OktetoVariableTypeAdminAndUser,
		}
		varManager.AddGroup(adminAndUserVars)
		expected := []string{"MY_VAR=admin-value"}
		assert.Equal(t, expected, varManager.GetAll())

		dotEnvVars := Group{
			Vars: []Var{
				{Name: "MY_VAR", Value: "dot-env-value"},
			},
			Type: OktetoVariableTypeDotEnv,
		}
		varManager.AddGroup(dotEnvVars)
		expected = []string{"MY_VAR=dot-env-value"}
		assert.Equal(t, expected, varManager.GetAll())

		localVars := Group{
			Vars: []Var{
				{Name: "MY_VAR", Value: "local-value"},
			},
			Type: OktetoVariableTypeLocal,
		}
		varManager.AddGroup(localVars)
		expected = []string{"MY_VAR=local-value"}
		assert.Equal(t, expected, varManager.GetAll())

		flagVars := Group{
			Vars: []Var{
				{Name: "MY_VAR", Value: "flag-value"},
			},
			Type: OktetoVariableTypeFlag,
		}
		varManager.AddGroup(flagVars)
		expected = []string{"MY_VAR=flag-value"}
		assert.Equal(t, expected, varManager.GetAll())

		builtInVars := Group{
			Vars: []Var{
				{Name: "MY_VAR", Value: "built-in-value"},
			},
			Type: OktetoVariableTypeBuiltIn,
		}
		varManager.AddGroup(builtInVars)
		expected = []string{"MY_VAR=built-in-value"}
		assert.Equal(t, expected, varManager.GetAll())
	})

	t.Run("adding vars individually", func(t *testing.T) {
		varManager := NewVarsManager(&VarManagerLogger{})

		// host environment variables should not affect the var manager unless they are loaded accordingly
		t.Setenv("MY_VAR", "host-local-value")
		assert.Equal(t, []string{}, varManager.GetAll())

		varManager.AddAdminAndUserVar("MY_VAR", "admin-value")
		expected := []string{"MY_VAR=admin-value"}
		assert.Equal(t, expected, varManager.GetAll())

		varManager.AddDotEnvVar("MY_VAR", "dot-env-value")
		expected = []string{"MY_VAR=dot-env-value"}
		assert.Equal(t, expected, varManager.GetAll())

		varManager.AddLocalVar("MY_VAR", "local-value")
		expected = []string{"MY_VAR=local-value"}
		assert.Equal(t, expected, varManager.GetAll())

		varManager.AddFlagVar("MY_VAR", "flag-value")
		expected = []string{"MY_VAR=flag-value"}
		assert.Equal(t, expected, varManager.GetAll())

		varManager.AddBuiltInVar("MY_VAR", "built-in-value")
		expected = []string{"MY_VAR=built-in-value"}
		assert.Equal(t, expected, varManager.GetAll())
	})
}

func TestLoadBoolean(t *testing.T) {
	tests := []struct {
		name      string
		mockKey   string
		mockValue string
		key       string
		expected  bool
	}{
		{
			name:     "empty key",
			expected: false,
		},
		{
			name:     "empty value",
			mockKey:  "NON_EXISTING_VAR_UNIT_TEST",
			expected: false,
		},
		{
			name:      "false - string",
			mockKey:   "VAR_UNIT_TEST",
			mockValue: "random value",
			expected:  false,
		},
		{
			name:      "false - boolean",
			mockKey:   "VAR_UNIT_TEST",
			mockValue: "false",
			expected:  false,
		},
		{
			name:      "false - int",
			mockKey:   "VAR_UNIT_TEST",
			mockValue: "0",
			expected:  false,
		},
		{
			name:      "true - boolean",
			mockKey:   "VAR_UNIT_TEST",
			mockValue: "true",
			expected:  true,
		},
		{
			name:      "true - int",
			mockKey:   "VAR_UNIT_TEST",
			mockValue: "1",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varManager := NewVarsManager(&VarManagerLogger{})
			if tt.mockKey != "" {
				varManager.AddLocalVar(tt.mockKey, tt.mockValue)
			}
			got := varManager.LoadBoolean(tt.mockKey)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestLoadTimeOrDefault(t *testing.T) {
	tests := []struct {
		name           string
		mockKey        string
		mockValue      string
		defaultValue   time.Duration
		expectedResult time.Duration
	}{
		{
			name:           "empty key",
			defaultValue:   5 * time.Second,
			expectedResult: 5 * time.Second,
		},
		{
			name:           "empty value",
			mockKey:        "NON_EXISTING_VAR_UNIT_TEST",
			defaultValue:   10 * time.Second,
			expectedResult: 10 * time.Second,
		},
		{
			name:           "valid duration",
			mockKey:        "VAR_UNIT_TEST",
			mockValue:      "5s",
			defaultValue:   10 * time.Second,
			expectedResult: 5 * time.Second,
		},
		{
			name:           "invalid duration",
			mockKey:        "VAR_UNIT_TEST",
			mockValue:      "invalid",
			defaultValue:   10 * time.Second,
			expectedResult: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varManager := NewVarsManager(&VarManagerLogger{})
			if tt.mockKey != "" {
				varManager.AddLocalVar(tt.mockKey, tt.mockValue)
			}
			got := varManager.LoadTimeOrDefault(tt.mockKey, tt.defaultValue)
			assert.Equal(t, tt.expectedResult, got)
		})
	}
}

func TestLoadBooleanOrDefault(t *testing.T) {
	type tc struct {
		Name           string
		EnvKey         string
		EnvValue       string
		DefaultValue   bool
		ExpectedResult bool
	}

	testCases := []tc{
		{
			Name:           "Environment variable is 'true'",
			EnvKey:         "TEST_KEY",
			EnvValue:       "true",
			DefaultValue:   false,
			ExpectedResult: true,
		},
		{
			Name:           "Environment variable is 'false'",
			EnvKey:         "TEST_KEY",
			EnvValue:       "false",
			DefaultValue:   true,
			ExpectedResult: false,
		},
		{
			Name:           "Environment variable is not defined",
			EnvKey:         "TEST_KEY",
			EnvValue:       "",
			DefaultValue:   true,
			ExpectedResult: true,
		},
		{
			Name:           "Environment variable has an invalid value",
			EnvKey:         "TEST_KEY",
			EnvValue:       "invalid",
			DefaultValue:   false,
			ExpectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			varManager := NewVarsManager(&VarManagerLogger{})
			varManager.AddLocalVar(tc.EnvKey, tc.EnvValue)
			result := varManager.LoadBooleanOrDefault(tc.EnvKey, tc.DefaultValue)
			assert.Equal(t, tc.ExpectedResult, result)
		})
	}
}
