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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deps"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd/api"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var errorManifest *model.Manifest = &model.Manifest{
	Name: "testManifest",
	Build: build.ManifestBuild{
		"service1": &build.Info{
			Dockerfile: "Dockerfile",
			Image:      "testImage",
		},
	},
	Deploy: &model.DeployInfo{
		Commands: []model.DeployCommand{
			{
				Name:    "printenv",
				Command: "printenv",
			},
		},
	},
}

type fakeRegistry struct {
	registry map[string]fakeImage
}

// fakeImage represents the data from an image
type fakeImage struct {
	Registry string
	Repo     string
	Tag      string
	ImageRef string
	Args     []string
}

func newFakeRegistry() fakeRegistry {
	return fakeRegistry{
		registry: map[string]fakeImage{},
	}
}

func (fr fakeRegistry) GetImageTagWithDigest(imageTag string) (string, error) {
	if _, ok := fr.registry[imageTag]; !ok {
		return "", oktetoErrors.ErrNotFound
	}
	return imageTag, nil
}

func (fr fakeRegistry) GetImageReference(image string) (registry.OktetoImageReference, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return registry.OktetoImageReference{}, err
	}
	return registry.OktetoImageReference{
		Registry: ref.Context().RegistryStr(),
		Repo:     ref.Context().RepositoryStr(),
		Tag:      ref.Identifier(),
		Image:    image,
	}, nil
}

func (fr fakeRegistry) HasGlobalPushAccess() (bool, error) { return false, nil }

func (fr fakeRegistry) IsOktetoRegistry(_ string) bool { return false }

func (fr fakeRegistry) AddImageByOpts(opts *types.BuildOptions) error {
	fr.registry[opts.Tag] = fakeImage{Args: opts.BuildArgs}
	return nil
}

func (fr fakeRegistry) IsGlobalRegistry(image string) bool { return false }

func (fr fakeRegistry) GetRegistryAndRepo(image string) (string, string) { return "", "" }
func (fr fakeRegistry) GetRepoNameAndTag(repo string) (string, string)   { return "", "" }
func (fr fakeRegistry) CloneGlobalImageToDev(string) (string, error) {
	return "", nil
}

