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

package up

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd/api"
)

func Test_waitUntilExitOrInterrupt(t *testing.T) {
	up := upContext{
		Options:           &Options{},
		K8sClientProvider: test.NewFakeK8sProvider(),
	}
	up.CommandResult = make(chan error, 1)
	up.CommandResult <- nil
	ctx := context.Background()
	err := up.waitUntilExitOrInterruptOrApply(ctx)
	if err != nil {
		t.Errorf("exited with error instead of nil: %s", err)
	}

	up.CommandResult <- fmt.Errorf("custom-error")
	err = up.waitUntilExitOrInterruptOrApply(ctx)
	if err == nil {
		t.Errorf("didn't report proper error")
	}
	if _, ok := err.(oktetoErrors.CommandError); !ok {
		t.Errorf("didn't translate the error: %s", err)
	}

	up.Disconnect = make(chan error, 1)
	up.Disconnect <- oktetoErrors.ErrLostSyncthing
	err = up.waitUntilExitOrInterruptOrApply(ctx)
	if err != oktetoErrors.ErrLostSyncthing {
		t.Errorf("exited with error %s instead of %s", err, oktetoErrors.ErrLostSyncthing)
	}
}

func Test_printDisplayContext(t *testing.T) {
	var tests = []struct {
		up   *upContext
		name string
	}{
		{
			name: "basic",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{},
				},
			},
		},
		{
			name: "single-forward",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
					Forward:   []forward.Forward{{Local: 1000, Remote: 1000}},
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{},
				},
			},
		},
		{
			name: "multiple-forward",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
					Forward:   []forward.Forward{{Local: 1000, Remote: 1000}, {Local: 2000, Remote: 2000}},
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{
						{
							Local:  8080,
							Remote: 8080,
						},
						{
							Local:       8080,
							Remote:      8080,
							ServiceName: "api",
						},
					},
				},
			},
		},
		{
			name: "single-reverse",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
					Reverse:   []model.Reverse{{Local: 1000, Remote: 1000}},
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{},
				},
			},
		},
		{
			name: "multiple-reverse+global-forward",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
					Reverse:   []model.Reverse{{Local: 1000, Remote: 1000}, {Local: 2000, Remote: 2000}},
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{
						{
							Local:  8080,
							Remote: 8080,
						},
						{
							Local:       8080,
							Remote:      8080,
							ServiceName: "api",
						},
					},
				},
			},
		},
		{
			name: "global-forward",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{
						{
							Local:       8080,
							Remote:      8080,
							ServiceName: "api",
						},
					},
				},
			},
		},
		{
			name: "multiple-global-forward",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{
						{
							Local:       8080,
							Remote:      8080,
							ServiceName: "api",
						},
						{
							Local:       27017,
							Remote:      27017,
							ServiceName: "mongodb",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			printDisplayContext(tt.up)
		})
	}

}

func TestEnvVarIsAddedProperlyToDevContainerWhenIsSetFromCmd(t *testing.T) {
	var tests = []struct {
		dev                     *model.Dev
		upOptions               *Options
		name                    string
		expectedNumManifestEnvs int
	}{
		{
			name:                    "Add only env vars from cmd to dev container",
			dev:                     &model.Dev{},
			upOptions:               &Options{Envs: []string{"VAR1=value1", "VAR2=value2"}},
			expectedNumManifestEnvs: 2,
		},
		{
			name:                    "Add only env vars from cmd to dev container using envsubst format",
			dev:                     &model.Dev{},
			upOptions:               &Options{Envs: []string{"VAR1=value1", "VAR2=${var=$USER}"}},
			expectedNumManifestEnvs: 2,
		},
		{
			name:                    "Add only env vars from cmd to dev container using non ascii characters",
			dev:                     &model.Dev{},
			upOptions:               &Options{Envs: []string{"PASS=~$#@"}},
			expectedNumManifestEnvs: 1,
		},
		{
			name: "Add env vars from cmd and manifest to dev container",
			dev: &model.Dev{
				Environment: env.Environment{
					{
						Name:  "VAR_FROM_MANIFEST",
						Value: "value",
					},
				},
			},
			upOptions:               &Options{Envs: []string{"VAR1=value1", "VAR2=value2"}},
			expectedNumManifestEnvs: 3,
		},
		{
			name: "Overwrite env vars when is required",
			dev: &model.Dev{
				Environment: env.Environment{
					{
						Name:  "VAR_TO_OVERWRITE",
						Value: "oldValue",
					},
				},
			},
			upOptions:               &Options{Envs: []string{"VAR_TO_OVERWRITE=newValue", "VAR2=value2"}},
			expectedNumManifestEnvs: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overridedEnvVars, err := getOverridedEnvVarsFromCmd(tt.dev.Environment, tt.upOptions.Envs)
			if err != nil {
				t.Fatalf("unexpected error in setEnvVarsFromCmd: %s", err)
			}

			if tt.expectedNumManifestEnvs != len(*overridedEnvVars) {
				t.Fatalf("error in setEnvVarsFromCmd; expected num variables in container %d but got %d", tt.expectedNumManifestEnvs, len(tt.dev.Environment))
			}
		})
	}
}

