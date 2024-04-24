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

package destroy

import (
	"context"
	"fmt"
	"os"
	"testing"

	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deps"
	"github.com/okteto/okteto/pkg/divert"
	okerrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd/api"
)

var fakeManifest = &model.Manifest{
	Name:      "test-app",
	Namespace: "namespace",
	Destroy: &model.DestroyInfo{
		Commands: []model.DeployCommand{
			{
				Name:    "printenv",
				Command: "printenv",
			},
			{
				Name:    "ls -la",
				Command: "ls -la",
			},
			{
				Name:    "cat /tmp/test.txt",
				Command: "cat /tmp/test.txt",
			},
		},
	},
}

var fakeManifestWithDivert = &model.Manifest{
	Deploy: &model.DeployInfo{
		Divert: &model.DivertDeploy{
			Namespace: "test",
			Driver:    "test-driver",
		},
	},
}

var fakeManifestWithDependencies = &model.Manifest{
	Name:      "test-app",
	Namespace: "namespace",
	Dependencies: map[string]*deps.Dependency{
		"dep1": {
			Namespace: "test-namespace",
		},
		"dep2": {
			Namespace: "test-namespace",
		},
		"dep3": {
			Namespace: "another-test-namespace",
		},
	},
}

type fakeDestroyer struct {
	err              error
	errOnVolumes     error
	destroyed        bool
	destroyedVolumes bool
}

type fakeSecretHandler struct {
	err     error
	secrets []v1.Secret
}

type fakeExecutor struct {
	err      error
	executed []model.DeployCommand
}

func (fd *fakeDestroyer) DestroyWithLabel(_ context.Context, _ string, _ namespaces.DeleteAllOptions) error {
	if fd.err != nil {
		return fd.err
	}

	fd.destroyed = true
	return nil
}

func (fd *fakeDestroyer) DestroySFSVolumes(_ context.Context, _ string, _ namespaces.DeleteAllOptions) error {
	if fd.errOnVolumes != nil {
		return fd.errOnVolumes
	}

	fd.destroyedVolumes = true
	return nil
}

func (fd *fakeSecretHandler) List(_ context.Context, _, _ string) ([]v1.Secret, error) {
	if fd.err != nil {
		return nil, fd.err
	}

	return fd.secrets, nil
}

func (fe *fakeExecutor) Execute(command model.DeployCommand, _ []string) error {
	fe.executed = append(fe.executed, command)
	if fe.err != nil {
		return fe.err
	}

	return nil
}

func (fe *fakeExecutor) CleanUp(_ error) {}

type fakeDivertDriver struct {
	mock.Mock
}

func (fd *fakeDivertDriver) Deploy(ctx context.Context) error {
	args := fd.Called(ctx)
	return args.Error(0)

}

func (fd *fakeDivertDriver) Destroy(ctx context.Context) error {
	args := fd.Called(ctx)
	return args.Error(0)

}

func (fd *fakeDivertDriver) UpdatePod(spec v1.PodSpec) v1.PodSpec {
	args := fd.Called(spec)
	return args.Get(0).(v1.PodSpec)

}

func (fd *fakeDivertDriver) UpdateVirtualService(vs *istioNetworkingV1beta1.VirtualService) {
	fd.Called(vs)
}

type fakePipelineDestroyer struct {
	mock.Mock
}

func (fpd *fakePipelineDestroyer) ExecuteDestroyPipeline(ctx context.Context, opts *pipelineCMD.DestroyOptions) error {
	args := fpd.Called(ctx, opts)
	return args.Error(0)
}

func TestMain(m *testing.M) {
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {
				Name:      "test",
				Namespace: "namespace",
				UserID:    "user-id",
			},
		},
	}
	os.Exit(m.Run())
}

