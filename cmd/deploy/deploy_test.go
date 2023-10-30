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
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/divert"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/registry"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var errorManifest *model.Manifest = &model.Manifest{
	Name: "testManifest",
	Build: model.ManifestBuild{
		"service1": &model.BuildInfo{
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
func (fr fakeRegistry) CloneGlobalImageToDev(imageWithDigest, tag string) (string, error) {
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
	Dependencies: model.ManifestDependencies{
		"a": &model.Dependency{
			Namespace: "b",
		},
		"b": &model.Dependency{},
	},
}

var noDeployNorDependenciesManifest *model.Manifest = &model.Manifest{
	Name: "testManifest",
	Build: model.ManifestBuild{
		"service1": &model.BuildInfo{
			Dockerfile: "Dockerfile",
			Image:      "testImage",
		},
	},
}

type fakeProxy struct {
	errOnShutdown error
	port          int
	token         string
	started       bool
	shutdown      bool
}

type fakeExecutor struct {
	err      error
	executed []model.DeployCommand
}

type fakeKubeConfig struct {
	config      *rest.Config
	errOnModify error
	errRead     error
}

type fakeCmapHandler struct {
	errUpdatingWithEnvs error
}

func (*fakeCmapHandler) translateConfigMapAndDeploy(context.Context, *pipeline.CfgData) (*apiv1.ConfigMap, error) {
	return nil, nil
}

func (f *fakeCmapHandler) getConfigmapVariablesEncoded(context.Context, string, string) (string, error) {
	return "", nil
}

func (f *fakeCmapHandler) updateConfigMap(context.Context, *apiv1.ConfigMap, *pipeline.CfgData, error) error {
	return nil
}

func (f *fakeCmapHandler) updateEnvsFromCommands(context.Context, string, string, []string) error {
	return f.errUpdatingWithEnvs
}

func (f *fakeKubeConfig) Read() (*rest.Config, error) {
	if f.errRead != nil {
		return nil, f.errRead
	}
	return f.config, nil
}

func (fc *fakeKubeConfig) Modify(_ int, _, _ string) error {
	return fc.errOnModify
}

func (fk *fakeProxy) Start() {
	fk.started = true
}

func (*fakeProxy) SetName(_ string) {}

func (*fakeProxy) SetDivert(_ divert.Driver) {}

func (fk *fakeProxy) Shutdown(_ context.Context) error {
	if fk.errOnShutdown != nil {
		return fk.errOnShutdown
	}

	fk.shutdown = true
	return nil
}

func (fk *fakeProxy) GetPort() int {
	return fk.port
}

func (fk *fakeProxy) GetToken() string {
	return fk.token
}

func (fe *fakeExecutor) Execute(command model.DeployCommand, _ []string) error {
	fe.executed = append(fe.executed, command)
	if fe.err != nil {
		return fe.err
	}

	return nil
}

func (*fakeExecutor) CleanUp(_ error) {}

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

func (b *fakeV2Builder) GetServicesToBuild(_ context.Context, manifest *model.Manifest, servicesToDeploy []string) ([]string, error) {
	toBuild := make(map[string]bool, len(manifest.Build))
	for service := range manifest.Build {
		toBuild[service] = true
	}

	return setToSlice(setDifference(setIntersection(toBuild, sliceToSet(servicesToDeploy)), sliceToSet(b.servicesAlreadyBuilt))), nil
}

func (*fakeV2Builder) GetBuildEnvVars() map[string]string {
	return nil
}

func TestDeployWithErrorChangingKubeConfig(t *testing.T) {
	p := &fakeProxy{}
	e := &fakeExecutor{}
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	c := &localDeployer{
		Proxy:    p,
		Executor: e,
		Kubeconfig: &fakeKubeConfig{
			errOnModify: assert.AnError,
		},
		K8sClientProvider: test.NewFakeK8sProvider(),
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	err := c.deploy(ctx, opts)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, e.executed, 0)
	// Proxy wasn't started
	assert.False(t, p.started)
}

type fakeDeployer struct {
	proxy                   *fakeProxy
	executor                *fakeExecutor
	kubeconfig              *fakeKubeConfig
	fs                      afero.Fs
	k8sClientProvider       *test.FakeK8sProvider
	externalControlProvider fakeExternalControlProvider
}

func (d fakeDeployer) Get(_ context.Context, _ *Options, _ builderInterface, cmapHandler configMapHandler, _ okteto.K8sClientProvider, _ kubeConfigHandler, _ portGetterFunc) (deployerInterface, error) {
	return &localDeployer{
		Proxy:              d.proxy,
		Executor:           d.executor,
		Kubeconfig:         d.kubeconfig,
		Fs:                 d.fs,
		K8sClientProvider:  d.k8sClientProvider,
		GetExternalControl: d.externalControlProvider.getFakeExternalControl,
		ConfigMapHandler:   cmapHandler,
	}, nil
}

func TestDeployWithErrorReadingManifestFile(t *testing.T) {
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	fakeDeployer := &fakeDeployer{
		proxy:      &fakeProxy{},
		executor:   &fakeExecutor{},
		kubeconfig: &fakeKubeConfig{},
		fs:         afero.NewMemMapFs(),
	}
	c := &DeployCommand{
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

	err := c.RunDeploy(ctx, opts)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, fakeDeployer.executor.executed, 0)
	// Proxy wasn't started
	assert.False(t, fakeDeployer.proxy.started)
}

func TestDeployWithNeitherDeployNorDependencyInManifestFile(t *testing.T) {
	fakeDeployer := &fakeDeployer{
		proxy:      &fakeProxy{},
		executor:   &fakeExecutor{},
		kubeconfig: &fakeKubeConfig{},
		fs:         afero.NewMemMapFs(),
	}
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	c := &DeployCommand{
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

	err := c.RunDeploy(ctx, opts)

	assert.ErrorIs(t, err, oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands)

	// No command was executed
	assert.Len(t, fakeDeployer.executor.executed, 0)
	// Proxy wasn't started
	assert.False(t, fakeDeployer.proxy.started)
}

type fakeAnalyticsTracker struct{}

func (fakeAnalyticsTracker) TrackImageBuild(...*analytics.ImageBuildMetadata) {}

func TestCreateConfigMapWithBuildError(t *testing.T) {
	fakeK8sClientProvider := test.NewFakeK8sProvider()
	fakeDeployer := &fakeDeployer{
		proxy: &fakeProxy{},
		executor: &fakeExecutor{
			err: assert.AnError,
		},
		kubeconfig:        &fakeKubeConfig{},
		fs:                afero.NewMemMapFs(),
		k8sClientProvider: fakeK8sClientProvider,
	}
	opts := &Options{
		Name:         "testErr",
		ManifestPath: "",
		Variables:    []string{},
		Build:        true,
	}

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	fakeTracker := fakeAnalyticsTracker{}
	c := &DeployCommand{
		GetManifest:       getErrorManifest,
		GetDeployer:       fakeDeployer.Get,
		Builder:           buildv2.NewBuilder(builder, registry, fakeTracker),
		K8sClientProvider: fakeK8sClientProvider,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider),
	}

	ctx := context.Background()

	err := c.RunDeploy(ctx, opts)

	// we should get a build error because Dockerfile does not exist
	assert.True(t, strings.Contains(err.Error(), oktetoErrors.InvalidDockerfile))

	fakeClient, _, err := c.K8sClientProvider.Provide(clientcmdapi.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}

	// sanitizeName is needed to check the CFGmap - this sanitization is done at RunDeploy, labels and cfg name
	sanitizedName := format.ResourceK8sMetaString(opts.Name)

	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(sanitizedName), okteto.Context().Namespace, fakeClient)
	assert.NoError(t, err)

	expectedCfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("okteto-git-%s", sanitizedName),
			Namespace: okteto.Context().Namespace,
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
	assert.Equal(t, expectedCfg.Data, cfg.Data)
	assert.NotEmpty(t, cfg.Annotations[constants.LastUpdatedAnnotation])
}

func TestDeployWithErrorExecutingCommands(t *testing.T) {
	fakeOs := afero.NewMemMapFs()
	fakeK8sClientProvider := test.NewFakeK8sProvider()
	fakeDeployer := &fakeDeployer{
		proxy: &fakeProxy{},
		executor: &fakeExecutor{
			err: assert.AnError,
		},
		kubeconfig:        &fakeKubeConfig{},
		fs:                fakeOs,
		k8sClientProvider: fakeK8sClientProvider,
	}
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	c := &DeployCommand{
		GetManifest:       getFakeManifest,
		GetDeployer:       fakeDeployer.Get,
		K8sClientProvider: fakeK8sClientProvider,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider),
		Fs:                fakeOs,
		Builder:           &fakeV2Builder{},
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	err := c.RunDeploy(ctx, opts)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, fakeDeployer.executor.executed, 1)
	// Check expected commands were executed
	assert.Equal(t, fakeManifest.Deploy.Commands[0], fakeDeployer.executor.executed[0])
	// Proxy started
	assert.True(t, fakeDeployer.proxy.started)
	// Proxy shutdown
	assert.True(t, fakeDeployer.proxy.shutdown)

	// check if configmap has been created
	fakeClient, _, err := c.K8sClientProvider.Provide(clientcmdapi.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.Context().Namespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, pipeline.ErrorStatus, cfg.Data["status"])
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
	fakeDeployer := &fakeDeployer{
		proxy: &fakeProxy{
			errOnShutdown: assert.AnError,
		},
		executor:          &fakeExecutor{},
		kubeconfig:        &fakeKubeConfig{},
		fs:                afero.NewMemMapFs(),
		k8sClientProvider: fakeK8sClientProvider,
	}

	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}

	c := &DeployCommand{
		GetManifest:       getFakeManifest,
		GetDeployer:       fakeDeployer.Get,
		K8sClientProvider: fakeK8sClientProvider,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider),
	}
	ctx := context.Background()

	err := c.RunDeploy(ctx, opts)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, fakeDeployer.executor.executed, 0)
	// Proxy didn't start
	assert.False(t, fakeDeployer.proxy.started)

	// check if configmap has been created
	fakeClient, _, err := c.K8sClientProvider.Provide(clientcmdapi.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.Context().Namespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
}

