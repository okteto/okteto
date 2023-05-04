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

package deploy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployNotRemovingEnvFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	_, err := fs.Create(".env")
	require.NoError(t, err)
	opts := &Options{
		Manifest: &model.Manifest{
			Deploy: &model.DeployInfo{},
		},
	}
	localDeployer := localDeployer{
		ConfigMapHandler: &fakeCmapHandler{},
		Fs:               fs,
	}
	localDeployer.runDeploySection(context.Background(), opts)
	_, err = fs.Stat(".env")
	require.NoError(t, err)

}

func TestAddOktetoEnvsValuesAsDesployVariables(t *testing.T) {

	fs := afero.NewOsFs()
	tempDir, err := afero.TempDir(fs, "", "")
	require.NoError(t, err)
	tempEnv, err := fs.Create(filepath.Join(tempDir, ".env"))
	require.NoError(t, err)
	var tests = []struct {
		name              string
		opts              *Options
		expectedErr       bool
		expectedVariables []string
		oktetoEnvContent  []byte
		currentAddedEnvs  map[string]string
	}{
		{
			name: "add new variables from okteto env",
			opts: &Options{},
			oktetoEnvContent: []byte(`ONEKEY=ONEVALUE
SECONGKEY=SECONDVALUE`),
			currentAddedEnvs: make(map[string]string, 0),
			expectedVariables: []string{
				"ONEKEY=ONEVALUE",
				"SECONGKEY=SECONDVALUE",
			},
		},
		{
			name: "variable from OKTETO_ENV already added",
			opts: &Options{
				Variables: []string{
					"ONEKEY=ONEVALUE",
				},
			},
			oktetoEnvContent: []byte(`ONEKEY=ONEVALUE`),
			currentAddedEnvs: map[string]string{
				"ONEKEY": "ONEVALUE",
			},
			expectedVariables: []string{
				"ONEKEY=ONEVALUE",
			},
		},
		{
			name:              "No vars to add",
			opts:              &Options{},
			oktetoEnvContent:  []byte(``),
			currentAddedEnvs:  make(map[string]string, 0),
			expectedVariables: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			afero.WriteFile(fs, tempEnv.Name(), tt.oktetoEnvContent, 0644)
			addOktetoEnvsValuesAsDesployVariables(tt.opts, tempEnv.Name(), tt.currentAddedEnvs)
			assert.ElementsMatch(t, tt.opts.Variables, tt.expectedVariables)
		})
	}
}

func TestRemoveDefaultVariables(t *testing.T) {
	var defaultVars = []string{
		"VARTOREMOVE",
		"VARTOREMOVE2",
		"VARTOREMOVE3",
	}
	var currentVars = []string{
		"VARTOREMOVE",
		"ANOTHERVAR",
	}
	var expectedVars = []string{
		"ANOTHERVAR",
	}
	result := removeDefaultVariables(currentVars, defaultVars)
	assert.ElementsMatch(t, expectedVars, result)
}