func TestDestroyWithErrorGettingManifestButDestroySuccess(t *testing.T) {
	k8sClientProvider := test.NewFakeK8sProvider()
	fakeClient, _, err := k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	destroyer := &fakeDestroyer{}
	dc := &destroyCommand{
		getManifest: func(_ string, _ afero.Fs) (*model.Manifest, error) {
			return nil, assert.AnError
		},
		ConfigMapHandler: NewConfigmapHandler(fakeClient),
		nsDestroyer:      destroyer,
		secrets:          &fakeSecretHandler{},
		buildCtrl: buildCtrl{
			builder: fakeBuilderV2{
				getSvcs: fakeGetSvcs{},
				build:   nil,
			},
		},
	}

	err = dc.destroy(context.Background(), &Options{})

	require.NoError(t, err)
	require.True(t, destroyer.destroyed)
	require.True(t, destroyer.destroyedVolumes)

}

func TestDestroyWithErrorDestroyingDependencies(t *testing.T) {
	ctx := context.Background()
	k8sClientProvider := test.NewFakeK8sProvider()
	fakeClient, _, err := k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	destroyer := &fakeDestroyer{}
	dc := &destroyCommand{
		getManifest: func(_ string, _ afero.Fs) (*model.Manifest, error) {
			return fakeManifestWithDependencies, nil
		},
		ConfigMapHandler: NewConfigmapHandler(fakeClient),
		nsDestroyer:      destroyer,
		secrets:          &fakeSecretHandler{},
		getPipelineDestroyer: func() (pipelineDestroyer, error) {
			return nil, assert.AnError
		},
	}

	err = dc.destroy(ctx, &Options{
		DestroyDependencies: true,
		Name:                fakeManifestWithDependencies.Name,
		Namespace:           "namespace",
	})

	require.Error(t, err)
	require.False(t, destroyer.destroyed)
	require.False(t, destroyer.destroyedVolumes)

	cfg, err := fakeClient.CoreV1().ConfigMaps("namespace").Get(ctx, pipeline.TranslatePipelineName(fakeManifestWithDependencies.Name), metav1.GetOptions{})

	require.NoError(t, err)
	require.Equal(t, pipeline.ErrorStatus, cfg.Data["status"])

}

func TestDestroyWithErrorDestroyingDivert(t *testing.T) {
	ctx := context.Background()
	k8sClientProvider := test.NewFakeK8sProvider()
	fakeClient, _, err := k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	destroyer := &fakeDestroyer{}
	dc := &destroyCommand{
		getManifest: func(_ string, _ afero.Fs) (*model.Manifest, error) {
			return fakeManifestWithDivert, nil
		},
		ConfigMapHandler:  NewConfigmapHandler(fakeClient),
		nsDestroyer:       destroyer,
		secrets:           &fakeSecretHandler{},
		k8sClientProvider: k8sClientProvider,
		getDivertDriver: func(_ *model.DivertDeploy, _, _ string, _ kubernetes.Interface) (divert.Driver, error) {
			return nil, assert.AnError
		},
	}

	err = dc.destroy(ctx, &Options{
		Name: fakeManifest.Name,
	})

	require.Error(t, err)
	require.False(t, destroyer.destroyed)
	require.False(t, destroyer.destroyedVolumes)

}

func TestDestroyWithErrorOnCommands(t *testing.T) {
	ctx := context.Background()
	k8sClientProvider := test.NewFakeK8sProvider()
	fakeClient, _, err := k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	destroyer := &fakeDestroyer{}
	dc := &destroyCommand{
		getManifest: func(_ string, _ afero.Fs) (*model.Manifest, error) {
			return fakeManifest, nil
		},
		ConfigMapHandler:  NewConfigmapHandler(fakeClient),
		nsDestroyer:       destroyer,
		secrets:           &fakeSecretHandler{},
		k8sClientProvider: k8sClientProvider,
		executor: &fakeExecutor{
			err: assert.AnError,
		},
		buildCtrl: buildCtrl{
			builder: fakeBuilderV2{
				getSvcs: fakeGetSvcs{},
				build:   nil,
			},
		},
	}

	err = dc.destroy(ctx, &Options{
		Name:      fakeManifest.Name,
		Namespace: "namespace",
	})

	require.Error(t, err)
	require.False(t, destroyer.destroyed)
	require.False(t, destroyer.destroyedVolumes)

	cfg, err := fakeClient.CoreV1().ConfigMaps("namespace").Get(ctx, pipeline.TranslatePipelineName(fakeManifestWithDependencies.Name), metav1.GetOptions{})

	require.NoError(t, err)
	require.Equal(t, pipeline.ErrorStatus, cfg.Data["status"])
}

