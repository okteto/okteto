// Copyright 2026 The Okteto Authors
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

package divert

import (
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/divert/gatewayapi"
	"github.com/okteto/okteto/pkg/divert/istio"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func setOktetoContext(t *testing.T, isOkteto bool) {
	t.Helper()
	previous := okteto.CurrentStore
	t.Cleanup(func() { okteto.CurrentStore = previous })
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  isOkteto,
				Cfg: &clientcmdapi.Config{
					Clusters:       map[string]*clientcmdapi.Cluster{"test": {Server: "https://localhost:6443"}},
					AuthInfos:      map[string]*clientcmdapi.AuthInfo{"test": {}},
					Contexts:       map[string]*clientcmdapi.Context{"test": {Cluster: "test", AuthInfo: "test"}},
					CurrentContext: "test",
				},
			},
		},
		CurrentContext: "test",
	}
}

func TestNew_NotOktetoContext(t *testing.T) {
	setOktetoContext(t, false)

	d, err := New(&model.DivertDeploy{Driver: constants.OktetoDivertGatewayAPIDriver}, "app", "test", fake.NewSimpleClientset(), io.NewIOController())

	require.ErrorIs(t, err, oktetoErrors.ErrDivertNotSupported)
	require.Nil(t, d)
}

func TestNew_GatewayAPIDriver(t *testing.T) {
	setOktetoContext(t, true)

	d, err := New(&model.DivertDeploy{Driver: constants.OktetoDivertGatewayAPIDriver, Namespace: "staging"}, "app", "test", fake.NewSimpleClientset(), io.NewIOController())

	require.NoError(t, err)
	require.IsType(t, &gatewayapi.Driver{}, d)
}

func TestNew_IstioDriver(t *testing.T) {
	setOktetoContext(t, true)

	d, err := New(&model.DivertDeploy{Driver: constants.OktetoDivertIstioDriver}, "app", "test", fake.NewSimpleClientset(), io.NewIOController())

	require.NoError(t, err)
	require.IsType(t, &istio.Driver{}, d)
}

// Older CLIs send the legacy nginx driver to the remote runner, so it must keep working there
func TestNew_LegacyNginxDriver(t *testing.T) {
	setOktetoContext(t, true)

	d, err := New(&model.DivertDeploy{Driver: "nginx", Namespace: "staging"}, "app", "test", fake.NewSimpleClientset(), io.NewIOController())

	require.NoError(t, err)
	require.IsType(t, &gatewayapi.Driver{}, d)
}

func TestNew_UnsupportedDriver(t *testing.T) {
	tests := []struct {
		name        string
		driver      string
		expectedErr string
	}{
		{
			name:        "empty driver",
			driver:      "",
			expectedErr: "the divert driver '' isn't supported",
		},
		{
			name:        "unknown driver",
			driver:      "invalid",
			expectedErr: "the divert driver 'invalid' isn't supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setOktetoContext(t, true)

			d, err := New(&model.DivertDeploy{Driver: tt.driver, Namespace: "staging"}, "app", "test", fake.NewSimpleClientset(), io.NewIOController())

			require.EqualError(t, err, tt.expectedErr)
			require.Nil(t, d)
		})
	}
}
