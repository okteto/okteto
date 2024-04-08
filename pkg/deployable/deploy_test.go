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

package deployable

import (
	"context"
	"testing"
	"time"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/divert"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

type fakeCmapHandler struct {
	errUpdatingWithEnvs error
	errAddingPhase      error
}

func (f *fakeCmapHandler) UpdateEnvsFromCommands(context.Context, string, string, []string) error {
	return f.errUpdatingWithEnvs
}

func (f *fakeCmapHandler) AddPhaseDuration(context.Context, string, string, string, time.Duration) error {
	return f.errAddingPhase
}

type fakeKubeconfigHandler struct {
	mock.Mock
}

func (f *fakeKubeconfigHandler) Read() (*rest.Config, error) {
	args := f.Called()
	return args.Get(0).(*rest.Config), args.Error(1)

}
func (f *fakeKubeconfigHandler) Modify(port int, sessionToken, destKubeconfigFile string) error {
	args := f.Called(port, sessionToken, destKubeconfigFile)
	return args.Error(0)
}

type fakeProxy struct {
	mock.Mock
}

func (f *fakeProxy) Start() {
	f.Called()
}

func (f *fakeProxy) Shutdown(ctx context.Context) error {
	args := f.Called(ctx)
	return args.Error(0)
}

func (f *fakeProxy) GetPort() int {
	args := f.Called()
	return args.Int(0)
}

func (f *fakeProxy) GetToken() string {
	args := f.Called()
	return args.String(0)
}

func (f *fakeProxy) SetName(name string) {
	f.Called(name)
}

func (f *fakeProxy) SetDivert(driver divert.Driver) {
	f.Called(driver)
}

type fakeExecutor struct {
	mock.Mock
}

func (f *fakeExecutor) Execute(command model.DeployCommand, env []string) error {
	args := f.Called(command, env)
	return args.Error(0)
}

func (f *fakeExecutor) CleanUp(err error) {
	f.Called(err)
}

type fakeDivert struct {
	mock.Mock
}

func (f *fakeDivert) Deploy(ctx context.Context) error {
	args := f.Called(ctx)
	return args.Error(0)
}

type fakeExternalResource struct {
	mock.Mock
}

func (f *fakeExternalResource) Deploy(ctx context.Context, name, ns string, externalInfo *externalresource.ExternalResource) error {
	args := f.Called(ctx, name, ns, externalInfo)
	return args.Error(0)
}

func TestDeployNotRemovingEnvFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	_, err := fs.Create(".env")
	require.NoError(t, err)
	params := DeployParameters{}
	r := DeployRunner{
		ConfigMapHandler: &fakeCmapHandler{},
		Fs:               fs,
	}
	err = r.runCommandsSection(context.Background(), params)
	assert.NoError(t, err)
	_, err = fs.Stat(".env")
	require.NoError(t, err)

}