func TestDestroyWithErrorOnCommandsForcingDestroy(t *testing.T) {
	ctx := context.Background()
	k8sClientProvider := test.NewFakeK8sProvider()
	fakeClient, _, err := k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	destroyer := &fakeDestroyer{}
	dc := &destroyCommand{
		getManifest: func(_ string, _ afero.Fs) (*model.Manifest, error) {
			return fakeManifest, nil
		},
		ConfigMapHandler:  NewConfigmapHandler(fakeClient),
		nsDestroyer:       destroyer,
		secrets:           &fakeSecretHandler{},
		k8sClientProvider: k8sClientProvider,
		executor: &fakeExecutor{
			err: fmt.Errorf("error executing command"),
		},
		buildCtrl: buildCtrl{
			builder: fakeBuilderV2{
				getSvcs: fakeGetSvcs{},
				build:   nil,
			},
		},
	}

	err = dc.destroy(ctx, &Options{
		Name:         fakeManifest.Name,
		ForceDestroy: true,
		Namespace:    "namespace",
	})

	require.ErrorContains(t, err, "error executing command")
	require.True(t, destroyer.destroyed)
	require.True(t, destroyer.destroyedVolumes)

	_, err = fakeClient.CoreV1().ConfigMaps("namespace").Get(ctx, pipeline.TranslatePipelineName(fakeManifestWithDependencies.Name), metav1.GetOptions{})

	require.True(t, okerrors.IsNotFound(err))
}

func TestDestroyWithErrorDestroyingK8sResources(t *testing.T) {
	ctx := context.Background()
	k8sClientProvider := test.NewFakeK8sProvider()
	fakeClient, _, err := k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	destroyer := &fakeDestroyer{
		errOnVolumes: assert.AnError,
	}
	dc := &destroyCommand{
		getManifest: func(_ string, _ afero.Fs) (*model.Manifest, error) {
			return fakeManifest, nil
		},
		ConfigMapHandler:  NewConfigmapHandler(fakeClient),
		nsDestroyer:       destroyer,
		secrets:           &fakeSecretHandler{},
		k8sClientProvider: k8sClientProvider,
		executor:          &fakeExecutor{},
		buildCtrl: buildCtrl{
			builder: fakeBuilderV2{
				getSvcs: fakeGetSvcs{},
				build:   nil,
			},
		},
	}

	err = dc.destroy(ctx, &Options{
		Name:      fakeManifest.Name,
		Namespace: "namespace",
	})

	require.Error(t, err)
	require.False(t, destroyer.destroyed)
	require.False(t, destroyer.destroyedVolumes)

	cfg, err := fakeClient.CoreV1().ConfigMaps("namespace").Get(ctx, pipeline.TranslatePipelineName(fakeManifestWithDependencies.Name), metav1.GetOptions{})

	require.NoError(t, err)
	require.Equal(t, pipeline.ErrorStatus, cfg.Data["status"])
}

func TestDestroyK8sResourcesWithErrorDestroyingVolumes(t *testing.T) {
	ctx := context.Background()
	opts := &Options{
		Name:           "test-app",
		DestroyVolumes: true,
	}

	destroyer := &fakeDestroyer{
		errOnVolumes: assert.AnError,
	}
	dc := &destroyCommand{
		nsDestroyer: destroyer,
	}

	err := dc.destroyK8sResources(ctx, opts)

	require.ErrorIs(t, err, assert.AnError)
	require.False(t, destroyer.destroyed)
}