var fakeManifest *model.Manifest = &model.Manifest{
	Deploy: &model.DeployInfo{
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

var fakeManifestWithDependency *model.Manifest = &model.Manifest{
	Dependencies: deps.ManifestSection{
		"a": &deps.Dependency{
			Namespace: "b",
		},
		"b": &deps.Dependency{},
	},
}

var noDeployNorDependenciesManifest *model.Manifest = &model.Manifest{
	Name: "testManifest",
	Build: build.ManifestBuild{
		"service1": &build.Info{
			Dockerfile: "Dockerfile",
			Image:      "testImage",
		},
	},
}

type fakeV2Builder struct {
	buildErr             error
	buildOptionsStorage  *types.BuildOptions
	servicesAlreadyBuilt []string
}

func (b *fakeV2Builder) Build(_ context.Context, buildOptions *types.BuildOptions) error {
	if b.buildErr != nil {
		return b.buildErr
	}
	b.buildOptionsStorage = buildOptions
	return nil
}

func (b *fakeV2Builder) GetServicesToBuildDuringDeploy(_ context.Context, manifest *model.Manifest, servicesToDeploy []string) ([]string, error) {
	toBuild := make(map[string]bool, len(manifest.Build))
	for service := range manifest.Build {
		toBuild[service] = true
	}

	return setToSlice(setDifference(setIntersection(toBuild, sliceToSet(servicesToDeploy)), sliceToSet(b.servicesAlreadyBuilt))), nil
}

func (*fakeV2Builder) GetBuildEnvVars() map[string]string {
	return nil
}

type fakeDeployer struct {
	mock.Mock
}

func getManifestWithError(_ string, _ afero.Fs) (*model.Manifest, error) {
	return nil, assert.AnError
}

func getFakeManifest(_ string, _ afero.Fs) (*model.Manifest, error) {
	return fakeManifest, nil
}

func getErrorManifest(_ string, _ afero.Fs) (*model.Manifest, error) {
	return errorManifest, nil
}

func getManifestWithNoDeployNorDependency(_ string, _ afero.Fs) (*model.Manifest, error) {
	return noDeployNorDependenciesManifest, nil
}

func getFakeManifestWithDependency(_ string, _ afero.Fs) (*model.Manifest, error) {
	return fakeManifestWithDependency, nil
}

type fakeEndpointControl struct {
	err       error
	endpoints []string
}

func (f *fakeEndpointControl) List(_ context.Context, _ *EndpointsOptions, _ string) ([]string, error) {
	return f.endpoints, f.err
}

func getFakeEndpoint(_ *io.K8sLogger) (EndpointGetter, error) {
	return EndpointGetter{
		endpointControl: &fakeEndpointControl{},
	}, nil
}

func (f *fakeDeployer) Get(ctx context.Context,
	opts *Options,
	buildEnvVarsGetter buildEnvVarsGetter,
	cmapHandler ConfigMapHandler,
	k8sProvider okteto.K8sClientProviderWithLogger,
	ioCtrl *io.Controller,
	k8Logger *io.K8sLogger,
	dependencyEnvVarsGetter dependencyEnvVarsGetter,
) (Deployer, error) {
	args := f.Called(ctx, opts, buildEnvVarsGetter, cmapHandler, k8sProvider, ioCtrl, k8Logger, dependencyEnvVarsGetter)
	return args.Get(0).(Deployer), args.Error(1)
}

func (f *fakeDeployer) Deploy(ctx context.Context, opts *Options) error {
	args := f.Called(ctx, opts)
	return args.Error(0)
}

func (f *fakeDeployer) CleanUp(ctx context.Context, err error) {
	f.Called(ctx, err)
}

func TestDeployWithErrorReadingManifestFile(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				Cfg:       &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	fakeDeployer := &fakeDeployer{}
	c := &Command{
		GetManifest:       getManifestWithError,
		GetDeployer:       fakeDeployer.Get,
		K8sClientProvider: test.NewFakeK8sProvider(),
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	err := c.Run(ctx, opts)

	// Verify the deploy phase is not even reached
	fakeDeployer.AssertNotCalled(t, "Get")
	assert.Error(t, err)
}

func TestDeployWithNeitherDeployNorDependencyInManifestFile(t *testing.T) {
	fakeDeployer := &fakeDeployer{}
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				Cfg:       &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	c := &Command{
		GetManifest:       getManifestWithNoDeployNorDependency,
		GetDeployer:       fakeDeployer.Get,
		K8sClientProvider: test.NewFakeK8sProvider(),
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	err := c.Run(ctx, opts)

	assert.ErrorIs(t, err, oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands)
	// Verify the deploy phase is not even reached
	fakeDeployer.AssertNotCalled(t, "Get")
}

func TestDeployWithServicesToBuildWithoutComposeSection(t *testing.T) {
	fakeDeployer := &fakeDeployer{}
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				Cfg:       &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	c := &Command{
		GetManifest:       getFakeManifest,
		GetDeployer:       fakeDeployer.Get,
		K8sClientProvider: test.NewFakeK8sProvider(),
	}
	ctx := context.Background()
	opts := &Options{
		Name:             "movies",
		ManifestPath:     "",
		Variables:        []string{},
		ServicesToDeploy: []string{"service1"},
	}

	err := c.Run(ctx, opts)

	assert.ErrorIs(t, err, oktetoErrors.ErrDeployCantDeploySvcsIfNotCompose)
	// Verify the deploy phase is not even reached
	fakeDeployer.AssertNotCalled(t, "Get")
}

func TestCreateConfigMapWithBuildError(t *testing.T) {
	fakeK8sClientProvider := test.NewFakeK8sProvider()
	opts := &Options{
		Name:         "testErr",
		ManifestPath: "",
		Variables:    []string{},
		Build:        true,
	}

	reg := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(reg)
	okCtx := &okteto.ContextStateless{
		Store: &okteto.ContextStore{
			Contexts: map[string]*okteto.Context{
				"test": {
					Namespace: "test",
					Cfg:       &api.Config{},
				},
			},
			CurrentContext: "test",
		},
	}

	builderV2 := buildv2.NewBuilder(builder, reg, io.NewIOController(), okCtx, io.NewK8sLogger(), []buildv2.OnBuildFinish{})
	c := &Command{
		GetManifest:       getErrorManifest,
		Builder:           builderV2,
		K8sClientProvider: fakeK8sClientProvider,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		Fs:                afero.NewMemMapFs(),
	}

	ctx := context.Background()

	err := c.Run(ctx, opts)

	// we should get a build error because Dockerfile does not exist
	assert.True(t, strings.Contains(err.Error(), oktetoErrors.InvalidDockerfile))

	fakeClient, _, err := c.K8sClientProvider.ProvideWithLogger(clientcmdapi.NewConfig(), nil)
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}

	// sanitizeName is needed to check the CFGmap - this sanitization is done at RunDeploy, labels and cfg name
	sanitizedName := format.ResourceK8sMetaString(opts.Name)

	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(sanitizedName), okteto.GetContext().Namespace, fakeClient)
	assert.NoError(t, err)

	expectedCfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("okteto-git-%s", sanitizedName),
			Namespace: okteto.GetContext().Namespace,
			Labels:    map[string]string{"dev.okteto.com/git-deploy": "true"},
		},
		Data: map[string]string{
			"actionName": "cli",
			"name":       "testErr",
			"output":     "",
			"status":     "error",
			"branch":     "",
			"filename":   "",
			"icon":       "",
			"repository": "",
			"yaml":       "",
		},
	}

	expectedCfg.Data["output"] = cfg.Data["output"]

	assert.Equal(t, expectedCfg.Name, cfg.Name)
	assert.Equal(t, expectedCfg.Namespace, cfg.Namespace)
	assert.Equal(t, expectedCfg.Labels, cfg.Labels)

	keysToCompare := []string{"actionName", "name", "status", "filename", "icon", "yaml"}
	for _, key := range keysToCompare {
		assert.Equal(t, expectedCfg.Data[key], cfg.Data[key])
	}
	assert.NotEmpty(t, cfg.Annotations[constants.LastUpdatedAnnotation])
}