func TestRunDeployWithErrorGettingClient(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	k8sProvider := test.NewFakeK8sProvider()
	k8sProvider.ErrProvide = assert.AnError

	r := DeployRunner{
		K8sClientProvider: k8sProvider,
	}

	err := r.RunDeploy(context.Background(), DeployParameters{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestRunDeployWithErrorModifyingKubeconfig(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	k8sProvider := test.NewFakeK8sProvider()
	proxy := &fakeProxy{}
	kubeconfigHandler := &fakeKubeconfigHandler{}

	proxy.On("GetPort").Return(80)
	proxy.On("GetToken").Return("fake-token")

	kubeconfigHandler.On("Modify", 80, "fake-token", "temp-kubeconfig").Return(assert.AnError)

	r := DeployRunner{
		K8sClientProvider:  k8sProvider,
		Proxy:              proxy,
		Kubeconfig:         kubeconfigHandler,
		TempKubeconfigFile: "temp-kubeconfig",
	}

	err := r.RunDeploy(context.Background(), DeployParameters{})

	require.ErrorIs(t, err, assert.AnError)

	proxy.AssertExpectations(t)
	kubeconfigHandler.AssertExpectations(t)
}

func TestRunDeployWithEmptyDeployable(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	k8sProvider := test.NewFakeK8sProvider()
	proxy := &fakeProxy{}
	kubeconfigHandler := &fakeKubeconfigHandler{}

	proxy.On("GetPort").Return(80)
	proxy.On("GetToken").Return("fake-token")
	proxy.On("SetName", "test-a").Return().Once()
	proxy.On("Start").Return().Once()
	proxy.On("Shutdown", mock.Anything).Return(nil).Once()
	proxy.On("SetDivert", mock.Anything).Return().Once()

	kubeconfigHandler.On("Modify", 80, "fake-token", "temp-kubeconfig").Return(nil)

	r := DeployRunner{
		K8sClientProvider:  k8sProvider,
		Proxy:              proxy,
		Kubeconfig:         kubeconfigHandler,
		TempKubeconfigFile: "temp-kubeconfig",
		Fs:                 afero.NewMemMapFs(),
		ConfigMapHandler:   &fakeCmapHandler{},
	}

	params := DeployParameters{
		Name:      "test a",
		Namespace: "test",
		Variables: []string{
			"A=value1",
		},
		Deployable: Entity{
			Divert: &model.DivertDeploy{
				Driver: constants.OktetoDivertWeaverDriver,
			},
		},
	}
	err := r.RunDeploy(context.Background(), params)

	require.NoError(t, err)

	proxy.AssertExpectations(t)
	kubeconfigHandler.AssertExpectations(t)
}

func TestRunCommandsSectionWithCommands(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	executor := &fakeExecutor{}
	r := DeployRunner{
		TempKubeconfigFile: "temp-kubeconfig",
		Fs:                 afero.NewMemMapFs(),
		ConfigMapHandler:   &fakeCmapHandler{},
		Executor:           executor,
	}

	params := DeployParameters{
		Variables: []string{
			"A=value1",
			"B=value2",
		},
		Deployable: Entity{
			Commands: []model.DeployCommand{
				{
					Name:    "test command",
					Command: "echo",
				},
				{
					Name:    "test command 2",
					Command: "echo 2",
				},
			},
		},
	}

	expectedCommand1 := model.DeployCommand{
		Name:    "test command",
		Command: "echo",
	}
	expectedCommand2 := model.DeployCommand{
		Name:    "test command 2",
		Command: "echo 2",
	}
	expectedVariables := []string{
		"A=value1",
		"B=value2",
	}
	executor.On("Execute", expectedCommand1, expectedVariables).Return(nil).Once()
	executor.On("Execute", expectedCommand2, expectedVariables).Return(nil).Once()

	err := r.runCommandsSection(context.Background(), params)

	require.NoError(t, err)
	executor.AssertExpectations(t)
}

func TestRunCommandsSectionWithErrorInCommands(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	executor := &fakeExecutor{}
	r := DeployRunner{
		TempKubeconfigFile: "temp-kubeconfig",
		Fs:                 afero.NewMemMapFs(),
		ConfigMapHandler:   &fakeCmapHandler{},
		Executor:           executor,
	}

	params := DeployParameters{
		Variables: []string{
			"A=value1",
			"B=value2",
		},
		Deployable: Entity{
			Commands: []model.DeployCommand{
				{
					Name:    "test command",
					Command: "echo",
				},
				{
					Name:    "test command 2",
					Command: "echo 2",
				},
			},
		},
	}

	expectedCommand1 := model.DeployCommand{
		Name:    "test command",
		Command: "echo",
	}
	expectedCommand2 := model.DeployCommand{
		Name:    "test command 2",
		Command: "echo 2",
	}
	expectedVariables := []string{
		"A=value1",
		"B=value2",
	}
	executor.On("Execute", expectedCommand1, expectedVariables).Return(assert.AnError).Once()

	err := r.runCommandsSection(context.Background(), params)

	require.Error(t, err)
	executor.AssertNotCalled(t, "Execute", expectedCommand2, expectedVariables)
	executor.AssertExpectations(t)
}

func TestRunCommandsSectionWithDivert(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	executor := &fakeExecutor{}
	divertDeployer := &fakeDivert{}
	r := DeployRunner{
		TempKubeconfigFile: "temp-kubeconfig",
		Fs:                 afero.NewMemMapFs(),
		ConfigMapHandler:   &fakeCmapHandler{},
		Executor:           executor,
		DivertDeployer:     divertDeployer,
	}

	params := DeployParameters{
		Namespace: "test1",
		Variables: []string{
			"A=value1",
			"B=value2",
		},
		Deployable: Entity{
			Divert: &model.DivertDeploy{
				Driver:    constants.OktetoDivertWeaverDriver,
				Namespace: "test2",
			},
		},
	}

	divertDeployer.On("Deploy", mock.Anything).Return(nil).Once()

	err := r.runCommandsSection(context.Background(), params)

	require.NoError(t, err)
	executor.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
	executor.AssertExpectations(t)
	divertDeployer.AssertExpectations(t)
}

func TestRunCommandsSectionWithErrorDeployingDivert(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	executor := &fakeExecutor{}
	divertDeployer := &fakeDivert{}
	r := DeployRunner{
		TempKubeconfigFile: "temp-kubeconfig",
		Fs:                 afero.NewMemMapFs(),
		ConfigMapHandler:   &fakeCmapHandler{},
		Executor:           executor,
		DivertDeployer:     divertDeployer,
	}

	params := DeployParameters{
		Namespace: "test1",
		Variables: []string{
			"A=value1",
			"B=value2",
		},
		Deployable: Entity{
			Divert: &model.DivertDeploy{
				Driver:    constants.OktetoDivertWeaverDriver,
				Namespace: "test2",
			},
		},
	}

	divertDeployer.On("Deploy", mock.Anything).Return(assert.AnError).Once()

	err := r.runCommandsSection(context.Background(), params)

	require.Error(t, err)
	executor.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
	executor.AssertExpectations(t)
	divertDeployer.AssertExpectations(t)
}

func TestRunCommandsSectionWithExternal(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	k8sProvider := test.NewFakeK8sProvider()
	executor := &fakeExecutor{}
	divertDeployer := &fakeDivert{}
	externalResource := &fakeExternalResource{}
	r := DeployRunner{
		TempKubeconfigFile: "temp-kubeconfig",
		Fs:                 afero.NewMemMapFs(),
		ConfigMapHandler:   &fakeCmapHandler{},
		Executor:           executor,
		DivertDeployer:     divertDeployer,
		K8sClientProvider:  k8sProvider,
		GetExternalControl: func(_ *rest.Config) ExternalResourceInterface {
			return externalResource
		},
	}

	params := DeployParameters{
		Namespace: "test1",
		Deployable: Entity{
			External: externalresource.Section{
				"db": {
					Icon: "icon",
					Endpoints: []*externalresource.ExternalEndpoint{
						{
							Name: "name",
							Url:  "url",
						},
					},
				},
			},
		},
	}

	expectedExternalInfo := &externalresource.ExternalResource{
		Icon: "icon",
		Endpoints: []*externalresource.ExternalEndpoint{
			{
				Name: "name",
				Url:  "url",
			},
		},
	}
	externalResource.On("Deploy", mock.Anything, "db", "test1", expectedExternalInfo).Return(nil).Once()

	err := r.runCommandsSection(context.Background(), params)

	require.NoError(t, err)
	executor.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
	divertDeployer.AssertNotCalled(t, "Deploy", mock.Anything)
	executor.AssertExpectations(t)
	divertDeployer.AssertExpectations(t)
	externalResource.AssertExpectations(t)
}

func TestRunCommandsSectionWithErrorDeployingExternal(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	k8sProvider := test.NewFakeK8sProvider()
	executor := &fakeExecutor{}
	divertDeployer := &fakeDivert{}
	externalResource := &fakeExternalResource{}
	r := DeployRunner{
		TempKubeconfigFile: "temp-kubeconfig",
		Fs:                 afero.NewMemMapFs(),
		ConfigMapHandler:   &fakeCmapHandler{},
		Executor:           executor,
		DivertDeployer:     divertDeployer,
		K8sClientProvider:  k8sProvider,
		GetExternalControl: func(_ *rest.Config) ExternalResourceInterface {
			return externalResource
		},
	}

	params := DeployParameters{
		Namespace: "test1",
		Deployable: Entity{
			External: externalresource.Section{
				"db": {
					Icon: "icon",
					Endpoints: []*externalresource.ExternalEndpoint{
						{
							Name: "name",
							Url:  "url",
						},
					},
				},
			},
		},
	}

	expectedExternalInfo := &externalresource.ExternalResource{
		Icon: "icon",
		Endpoints: []*externalresource.ExternalEndpoint{
			{
				Name: "name",
				Url:  "url",
			},
		},
	}
	externalResource.On("Deploy", mock.Anything, "db", "test1", expectedExternalInfo).Return(assert.AnError).Once()

	err := r.runCommandsSection(context.Background(), params)

	require.Error(t, err)
	executor.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
	divertDeployer.AssertNotCalled(t, "Deploy", mock.Anything)
	executor.AssertExpectations(t)
	divertDeployer.AssertExpectations(t)
	externalResource.AssertExpectations(t)
}

func TestDeployExternalWithErrorGettingClient(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	k8sProvider := test.NewFakeK8sProvider()
	k8sProvider.ErrProvide = assert.AnError

	r := DeployRunner{
		K8sClientProvider: k8sProvider,
	}

	params := DeployParameters{
		Namespace: "test1",
		Deployable: Entity{
			External: externalresource.Section{
				"db": {
					Icon: "icon",
					Endpoints: []*externalresource.ExternalEndpoint{
						{
							Name: "name",
							Url:  "url",
						},
					},
				},
			},
		},
	}

	err := r.deployExternals(context.Background(), params, map[string]string{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestCleaUp(t *testing.T) {
	executor := &fakeExecutor{}
	proxy := &fakeProxy{}
	r := DeployRunner{
		Executor:           executor,
		Proxy:              proxy,
		TempKubeconfigFile: "temp-kubeconfig",
	}

	proxy.On("Shutdown", mock.Anything).Return(nil).Once()
	executor.On("CleanUp", assert.AnError).Return().Once()

	r.CleanUp(context.Background(), assert.AnError)

	proxy.AssertExpectations(t)
	executor.AssertExpectations(t)
}