func TestDestroyK8sResourcesWithErrorDestroyingHelmAppWithoutForce(t *testing.T) {
	ctx := context.Background()
	opts := &Options{
		Name:           "test-app",
		DestroyVolumes: true,
		ForceDestroy:   false,
	}

	destroyer := &fakeDestroyer{}
	dc := &destroyCommand{
		nsDestroyer: destroyer,
		secrets: &fakeSecretHandler{
			err: assert.AnError,
		},
	}

	err := dc.destroyK8sResources(ctx, opts)

	require.ErrorIs(t, err, assert.AnError)
	require.True(t, destroyer.destroyedVolumes)
}

func TestDestroyK8sResourcesWithErrorDestroyingHelmAppWithForce(t *testing.T) {
	ctx := context.Background()
	opts := &Options{
		Name:           "test-app",
		DestroyVolumes: true,
		ForceDestroy:   true,
	}

	destroyer := &fakeDestroyer{}
	dc := &destroyCommand{
		nsDestroyer: destroyer,
		secrets: &fakeSecretHandler{
			err: assert.AnError,
		},
	}

	err := dc.destroyK8sResources(ctx, opts)

	require.NoError(t, err)
	require.True(t, destroyer.destroyedVolumes)
	require.True(t, destroyer.destroyed)
}

func TestDestroyK8sResourcesWithErrorDestroyingWithLabel(t *testing.T) {
	ctx := context.Background()
	opts := &Options{
		Name:           "test-app",
		DestroyVolumes: true,
	}

	destroyer := &fakeDestroyer{
		err: assert.AnError,
	}
	dc := &destroyCommand{
		nsDestroyer: destroyer,
		secrets:     &fakeSecretHandler{},
	}

	err := dc.destroyK8sResources(ctx, opts)

	require.ErrorIs(t, err, assert.AnError)
	require.True(t, destroyer.destroyedVolumes)
	require.False(t, destroyer.destroyed)
}

func TestDestroyK8sResourcesWithoutErrors(t *testing.T) {
	ctx := context.Background()
	opts := &Options{
		Name:           "test-app",
		DestroyVolumes: true,
	}

	destroyer := &fakeDestroyer{}
	dc := &destroyCommand{
		nsDestroyer: destroyer,
		secrets:     &fakeSecretHandler{},
	}

	err := dc.destroyK8sResources(ctx, opts)

	require.NoError(t, err)
	require.True(t, destroyer.destroyedVolumes)
	require.True(t, destroyer.destroyed)
}