func TestEnvVarIsNotAddedWhenHasBuiltInOktetoEnvVarsFormat(t *testing.T) {
	var tests = []struct {
		dev                     *model.Dev
		upOptions               *Options
		name                    string
		expectedNumManifestEnvs int
	}{
		{
			name:                    "Unable to set built-in okteto environment variable OKTETO_NAMESPACE",
			dev:                     &model.Dev{},
			upOptions:               &Options{Envs: []string{"OKTETO_NAMESPACE=value"}},
			expectedNumManifestEnvs: 2,
		},
		{
			name:                    "Unable to set built-in okteto environment variable OKTETO_GIT_BRANCH",
			dev:                     &model.Dev{},
			upOptions:               &Options{Envs: []string{"OKTETO_GIT_BRANCH=value"}},
			expectedNumManifestEnvs: 2,
		},
		{
			name:                    "Unable to set built-in okteto environment variable OKTETO_GIT_COMMIT",
			dev:                     &model.Dev{},
			upOptions:               &Options{Envs: []string{"OKTETO_GIT_COMMIT=value"}},
			expectedNumManifestEnvs: 2,
		},
		{
			name:                    "Unable to set built-in okteto environment variable OKTETO_REGISTRY_URL",
			dev:                     &model.Dev{},
			upOptions:               &Options{Envs: []string{"OKTETO_REGISTRY_URL=value"}},
			expectedNumManifestEnvs: 2,
		},
		{
			name:                    "Unable to set built-in okteto environment variable BUILDKIT_HOST",
			dev:                     &model.Dev{},
			upOptions:               &Options{Envs: []string{"BUILDKIT_HOST=value"}},
			expectedNumManifestEnvs: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getOverridedEnvVarsFromCmd(tt.dev.Environment, tt.upOptions.Envs)
			if !errors.Is(err, oktetoErrors.ErrBuiltInOktetoEnvVarSetFromCMD) {
				t.Fatalf("expected error in setEnvVarsFromCmd: %s due to try to set a built-in okteto environment variable", err)
			}
		})
	}
}

func TestCommandAddedToUpOptionsWhenPassedAsFlag(t *testing.T) {
	var tests = []struct {
		name            string
		command         []string
		expectedCommand []string
	}{
		{
			name:            "Passing no commands",
			command:         []string{""},
			expectedCommand: []string{},
		},
		{
			name:            "Passing commands",
			command:         []string{"echo", "hello"},
			expectedCommand: []string{"echo", "hello"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			cmd := Up(nil, io.NewIOController(), nil)
			for _, val := range tt.command {
				err := cmd.Flags().Set("command", val)
				if err != nil {
					t.Fatalf("unexpected error in Set: %s", err)
				}
			}

			flagValue, err := cmd.Flags().GetStringArray("command")
			if err != nil {
				t.Fatalf("unexpected error in GetStringArray: %s", err)
			}

			assert.Equal(t, tt.expectedCommand, flagValue)
		})
	}
}

func TestWakeNamespaceIfAppliesWithoutErrors(t *testing.T) {
	tests := []struct {
		name              string
		ns                v1.Namespace
		expectedWakeCalls int
	}{
		{
			name: "wake namespace if it is not sleeping",
			ns: v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Labels: map[string]string{
						constants.NamespaceStatusLabel: "Active",
					},
				},
			},
			expectedWakeCalls: 0,
		},
		{
			name: "wake namespace if it is sleeping",
			ns: v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Labels: map[string]string{
						constants.NamespaceStatusLabel: constants.NamespaceStatusSleeping,
					},
				},
			},
			expectedWakeCalls: 1,
		},
	}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := fake.NewSimpleClientset(&tt.ns)
			nsClient := client.NewFakeNamespaceClient([]types.Namespace{}, nil)
			oktetoClient := &client.FakeOktetoClient{
				Namespace: nsClient,
			}

			err := wakeNamespaceIfApplies(ctx, tt.ns.Name, k8sClient, oktetoClient)

			require.NoError(t, err)
			require.Equal(t, tt.expectedWakeCalls, nsClient.WakeCalls)
		})
	}
}

