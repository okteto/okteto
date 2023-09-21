//go:build integration
// +build integration

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
package kubeconfig

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/stretchr/testify/require"
)

// Test_KubeconfigHasExec kubeconfig command should use exec instead of token for the user auth if feature flag enabled
func Test_KubeconfigHasExec(t *testing.T) {
	tests := []struct {
		name           string
		useStaticToken bool
	}{
		{
			name:           "enabling static token feature flag",
			useStaticToken: true,
		},
		{
			name:           "disabling static token feature flag",
			useStaticToken: false,
		},
	}

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	home := t.TempDir()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(context.OktetoUseStaticKubetokenEnvVar, strconv.FormatBool(tt.useStaticToken))

			err = commands.RunOktetoKubeconfig(oktetoPath, home)
			require.NoError(t, err)

			cfg := kubeconfig.Get([]string{filepath.Join(home, ".kube", "config")})
			require.Len(t, cfg.AuthInfos, 1)

			for _, v := range cfg.AuthInfos {
				require.NotNil(t, v)
				if tt.useStaticToken {
					require.NotEmpty(t, v.Token)
					require.Nil(t, v.Exec)
				} else {
					require.Empty(t, v.Token)
					require.NotNil(t, v.Exec)
				}
			}
		})
	}
}