func TestDeployWithErrorDeploying(t *testing.T) {
	fakeOs := afero.NewMemMapFs()
	fakeK8sClientProvider := test.NewFakeK8sProvider()
	fakeDeployer := &fakeDeployer{}
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				Cfg:       &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	c := &Command{
		GetManifest:       getFakeManifest,
		GetDeployer:       fakeDeployer.Get,
		K8sClientProvider: fakeK8sClientProvider,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		Fs:                fakeOs,
		Builder:           &fakeV2Builder{},
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	fakeDeployer.On(
		"Get",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(fakeDeployer, nil)

	expectedOpts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
		Manifest:     fakeManifest,
	}
	fakeDeployer.On("Deploy", mock.Anything, expectedOpts).Return(assert.AnError)

	err := c.Run(ctx, opts)

	assert.Error(t, err)

	// check if configmap has been created
	fakeClient, _, err := c.K8sClientProvider.ProvideWithLogger(clientcmdapi.NewConfig(), nil)
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.GetContext().Namespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, pipeline.ErrorStatus, cfg.Data["status"])

	fakeDeployer.AssertExpectations(t)
}

func TestDeployWithErrorBecauseOtherPipelineRunning(t *testing.T) {
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}
	fakeK8sClientProvider := test.NewFakeK8sProvider(&apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.TranslatePipelineName(opts.Name),
			Namespace: "test",
		},
		Data: map[string]string{
			"actionLock": "test",
		},
	}, &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				model.DeployedByLabel: "movies",
			},
		},
	})
	fakeDeployer := &fakeDeployer{}

	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				Cfg:       &api.Config{},
			},
		},
		CurrentContext: "test",
	}

	c := &Command{
		GetManifest:       getFakeManifest,
		GetDeployer:       fakeDeployer.Get,
		K8sClientProvider: fakeK8sClientProvider,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		Fs:                afero.NewMemMapFs(),
	}
	ctx := context.Background()

	err := c.Run(ctx, opts)

	assert.Error(t, err)

	// Verify the deploy phase is not even reached
	fakeDeployer.AssertNotCalled(t, "Get")

	// check if configmap has been created
	fakeClient, _, err := c.K8sClientProvider.ProvideWithLogger(clientcmdapi.NewConfig(), nil)
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.GetContext().Namespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
}