func TestDeployWithErrorShuttingdownProxy(t *testing.T) {
	fakeOs := afero.NewMemMapFs()
	fakeK8sClientProvider := test.NewFakeK8sProvider(&v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				model.DeployedByLabel: "movies",
			},
			Namespace: "test",
		},
	})
	fakeExternalControlProvider := fakeExternalControlProvider{
		control: &fakeExternalControl{},
	}
	fakeDeployer := &fakeDeployer{
		proxy: &fakeProxy{
			errOnShutdown: assert.AnError,
		},
		executor:                &fakeExecutor{},
		kubeconfig:              &fakeKubeConfig{},
		fs:                      fakeOs,
		k8sClientProvider:       fakeK8sClientProvider,
		externalControlProvider: fakeExternalControlProvider,
	}

	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
				Cfg:       clientcmdapi.NewConfig(),
			},
		},
		CurrentContext: "test",
	}
	c := &DeployCommand{
		GetManifest:        getFakeManifest,
		GetDeployer:        fakeDeployer.Get,
		GetExternalControl: fakeExternalControlProvider.getFakeExternalControl,
		K8sClientProvider:  fakeK8sClientProvider,
		EndpointGetter:     getFakeEndpoint,
		CfgMapHandler:      newDefaultConfigMapHandler(fakeK8sClientProvider),
		Fs:                 fakeOs,
		Builder:            &fakeV2Builder{},
	}
	ctx := context.Background()

	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	err := c.RunDeploy(ctx, opts)

	assert.NoError(t, err)
	// No command was executed
	assert.Len(t, fakeDeployer.executor.executed, 3)
	// Check expected commands were executed
	assert.Equal(t, fakeManifest.Deploy.Commands, fakeDeployer.executor.executed)
	// Proxy started
	assert.True(t, fakeDeployer.proxy.started)
	// Proxy wasn't shutdown
	assert.False(t, fakeDeployer.proxy.shutdown)

	// check if configmap has been created
	fakeClient, _, err := c.K8sClientProvider.Provide(clientcmdapi.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.Context().Namespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, pipeline.DeployedStatus, cfg.Data["status"])
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
	fakeExternalControlProvider := fakeExternalControlProvider{
		control: &fakeExternalControl{},
	}
	fakeDeployer := &fakeDeployer{
		proxy:                   &fakeProxy{},
		executor:                &fakeExecutor{},
		kubeconfig:              &fakeKubeConfig{},
		fs:                      fakeOs,
		k8sClientProvider:       fakeK8sClientProvider,
		externalControlProvider: fakeExternalControlProvider,
	}

	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}

	c := &DeployCommand{
		GetManifest:        getFakeManifest,
		K8sClientProvider:  fakeK8sClientProvider,
		EndpointGetter:     getFakeEndpoint,
		GetExternalControl: fakeExternalControlProvider.getFakeExternalControl,
		Fs:                 fakeOs,
		CfgMapHandler:      newDefaultConfigMapHandler(fakeK8sClientProvider),
		GetDeployer:        fakeDeployer.Get,
		Builder:            &fakeV2Builder{},
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	err := c.RunDeploy(ctx, opts)

	assert.NoError(t, err)
	// No command was executed
	assert.Len(t, fakeDeployer.executor.executed, 3)
	// Check expected commands were executed
	assert.Equal(t, fakeManifest.Deploy.Commands, fakeDeployer.executor.executed)
	// Proxy started
	assert.True(t, fakeDeployer.proxy.started)
	// Proxy was shutdown
	assert.True(t, fakeDeployer.proxy.shutdown)

	// check if configmap has been created
	fakeClient, _, err := c.K8sClientProvider.Provide(clientcmdapi.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.Context().Namespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, pipeline.DeployedStatus, cfg.Data["status"])
}