func TestShouldRunInRemoteDestroy(t *testing.T) {
	var tempManifest = &model.Manifest{
		Destroy: &model.DestroyInfo{
			Remote: true,
		},
	}
	var tests = []struct {
		Name          string
		opts          *Options
		remoteDestroy string
		remoteForce   string
		expected      bool
	}{
		{
			Name: "Okteto_Deploy_Remote env variable is set to True",
			opts: &Options{
				RunInRemote: false,
			},
			remoteDestroy: "True",
			remoteForce:   "",
			expected:      false,
		},
		{
			Name: "Okteto_Force_Remote env variable is set to True",
			opts: &Options{
				RunInRemote: true,
			},
			remoteDestroy: "",
			remoteForce:   "True",
			expected:      true,
		},
		{
			Name: "Remote flag is set to True by CLI",
			opts: &Options{
				RunInRemote: true,
			},
			remoteDestroy: "",
			remoteForce:   "",
			expected:      true,
		},
		{
			Name: "Remote option set by manifest is True & Image is not nil",
			opts: &Options{
				Manifest: tempManifest,
			},
			remoteDestroy: "",
			remoteForce:   "",
			expected:      true,
		},
		{
			Name: "Remote option set by manifest is True and Image is not nil",
			opts: &Options{
				Manifest: tempManifest,
			},
			remoteDestroy: "",
			remoteForce:   "",
			expected:      true,
		},
		{
			Name: "Remote option set by manifest is True and Image is nil",
			opts: &Options{
				Manifest: &model.Manifest{
					Destroy: &model.DestroyInfo{
						Image:  "",
						Remote: true,
					},
				},
			},
			remoteDestroy: "",
			remoteForce:   "",
			expected:      true,
		},
		{
			Name: "Remote option set by manifest is False and Image is nil",
			opts: &Options{
				Manifest: &model.Manifest{
					Destroy: &model.DestroyInfo{
						Image:  "",
						Remote: false,
					},
				},
			},
			remoteDestroy: "",
			remoteForce:   "",
			expected:      false,
		},
		{
			Name: "Default case",
			opts: &Options{
				RunInRemote: false,
			},
			remoteDestroy: "",
			remoteForce:   "",
			expected:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Setenv(constants.OktetoDeployRemote, tt.remoteDestroy)
			t.Setenv(constants.OktetoForceRemote, tt.remoteForce)
			result := shouldRunInRemote(tt.opts)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestDestroyDivertWithErrorGettingKubernetesClient(t *testing.T) {
	k8sClientProvider := test.NewFakeK8sProvider()
	k8sClientProvider.ErrProvide = assert.AnError
	dc := &destroyCommand{
		k8sClientProvider: k8sClientProvider,
	}

	ctx := context.Background()

	err := dc.destroyDivert(ctx, fakeManifestWithDivert)

	require.ErrorIs(t, err, assert.AnError)
}

func TestDestroyDivertWithErrorCreatingDivertDriver(t *testing.T) {
	k8sClientProvider := test.NewFakeK8sProvider()

	dc := &destroyCommand{
		k8sClientProvider: k8sClientProvider,
		getDivertDriver: func(_ *model.DivertDeploy, _, _ string, _ kubernetes.Interface) (divert.Driver, error) {
			return nil, assert.AnError
		},
	}

	ctx := context.Background()

	err := dc.destroyDivert(ctx, fakeManifestWithDivert)

	require.ErrorIs(t, err, assert.AnError)
}

func TestDestroyDivertWithoutError(t *testing.T) {
	k8sClientProvider := test.NewFakeK8sProvider()

	divertDriver := &fakeDivertDriver{}
	dc := &destroyCommand{
		k8sClientProvider: k8sClientProvider,
		getDivertDriver: func(_ *model.DivertDeploy, _, _ string, _ kubernetes.Interface) (divert.Driver, error) {
			return divertDriver, nil
		},
	}

	divertDriver.On("Destroy", mock.Anything).Return(nil)
	ctx := context.Background()

	err := dc.destroyDivert(ctx, fakeManifestWithDivert)

	require.NoError(t, err)
	divertDriver.AssertExpectations(t)
}

func TestDestroyDependenciesWithErrorGettingCommand(t *testing.T) {
	dc := &destroyCommand{
		getPipelineDestroyer: func() (pipelineDestroyer, error) {
			return nil, assert.AnError
		},
	}

	opts := &Options{
		Manifest: fakeManifestWithDependencies,
	}
	ctx := context.Background()

	err := dc.destroyDependencies(ctx, opts)

	require.ErrorIs(t, err, assert.AnError)
}

func TestDestroyDependenciesWithErrorDeletingDep(t *testing.T) {
	pipDestroyer := &fakePipelineDestroyer{}
	pipDestroyer.On("ExecuteDestroyPipeline", mock.Anything, mock.Anything).Return(assert.AnError)
	dc := &destroyCommand{
		getPipelineDestroyer: func() (pipelineDestroyer, error) {
			return pipDestroyer, nil
		},
	}

	opts := &Options{
		Manifest: fakeManifestWithDependencies,
	}
	ctx := context.Background()

	err := dc.destroyDependencies(ctx, opts)

	require.ErrorIs(t, err, assert.AnError)
	pipDestroyer.AssertExpectations(t)
}

func TestDestroyDependenciesWithoutError(t *testing.T) {
	pipDestroyer := &fakePipelineDestroyer{}
	expectedOpts1 := &pipelineCMD.DestroyOptions{
		Name:      "dep1",
		Namespace: "test-namespace",
	}
	expectedOpts2 := &pipelineCMD.DestroyOptions{
		Name:      "dep2",
		Namespace: "test-namespace",
	}
	expectedOpts3 := &pipelineCMD.DestroyOptions{
		Name:      "dep3",
		Namespace: "another-test-namespace",
	}
	pipDestroyer.On("ExecuteDestroyPipeline", mock.Anything, expectedOpts1).Return(nil)
	pipDestroyer.On("ExecuteDestroyPipeline", mock.Anything, expectedOpts2).Return(nil)
	pipDestroyer.On("ExecuteDestroyPipeline", mock.Anything, expectedOpts3).Return(nil)
	dc := &destroyCommand{
		getPipelineDestroyer: func() (pipelineDestroyer, error) {
			return pipDestroyer, nil
		},
	}

	opts := &Options{
		Manifest: fakeManifestWithDependencies,
	}
	ctx := context.Background()

	err := dc.destroyDependencies(ctx, opts)

	require.NoError(t, err)
	pipDestroyer.AssertExpectations(t)
}

func TestGetDestroyer(t *testing.T) {
	tests := []struct {
		expectedType interface{}
		opts         *Options
		name         string
	}{
		{
			name: "local",
			opts: &Options{
				RunInRemote: false,
			},
			expectedType: &localDestroyCommand{},
		},
		{
			name: "remote",
			opts: &Options{
				RunInRemote: true,
				Manifest:    &model.Manifest{},
			},
			expectedType: &remoteDestroyCommand{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := &destroyCommand{}
			deployer := dc.getDestroyer(tt.opts)
			require.IsType(t, tt.expectedType, deployer)
		})
	}
}

func TestDestroyHelmReleasesIfPresentWithErrorGettingSecrets(t *testing.T) {
	secrets := &fakeSecretHandler{
		err: assert.AnError,
	}
	dc := &destroyCommand{
		secrets: secrets,
	}

	ctx := context.Background()
	opts := &Options{
		Namespace: "test",
	}
	labelSelector := ""
	err := dc.destroyHelmReleasesIfPresent(ctx, opts, labelSelector)

	require.ErrorIs(t, err, assert.AnError)
}

func TestDestroyHelmReleasesIfPresentWithoutError(t *testing.T) {
	secrets := &fakeSecretHandler{
		secrets: []v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secret-1",
					Labels: map[string]string{
						ownerLabel: helmOwner,
						nameLabel:  "helm-app",
					},
				},
				Type: model.HelmSecretType,
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secret-2",
					Labels: map[string]string{
						ownerLabel: helmOwner,
						nameLabel:  "helm-app-2",
					},
				},
				Type: model.HelmSecretType,
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "secret-3",
					Labels: map[string]string{},
				},
				Type: "Opaque",
			},
		},
	}
	executor := &fakeExecutor{}

	dc := &destroyCommand{
		secrets:  secrets,
		executor: executor,
	}

	ctx := context.Background()
	opts := &Options{}
	labelSelector := ""
	err := dc.destroyHelmReleasesIfPresent(ctx, opts, labelSelector)

	require.NoError(t, err)
	expectedExecutedCommands := []model.DeployCommand{
		{
			Name:    fmt.Sprintf(helmUninstallCommand, "helm-app"),
			Command: fmt.Sprintf(helmUninstallCommand, "helm-app"),
		},
		{
			Name:    fmt.Sprintf(helmUninstallCommand, "helm-app-2"),
			Command: fmt.Sprintf(helmUninstallCommand, "helm-app-2"),
		},
	}
	require.ElementsMatch(t, executor.executed, expectedExecutedCommands)
}