func TestSetSyncDefaultsByDevMode(t *testing.T) {
	fakeSyncFolderName := "test"
	tests := []struct {
		expectedError  error
		dev            *model.Dev
		expectedDev    *model.Dev
		name           string
		syncFolderName string
	}{
		{
			name: "hybrid mode not enabled: return nil",
			dev: &model.Dev{
				Mode: "sync",
			},
			expectedDev: &model.Dev{
				Mode: "sync",
			},
		},
		{
			name:           "hybrid mode enabled: return dev modified",
			syncFolderName: fakeSyncFolderName,
			dev: &model.Dev{
				Mode: "hybrid",
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
			},
			expectedDev: &model.Dev{
				Mode: "hybrid",
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							LocalPath:  fakeSyncFolderName,
							RemotePath: "/okteto",
						},
					},
				},
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getSyncTempDirFake := func() (string, error) {
				return tt.syncFolderName, tt.expectedError
			}
			err := setSyncDefaultsByDevMode(tt.dev, getSyncTempDirFake)
			require.NoError(t, err)
			require.Equal(t, tt.dev, tt.expectedDev)
		})
	}
}

func TestSetSyncDefaultsByDevModeError(t *testing.T) {
	dev := &model.Dev{
		Mode: "hybrid",
		PersistentVolumeInfo: &model.PersistentVolumeInfo{
			Enabled: true,
		},
	}

	expectedDev := *dev
	getSyncTempDirFake := func() (string, error) {
		return "", assert.AnError
	}
	err := setSyncDefaultsByDevMode(dev, getSyncTempDirFake)
	require.Error(t, err)
	require.Equal(t, *dev, expectedDev)
}

func TestUpdateKubetoken(t *testing.T) {
	tt := []struct {
		expected    error
		f           *client.FakeOktetoClient
		context     *okteto.ContextStore
		expectedCfg *api.Config
		name        string
	}{
		{
			name: "oktetoClientError",
			f: &client.FakeOktetoClient{
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
					Err: assert.AnError,
					Token: types.KubeTokenResponse{
						TokenRequest: authenticationv1.TokenRequest{
							Status: authenticationv1.TokenRequestStatus{
								Token: "token",
							},
						},
					},
				}),
			},
			context: &okteto.ContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.Context{
					"test": {
						UserID: "test",
						Cfg: &api.Config{
							AuthInfos: map[string]*api.AuthInfo{
								"test": {},
							},
						},
					},
				},
			},
			expectedCfg: &api.Config{
				AuthInfos: map[string]*api.AuthInfo{
					"test": {},
				},
			},
			expected: assert.AnError,
		},
		{
			name: "oktetoClientCorrect",
			f: &client.FakeOktetoClient{
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
					Token: types.KubeTokenResponse{
						TokenRequest: authenticationv1.TokenRequest{
							Status: authenticationv1.TokenRequestStatus{
								Token: "token",
							},
						},
					},
				}),
			},
			context: &okteto.ContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.Context{
					"test": {
						UserID: "test",
						Cfg: &api.Config{
							AuthInfos: map[string]*api.AuthInfo{
								"test": {},
							},
						},
					},
				},
			},
			expectedCfg: &api.Config{
				AuthInfos: map[string]*api.AuthInfo{
					"test": {
						Token: "token",
					},
				},
			},
			expected: nil,
		},
		{
			name: "cfg not configured",
			f: &client.FakeOktetoClient{
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
					Token: types.KubeTokenResponse{
						TokenRequest: authenticationv1.TokenRequest{
							Status: authenticationv1.TokenRequestStatus{
								Token: "token",
							},
						},
					},
				}),
			},
			context: &okteto.ContextStore{
				CurrentContext: "123",
				Contexts: map[string]*okteto.Context{
					"test": {
						UserID: "test",
						Cfg: &api.Config{
							AuthInfos: map[string]*api.AuthInfo{
								"test": {},
							},
						},
					},
					"123": {},
				},
			},
			expectedCfg: &api.Config{
				AuthInfos: map[string]*api.AuthInfo{
					"test": {},
				},
			},
			expected: errConfigNotConfigured,
		},
		{
			name: "incorrect authInfo",
			f: &client.FakeOktetoClient{
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
					Token: types.KubeTokenResponse{
						TokenRequest: authenticationv1.TokenRequest{
							Status: authenticationv1.TokenRequestStatus{
								Token: "token",
							},
						},
					},
				}),
			},
			context: &okteto.ContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.Context{
					"test": {
						UserID: "test",
						Cfg: &api.Config{
							AuthInfos: map[string]*api.AuthInfo{
								"123": {},
							},
						},
					},
				},
			},
			expectedCfg: &api.Config{
				AuthInfos: map[string]*api.AuthInfo{
					"123": {},
				},
			},
			expected: fmt.Errorf("user %s not found in kubeconfig", "test"),
		},
	}
	for _, tt := range tt {
		t.Run(tt.name, func(t *testing.T) {
			tokenUpdater := newTokenUpdaterController()
			tokenUpdater.oktetoClientProvider = client.NewFakeOktetoClientProvider(tt.f)
			okteto.CurrentStore = tt.context

			err := tokenUpdater.UpdateKubeConfigToken()
			if err != nil {
				assert.Equal(t, tt.expected, err)
			}

			resultCFG := okteto.CurrentStore.Contexts["test"].Cfg
			assert.Equal(t, tt.expectedCfg, resultCFG)
		})
	}
}