func getManifestWithError(_ string) (*model.Manifest, error) {
	return nil, assert.AnError
}

func getFakeManifest(_ string) (*model.Manifest, error) {
	return fakeManifest, nil
}

func getErrorManifest(_ string) (*model.Manifest, error) {
	return errorManifest, nil
}

func getManifestWithNoDeployNorDependency(_ string) (*model.Manifest, error) {
	return noDeployNorDependenciesManifest, nil
}

func getFakeManifestWithDependency(_ string) (*model.Manifest, error) {
	return fakeManifestWithDependency, nil
}

func TestBuildImages(t *testing.T) {
	testCases := []struct {
		name                 string
		builder              *fakeV2Builder
		build                bool
		buildServices        []string
		stack                *model.Stack
		servicesToDeploy     []string
		servicesAlreadyBuilt []string
		expectedError        error
		expectedImages       []string
	}{
		{
			name: "everything",
			builder: &fakeV2Builder{
				servicesAlreadyBuilt: []string{"manifest B", "stack A"},
			},
			build:         false,
			buildServices: []string{"manifest A", "manifest B", "stack A", "stack B"},
			stack: &model.Stack{Services: map[string]*model.Service{
				"stack A":             {Build: &model.BuildInfo{}},
				"stack B":             {Build: &model.BuildInfo{}},
				"stack without build": {},
			}},
			servicesToDeploy: []string{"stack A", "stack without build"},
			expectedError:    nil,
			expectedImages:   []string{"manifest A"},
		},
		{
			name:             "nil stack",
			builder:          &fakeV2Builder{},
			build:            false,
			buildServices:    []string{"manifest A", "manifest B"},
			stack:            nil,
			servicesToDeploy: []string{"manifest A"},
			expectedError:    nil,
			expectedImages:   []string{"manifest A", "manifest B"},
		},
		{
			name: "no services to deploy",
			builder: &fakeV2Builder{
				servicesAlreadyBuilt: []string{"stack"},
			},
			build:         false,
			buildServices: []string{"manifest", "stack"},
			stack: &model.Stack{Services: map[string]*model.Service{
				"stack": {Build: &model.BuildInfo{}},
			}},
			servicesToDeploy: []string{},
			expectedError:    nil,
			expectedImages:   []string{"manifest"},
		},
		{
			name:          "no services already built",
			builder:       &fakeV2Builder{},
			build:         false,
			buildServices: []string{"manifest A", "stack B", "stack C"},
			stack: &model.Stack{Services: map[string]*model.Service{
				"stack B": {Build: &model.BuildInfo{}},
				"stack C": {Build: &model.BuildInfo{}},
			}},
			servicesToDeploy: []string{"manifest A", "stack C"},
			expectedError:    nil,
			expectedImages:   []string{"manifest A", "stack C"},
		},
		{
			name: "force build",
			builder: &fakeV2Builder{
				servicesAlreadyBuilt: []string{"should be ignored since build is forced", "manifest A", "stack B"},
			},
			build:         true,
			buildServices: []string{"manifest A", "manifest B", "stack A", "stack B"},
			stack: &model.Stack{Services: map[string]*model.Service{
				"stack A": {Build: &model.BuildInfo{}},
				"stack B": {Build: &model.BuildInfo{}},
			}},
			servicesToDeploy: []string{"stack A", "stack B"},
			expectedError:    nil,
			expectedImages:   []string{"manifest A", "manifest B", "stack A", "stack B"},
		},
		{
			name: "force build specific services",
			builder: &fakeV2Builder{
				servicesAlreadyBuilt: []string{"should be ignored since build is forced", "manifest A", "stack B"},
			},
			build:         true,
			buildServices: []string{"manifest A", "manifest B", "stack A", "stack B"},
			stack: &model.Stack{Services: map[string]*model.Service{
				"stack A":             {Build: &model.BuildInfo{}},
				"stack B":             {Build: &model.BuildInfo{}},
				"stack without build": {},
			}},
			servicesToDeploy: []string{"stack A", "stack without build"},
			expectedError:    nil,
			expectedImages:   []string{"manifest A", "manifest B", "stack A"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			deployOptions := &Options{
				Build: testCase.build,
				Manifest: &model.Manifest{
					Build: model.ManifestBuild{},
					Deploy: &model.DeployInfo{
						ComposeSection: &model.ComposeSectionInfo{
							Stack: testCase.stack,
						},
					},
				},
				servicesToDeploy: testCase.servicesToDeploy,
			}

			for _, service := range testCase.buildServices {
				deployOptions.Manifest.Build[service] = &model.BuildInfo{}
			}

			err := buildImages(context.Background(), testCase.builder, deployOptions)
			assert.Equal(t, testCase.expectedError, err)
			assert.Equal(t, sliceToSet(testCase.expectedImages), sliceToSet(testCase.builder.buildOptionsStorage.CommandArgs))
		})
	}

}

