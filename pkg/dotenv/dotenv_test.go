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

package dotenv

import (
	"fmt"
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/vars"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

type fakeVarManager struct{}

func (*fakeVarManager) MaskVar(string)                     {}
func (*fakeVarManager) WarningLogf(string, ...interface{}) {}

func TestLoad(t *testing.T) {
	type expected struct {
		err                  error
		varManagerVars       map[string]string
		mustHaveOsEnvVars    map[string]string
		mustNotLoadVars      []string
		mustNotHaveOsEnvVars []string
	}

	tests := []struct {
		mockfs           func() afero.Fs
		mockEnv          func(*testing.T)
		updateVarManager func(*vars.Manager)
		dotEnvFilePath   string
		name             string
		expected         expected
	}{
		{
			name: "missing .env",
			mockfs: func() afero.Fs {
				return afero.NewMemMapFs()
			},
			dotEnvFilePath: "",
			expected: expected{
				varManagerVars: map[string]string{},
				err:            nil,
			},
		},
		{
			name: "empty .env",
			mockfs: func() afero.Fs {
				_ = afero.WriteFile(afero.NewMemMapFs(), DefaultDotEnvFile, []byte(""), 0644)
				return afero.NewMemMapFs()
			},
			dotEnvFilePath: DefaultDotEnvFile,
			expected: expected{
				varManagerVars: map[string]string{},
				err:            nil,
			},
		},
		{
			name: "syntax errors in .env",
			mockfs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, DefaultDotEnvFile, []byte("@"), 0644)
				return fs
			},
			dotEnvFilePath: DefaultDotEnvFile,
			expected: expected{
				varManagerVars: map[string]string{},
				err:            fmt.Errorf("error parsing dot env file: unexpected character \"@\" in variable name near \"@\""),
			},
		},
		{
			name: "valid variables are not loaded if syntax errors in .env",
			mockfs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, DefaultDotEnvFile, []byte(`
TEST=VALID-VALUE
@`), 0644)
				return fs
			},
			dotEnvFilePath: DefaultDotEnvFile,
			expected: expected{
				varManagerVars: map[string]string{},
				err:            fmt.Errorf("error parsing dot env file: unexpected character \"@\" in variable name near \"@\""),
			},
		},
		{
			name: "valid .env with a single var",
			mockfs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, DefaultDotEnvFile, []byte("TEST=VALUE"), 0644)
				return fs
			},
			dotEnvFilePath: DefaultDotEnvFile,
			expected: expected{
				varManagerVars: map[string]string{
					"TEST": "VALUE",
				},
				mustNotHaveOsEnvVars: []string{
					"TEST",
				},
				err: nil,
			},
		},
		{
			name: "valid .env with multiple vars",
			mockfs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, DefaultDotEnvFile, []byte(`
TEST=VALUE
TEST2=VALUE2`), 0644)
				return fs
			},
			dotEnvFilePath: DefaultDotEnvFile,
			expected: expected{
				varManagerVars: map[string]string{
					"TEST":  "VALUE",
					"TEST2": "VALUE2",
				},
				mustNotHaveOsEnvVars: []string{
					"TEST",
					"TEST2",
				},
				err: nil,
			},
		},
		{
			name: "valid .env with multiple vars expanded from local vars",
			mockEnv: func(test *testing.T) {
				test.Setenv("LOCAL_VAR", "my-local-value")
			},
			updateVarManager: func(varManager *vars.Manager) {
				varManager.AddLocalVar("LOCAL_VAR", "my-local-value")
			},
			mockfs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, DefaultDotEnvFile, []byte(`
VAR1=value1
VAR2=value2
VAR3=${VALUE3:-defaultValue3}
VAR4=${LOCAL_VAR:-defaultValue4}`), 0644)
				return fs
			},
			dotEnvFilePath: DefaultDotEnvFile,
			expected: expected{
				varManagerVars: map[string]string{
					"VAR1":      "value1",
					"VAR2":      "value2",
					"VAR3":      "defaultValue3",
					"VAR4":      "my-local-value",
					"LOCAL_VAR": "my-local-value",
				},
				mustHaveOsEnvVars: map[string]string{
					"LOCAL_VAR": "my-local-value",
				},
				mustNotHaveOsEnvVars: []string{
					"VAR1",
					"VAR2",
					"VAR3",
					"VAR4",
				},
				err: nil,
			},
		},
		{
			name: "local vars are not loaded unless used in the .env",
			mockEnv: func(test *testing.T) {
				test.Setenv("LOCAL_VAR", "my-local-value")
			},
			updateVarManager: func(varManager *vars.Manager) {
				varManager.AddFlagVar("FLAG_VAR", "my-flag-value")
			},
			mockfs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, DefaultDotEnvFile, []byte(`
VAR1=value1
VAR2=value2
VAR3=${VALUE3:-defaultValue3}
VAR4=${VALUE4:-defaultValue4}`), 0644)
				return fs
			},
			dotEnvFilePath: DefaultDotEnvFile,
			expected: expected{
				varManagerVars: map[string]string{
					"VAR1":     "value1",
					"VAR2":     "value2",
					"VAR3":     "defaultValue3",
					"VAR4":     "defaultValue4",
					"FLAG_VAR": "my-flag-value",
				},
				mustHaveOsEnvVars: map[string]string{
					"LOCAL_VAR": "my-local-value",
				},
				mustNotLoadVars: []string{
					"LOCAL_VAR",
				},
				err: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := tt.mockfs()
			varManager := vars.NewVarsManager(&fakeVarManager{})
			if tt.mockEnv != nil {
				tt.mockEnv(t)
			}
			if tt.updateVarManager != nil {
				tt.updateVarManager(varManager)
			}
			err := Load(tt.dotEnvFilePath, varManager, fs)
			if tt.expected.err != nil {
				assert.Equal(t, tt.expected.err.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			for k, v := range tt.expected.varManagerVars {
				actualVar, exists := varManager.LookupIncLocal(k)
				assert.Equal(t, v, actualVar)
				assert.Equal(t, true, exists)
			}
			for k, v := range tt.expected.mustHaveOsEnvVars {
				actualVar, exists := os.LookupEnv(k)
				assert.Equal(t, v, actualVar)
				assert.Equal(t, true, exists)
			}
			for _, k := range tt.expected.mustNotHaveOsEnvVars {
				_, exists := os.LookupEnv(k)
				if exists {
					log.Fatalf("os env var %s exists", k)
				}
				assert.Equal(t, false, exists)
			}
			for _, k := range tt.expected.mustNotLoadVars {
				_, exists := varManager.LookupIncLocal(k)
				if exists {
					log.Fatalf("var %s loaded in the varManager", k)
				}
				assert.Equal(t, false, exists)
			}
		})
	}
}