func TestDeployWithoutErrors(t *testing.T) {
	fakeOs := afero.NewMemMapFs()
	fakeK8sClientProvider := test.NewFakeK8sProvider(&v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				model.DeployedByLabel: "movies",
			},
			Namespace: "test",
		},
	})
	fakeDeployer := &fakeDeployer{}

	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				Cfg:       &api.Config{},
			},
		},
		CurrentContext: "test",
	}

	c := &Command{
		GetManifest:       getFakeManifest,
		K8sClientProvider: fakeK8sClientProvider,
		EndpointGetter:    getFakeEndpoint,
		Fs:                fakeOs,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		GetDeployer:       fakeDeployer.Get,
		Builder:           &fakeV2Builder{},
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	fakeDeployer.On(
		"Get",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(fakeDeployer, nil)

	expectedOpts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
		Manifest:     fakeManifest,
	}
	fakeDeployer.On("Deploy", mock.Anything, expectedOpts).Return(nil)

	err := c.Run(ctx, opts)

	assert.NoError(t, err)

	// check if configmap has been created
	fakeClient, _, err := c.K8sClientProvider.ProvideWithLogger(clientcmdapi.NewConfig(), nil)
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.GetContext().Namespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, pipeline.DeployedStatus, cfg.Data["status"])
}

func TestGetDefaultTimeout(t *testing.T) {
	tt := []struct {
		name       string
		envarValue string
		expected   time.Duration
	}{
		{
			name:       "env var not set",
			envarValue: "",
			expected:   5 * time.Minute,
		},
		{
			name:       "env var set with not the proper syntax",
			envarValue: "hello world",
			expected:   5 * time.Minute,
		},
		{
			name:       "env var set",
			envarValue: "10m",
			expected:   10 * time.Minute,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(model.OktetoTimeoutEnvVar, tc.envarValue)
			assert.Equal(t, tc.expected, getDefaultTimeout())
		})
	}
}

type fakePipelineDeployer struct {
	err error
}

func (fd fakePipelineDeployer) ExecuteDeployPipeline(_ context.Context, _ *pipelineCMD.DeployOptions) error {
	return fd.err
}

func TestDeployDependencies(t *testing.T) {
	fakeManifest := &model.Manifest{
		Dependencies: deps.ManifestSection{
			"a": &deps.Dependency{
				Namespace: "b",
			},
			"b": &deps.Dependency{},
		},
	}

	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
				Cfg:       &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	type config struct {
		pipelineErr error
	}
	tt := []struct {
		config   config
		expected error
		name     string
	}{
		{
			name:     "error deploying dependency",
			config:   config{pipelineErr: assert.AnError},
			expected: assert.AnError,
		},
		{
			name: "successful",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			dc := &Command{
				PipelineCMD: fakePipelineDeployer{tc.config.pipelineErr},
			}
			assert.ErrorIs(t, tc.expected, dc.deployDependencies(context.Background(), &Options{Manifest: fakeManifest}))
		})
	}
}