type fakeExternalControl struct {
	externals []externalresource.ExternalResource
	err       error
}

type fakeExternalControlProvider struct {
	control ExternalResourceInterface
}

func (f *fakeExternalControl) Deploy(_ context.Context, _ string, _ string, _ *externalresource.ExternalResource) error {
	return f.err
}

func (f *fakeExternalControl) List(ctx context.Context, ns string, labelSelector string) ([]externalresource.ExternalResource, error) {
	return f.externals, f.err
}

func (f *fakeExternalControl) Validate(_ context.Context, _ string, _ string, _ *externalresource.ExternalResource) error {
	return f.err
}

func (f *fakeExternalControlProvider) getFakeExternalControl(_ *rest.Config) ExternalResourceInterface {
	return f.control
}

func getFakeEndpoint() (EndpointGetter, error) {
	return EndpointGetter{
		K8sClientProvider: test.NewFakeK8sProvider(),
		endpointControl:   &fakeExternalControl{},
	}, nil
}

func TestDeployExternals(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	testCases := []struct {
		name        string
		options     *Options
		expectedErr bool
		control     ExternalResourceInterface
	}{
		{
			name: "no externals to deploy",
			options: &Options{
				Manifest: &model.Manifest{
					Deploy:   &model.DeployInfo{},
					External: nil,
				},
			},
			control: &fakeExternalControl{},
		},
		{
			name: "deploy external",
			options: &Options{
				Manifest: &model.Manifest{
					Deploy: &model.DeployInfo{},
					External: externalresource.ExternalResourceSection{
						"test": &externalresource.ExternalResource{
							Icon: "myIcon",
							Notes: &externalresource.Notes{
								Path: "/some/path",
							},
							Endpoints: []*externalresource.ExternalEndpoint{},
						},
					},
				},
			},
			control: &fakeExternalControl{},
		},
		{
			name: "error when deploy external",
			options: &Options{
				Manifest: &model.Manifest{
					Deploy: &model.DeployInfo{},
					External: externalresource.ExternalResourceSection{
						"test": &externalresource.ExternalResource{
							Icon: "myIcon",
							Notes: &externalresource.Notes{
								Path: "/some/path",
							},
							Endpoints: []*externalresource.ExternalEndpoint{},
						},
					},
				},
			},
			control: &fakeExternalControl{
				err: assert.AnError,
			},
			expectedErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			cp := fakeExternalControlProvider{
				control: tc.control,
			}

			ld := localDeployer{
				ConfigMapHandler:   &fakeCmapHandler{},
				GetExternalControl: cp.getFakeExternalControl,
				Fs:                 afero.NewMemMapFs(),
				K8sClientProvider:  test.NewFakeK8sProvider(),
			}

			if tc.expectedErr {
				assert.Error(t, ld.runDeploySection(ctx, tc.options))
			} else {
				assert.NoError(t, ld.runDeploySection(ctx, tc.options))
			}
		})
	}
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
		Dependencies: model.ManifestDependencies{
			"a": &model.Dependency{
				Namespace: "b",
			},
			"b": &model.Dependency{},
		},
	}
	type config struct {
		pipelineErr error
	}
	tt := []struct {
		name     string
		config   config
		expected error
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
			dc := &DeployCommand{
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

	fakeExternalControlProvider := fakeExternalControlProvider{
		control: &fakeExternalControl{},
	}
	fakeDeployer := &fakeDeployer{
		proxy:                   &fakeProxy{},
		executor:                &fakeExecutor{},
		kubeconfig:              &fakeKubeConfig{},
		fs:                      fakeOs,
		k8sClientProvider:       fakeK8sClientProvider,
		externalControlProvider: fakeExternalControlProvider,
	}

	c := &DeployCommand{
		PipelineCMD:        fakePipelineDeployer{nil},
		GetManifest:        getFakeManifestWithDependency,
		K8sClientProvider:  fakeK8sClientProvider,
		GetExternalControl: fakeExternalControlProvider.getFakeExternalControl,
		Fs:                 fakeOs,
		CfgMapHandler:      newDefaultConfigMapHandler(fakeK8sClientProvider),
		GetDeployer:        fakeDeployer.Get,
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	tt := []struct {
		name        string
		expecterErr error
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
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Namespace: "test",
						IsOkteto:  tc.isOkteto,
					},
				},
				CurrentContext: "test",
			}

			err := c.RunDeploy(ctx, opts)

			require.ErrorIs(t, err, tc.expecterErr)
		})
	}
}