type fakeBuilder struct {
	getServicesErr   error
	buildErr         error
	usedBuildOptions *types.BuildOptions
	services         []string
}

func (b *fakeBuilder) GetServicesToBuild(_ context.Context, _ *model.Manifest, _ []string) ([]string, error) {
	if b.getServicesErr != nil {
		return nil, b.getServicesErr
	}
	return b.services, nil
}

func (b *fakeBuilder) Build(_ context.Context, opts *types.BuildOptions) error {
	b.usedBuildOptions = opts
	if b.buildErr != nil {
		return b.buildErr
	}
	return nil
}

func (*fakeBuilder) GetBuildEnvVars() map[string]string {
	return nil
}

func Test_buildServicesAndSetBuildEnvs(t *testing.T) {
	tests := []struct {
		expectedErr       error
		m                 *model.Manifest
		builder           *fakeBuilder
		expectedBuildOpts *types.BuildOptions
		name              string
	}{
		{
			name: "builder GetServicesToBuild returns error",
			builder: &fakeBuilder{
				getServicesErr: assert.AnError,
			},
			expectedErr: assert.AnError,
		},
		{
			name: "builder GetServicesToBuild returns empty list",
			builder: &fakeBuilder{
				services: nil,
			},
		},
		{
			name: "builder GetServicesToBuild returns list, Build is called with the list and the input manifest",
			builder: &fakeBuilder{
				services: []string{"test", "okteto"},
				usedBuildOptions: &types.BuildOptions{
					CommandArgs: []string{"test", "okteto"},
					Manifest: &model.Manifest{
						Name: "test-okteto",
					},
				},
			},
			m: &model.Manifest{
				Name: "test-okteto",
			},
			expectedBuildOpts: &types.BuildOptions{
				CommandArgs: []string{"test", "okteto"},
				Manifest: &model.Manifest{
					Name: "test-okteto",
				},
			},
		},
		{
			name: "builder GetServicesToBuild returns list, Build returns error",
			builder: &fakeBuilder{
				services: []string{"test", "okteto"},
				usedBuildOptions: &types.BuildOptions{
					CommandArgs: []string{"test", "okteto"},
					Manifest: &model.Manifest{
						Name: "test-okteto",
					},
				},
				buildErr: assert.AnError,
			},
			m: &model.Manifest{
				Name: "test-okteto",
			},
			expectedBuildOpts: &types.BuildOptions{
				CommandArgs: []string{"test", "okteto"},
				Manifest: &model.Manifest{
					Name: "test-okteto",
				},
			},
			expectedErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got := buildServicesAndSetBuildEnvs(ctx, tt.m, tt.builder)

			require.Equal(t, tt.expectedErr, got)
			require.Equal(t, tt.expectedBuildOpts, tt.builder.usedBuildOptions)
		})
	}
}