func TestDestroyHelmReleasesIfPresentWithErrorExecutingCommand(t *testing.T) {
	secrets := &fakeSecretHandler{
		secrets: []v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secret-1",
					Labels: map[string]string{
						ownerLabel: helmOwner,
						nameLabel:  "helm-app",
					},
				},
				Type: model.HelmSecretType,
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secret-2",
					Labels: map[string]string{
						ownerLabel: helmOwner,
						nameLabel:  "helm-app-2",
					},
				},
				Type: model.HelmSecretType,
			},
		},
	}
	executor := &fakeExecutor{
		err: assert.AnError,
	}

	dc := &destroyCommand{
		secrets:  secrets,
		executor: executor,
	}

	ctx := context.Background()
	opts := &Options{}
	labelSelector := ""
	err := dc.destroyHelmReleasesIfPresent(ctx, opts, labelSelector)

	require.ErrorIs(t, err, assert.AnError)
	// Ideally we should check if the executed command is the expected one to delete one helm release, but as the function
	// internally uses a map, the order of helm releases is not the same as the one in the secrets list, and it provokes
	// random failures. As it is not critical the order of helm destruction, we are just checking that just one command
	// is executed
	require.True(t, len(executor.executed) == 1)
}

func TestDestroyHelmReleasesIfPresentWithErrorExecutingCommandWithForceDestroy(t *testing.T) {
	secrets := &fakeSecretHandler{
		secrets: []v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secret-1",
					Labels: map[string]string{
						ownerLabel: helmOwner,
						nameLabel:  "helm-app",
					},
				},
				Type: model.HelmSecretType,
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secret-2",
					Labels: map[string]string{
						ownerLabel: helmOwner,
						nameLabel:  "helm-app-2",
					},
				},
				Type: model.HelmSecretType,
			},
		},
	}
	executor := &fakeExecutor{
		err: assert.AnError,
	}

	dc := &destroyCommand{
		secrets:  secrets,
		executor: executor,
	}

	ctx := context.Background()
	opts := &Options{
		ForceDestroy: true,
	}
	labelSelector := ""
	err := dc.destroyHelmReleasesIfPresent(ctx, opts, labelSelector)

	require.NoError(t, err)
	// As there is an error in the command execution and there is no force flag,
	// the second command should not be executed
	expectedExecutedCommands := []model.DeployCommand{
		{
			Name:    fmt.Sprintf(helmUninstallCommand, "helm-app"),
			Command: fmt.Sprintf(helmUninstallCommand, "helm-app"),
		},
		{
			Name:    fmt.Sprintf(helmUninstallCommand, "helm-app-2"),
			Command: fmt.Sprintf(helmUninstallCommand, "helm-app-2"),
		},
	}
	require.ElementsMatch(t, executor.executed, expectedExecutedCommands)
}

func TestHasDivert(t *testing.T) {
	tests := []struct {
		manifest *model.Manifest
		name     string
		expected bool
	}{
		{
			name:     "NoDeploySection",
			manifest: &model.Manifest{},
			expected: false,
		},
		{
			name: "NoDivertSection",
			manifest: &model.Manifest{
				Deploy: &model.DeployInfo{
					Commands: make([]model.DeployCommand, 0),
				},
			},
			expected: false,
		},
		{
			name: "WithDivertAndDifferentNamespaces",
			manifest: &model.Manifest{
				Namespace: "manifest-namespace",
				Deploy: &model.DeployInfo{
					Divert: &model.DivertDeploy{
						Namespace: "test-namespace",
					},
				},
			},
			expected: true,
		},
		{
			name: "WithDivertAndSameNamespace",
			manifest: &model.Manifest{
				Namespace: "test-namespace",
				Deploy: &model.DeployInfo{
					Divert: &model.DivertDeploy{
						Namespace: "test-namespace",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, hasDivert(tt.manifest))
		})
	}
}