type fakeTracker struct{}

func (*fakeTracker) TrackDeploy(analytics.DeployMetadata)             {}
func (*fakeTracker) TrackImageBuild(...*analytics.ImageBuildMetadata) {}

func TestTrackDeploy(t *testing.T) {
	tt := []struct {
		name       string
		manifest   *model.Manifest
		remoteFlag bool
		commandErr error
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
			dc := &DeployCommand{
				AnalyticsTracker: &fakeTracker{},
			}

			dc.trackDeploy(tc.manifest, tc.remoteFlag, time.Now(), tc.commandErr)
		})
	}
}

func TestShouldRunInRemoteDeploy(t *testing.T) {
	var tempManifest *model.Manifest = &model.Manifest{
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
			remoteDeploy: "",
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
				RunInRemote: true,
			},
			remoteDeploy: "",
			remoteForce:  "",
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
			t.Setenv(constants.OktetoDeployRemote, string(tt.remoteDeploy))
			t.Setenv(constants.OktetoForceRemote, string(tt.remoteForce))
			result := shouldRunInRemote(tt.opts)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestOktetoManifestPathFlag(t *testing.T) {
	opts := &Options{}
	var tests = []struct {
		name        string
		manifest    string
		expectedErr error
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

func Test_GetDeployer(t *testing.T) {
	dnsErr := &net.DNSError{
		IsNotFound: true,
	}
	tests := []struct {
		name        string
		opts        *Options
		portGetter  func(string) (int, error)
		expectedErr error
		isUserErr   bool
	}{
		{
			name: "local deployer returns port user error",
			opts: &Options{},
			portGetter: func(_ string) (int, error) {
				return 0, dnsErr
			},
			expectedErr: dnsErr,
			isUserErr:   true,
		},
		{
			name: "local deployer returns other error",
			opts: &Options{},
			portGetter: func(_ string) (int, error) {
				return 0, assert.AnError
			},
			expectedErr: assert.AnError,
		},
		{
			name: "local deployer returned",
			opts: &Options{},
			portGetter: func(_ string) (int, error) {
				return 123456, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			got, err := GetDeployer(ctx, tt.opts, nil, &fakeCmapHandler{}, &fakeK8sProvider{}, &fakeKubeConfig{
				config: &rest.Config{},
			}, tt.portGetter)

			if tt.expectedErr == nil {
				require.NotNil(t, got)
			}

			require.ErrorIs(t, err, tt.expectedErr)
			_, ok := err.(oktetoErrors.UserError)
			require.True(t, tt.isUserErr == ok)
		})
	}
}