func TestDeployOnlyDependencies(t *testing.T) {
	fakeOs := afero.NewMemMapFs()
	fakeK8sClientProvider := test.NewFakeK8sProvider(&v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				model.DeployedByLabel: "movies",
			},
			Namespace: "test",
		},
	})

	fakeDeployer := &fakeDeployer{}

	c := &Command{
		PipelineCMD:       fakePipelineDeployer{nil},
		GetManifest:       getFakeManifestWithDependency,
		K8sClientProvider: fakeK8sClientProvider,
		Fs:                fakeOs,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		GetDeployer:       fakeDeployer.Get,
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	tt := []struct {
		expecterErr error
		name        string
		isOkteto    bool
	}{
		{
			name:        "deploy dependency with no error",
			expecterErr: nil,
			isOkteto:    true,
		},
		{
			name:        "error okteto not installed",
			expecterErr: errDepenNotAvailableInVanilla,
			isOkteto:    false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"test": {
						Namespace: "test",
						IsOkteto:  tc.isOkteto,
						Cfg:       &api.Config{},
					},
				},
				CurrentContext: "test",
			}

			err := c.Run(ctx, opts)

			require.ErrorIs(t, err, tc.expecterErr)
		})
	}
}

type fakeTracker struct{}

func (*fakeTracker) TrackImageBuild(context.Context, *analytics.ImageBuildMetadata) {}
func (*fakeTracker) TrackDeploy(analytics.DeployMetadata)                           {}

func TestTrackDeploy(t *testing.T) {
	tt := []struct {
		commandErr error
		manifest   *model.Manifest
		name       string
		remoteFlag bool
	}{
		{
			name:       "error tracking deploy",
			commandErr: assert.AnError,
		},
		{
			name: "successful with V2",
			manifest: &model.Manifest{
				IsV2: true,
				Deploy: &model.DeployInfo{
					Commands: []model.DeployCommand{
						{
							Name:    "test",
							Command: "test",
						},
					},
				},
			},
			remoteFlag: true,
		},
		{
			name: "successful with compose",
			manifest: &model.Manifest{
				IsV2: true,
				Deploy: &model.DeployInfo{
					ComposeSection: &model.ComposeSectionInfo{
						ComposesInfo: model.ComposeInfoList{},
					},
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			dc := &Command{
				AnalyticsTracker: &fakeTracker{},
			}

			dc.TrackDeploy(tc.manifest, tc.remoteFlag, time.Now(), tc.commandErr)
		})
	}
}

func TestShouldRunInRemoteDeploy(t *testing.T) {
	tempManifest := &model.Manifest{
		Deploy: &model.DeployInfo{
			Remote: true,
			Image:  "some-image",
		},
	}
	var tests = []struct {
		Name         string
		opts         *Options
		remoteDeploy string
		remoteForce  string
		expected     bool
	}{
		{
			Name: "Okteto_Deploy_Remote env is set to True",
			opts: &Options{
				RunInRemote: false,
			},
			remoteDeploy: "true",
			remoteForce:  "",
			expected:     false,
		},
		{
			Name: "Remote flag is set to True",
			opts: &Options{
				RunInRemote: true,
			},
			remoteDeploy: "",
			remoteForce:  "",
			expected:     true,
		},
		{
			Name: "Remote option set by manifest is True and Image is not nil",
			opts: &Options{
				Manifest: tempManifest,
			},
			remoteDeploy: "",
			remoteForce:  "",
			expected:     true,
		},
		{
			Name: "Remote option set by manifest is True and Image is nil",
			opts: &Options{
				Manifest: &model.Manifest{
					Deploy: &model.DeployInfo{
						Image:  "",
						Remote: true,
					},
				},
			},
			remoteDeploy: "",
			remoteForce:  "",
			expected:     true,
		},
		{
			Name: "Remote option set by manifest is False and Image is nil",
			opts: &Options{
				Manifest: &model.Manifest{
					Deploy: &model.DeployInfo{
						Image:  "",
						Remote: false,
					},
				},
			},
			remoteDeploy: "",
			remoteForce:  "",
			expected:     false,
		},
		{
			Name: "Okteto_Force_Remote env is set to True",
			opts: &Options{
				RunInRemote: false,
			},
			remoteDeploy: "",
			remoteForce:  "true",
			expected:     true,
		},
		{
			Name: "Default case",
			opts: &Options{
				RunInRemote: false,
			},
			remoteDeploy: "",
			remoteForce:  "",
			expected:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Setenv(constants.OktetoDeployRemote, tt.remoteDeploy)
			t.Setenv(constants.OktetoForceRemote, tt.remoteForce)
			result := shouldRunInRemote(tt.opts)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestOktetoManifestPathFlag(t *testing.T) {
	opts := &Options{}
	var tests = []struct {
		expectedErr error
		name        string
		manifest    string
	}{
		{
			name:        "manifest file path exists",
			manifest:    "okteto.yml",
			expectedErr: nil,
		},
		{
			name:        "manifest file path doesn't exist",
			manifest:    "nonexistent.yml",
			expectedErr: fmt.Errorf("nonexistent.yml file doesn't exist"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewOsFs()
			dir, err := os.Getwd()
			assert.NoError(t, err)
			fullpath := filepath.Join(dir, tt.manifest)
			opts.ManifestPath = fullpath
			if tt.manifest != "nonexistent.yml" {
				// create the manifest file only if it's not the nonexistent scenario
				f, err := fs.Create(fullpath)
				assert.NoError(t, err)
				defer func() {
					if err := f.Close(); err != nil {
						t.Fatalf("Error closing file %s: %s", fullpath, err)
					}
					if err := fs.RemoveAll(fullpath); err != nil {
						t.Fatalf("Error removing the file %v", err)
					}
				}()
			}
			err = checkOktetoManifestPathFlag(opts, fs)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestGetDependencyEnvVars(t *testing.T) {
	tt := []struct {
		environGetter environGetter
		expected      map[string]string
		name          string
	}{
		{
			name: "WithEmptyEnvironment",
			environGetter: func() []string {
				return []string{}
			},
			expected: map[string]string{},
		},
		{
			name: "WithEnvironmentWithoutDependencyVars",
			environGetter: func() []string {
				return []string{
					"OKTETO_NAMESPACE=test",
					"OKTETO_USER=cindy",
					"MYVAR=foo",
				}
			},
			expected: map[string]string{},
		},
		{
			name: "WithEnvironmentOnlyWithDependencyVars",
			environGetter: func() []string {
				return []string{
					"OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME=dbuser",
					"OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD=mystrongpassword",
					"OKTETO_DEPENDENCY_API_VARIABLE_HOST=apihost",
				}
			},
			expected: map[string]string{
				"OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME": "dbuser",
				"OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD": "mystrongpassword",
				"OKTETO_DEPENDENCY_API_VARIABLE_HOST":          "apihost",
			},
		},
		{
			name: "WithEnvironmentMixingVars",
			environGetter: func() []string {
				return []string{
					"OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME=dbuser",
					"OKTETO_NAMESPACE=test",
					"OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD=mystrongpassword",
					"OKTETO_USER=cindy",
					"OKTETO_DEPENDENCY_API_VARIABLE_HOST=apihost",
					"MYVAR=foo",
				}
			},
			expected: map[string]string{
				"OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME": "dbuser",
				"OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD": "mystrongpassword",
				"OKTETO_DEPENDENCY_API_VARIABLE_HOST":          "apihost",
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			result := GetDependencyEnvVars(tc.environGetter)

			require.Equal(t, tc.expected, result)
		})
	}
}
