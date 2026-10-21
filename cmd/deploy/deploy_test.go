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
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/build"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deps"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd/api"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var errorManifest = &model.Manifest{
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

func (fr fakeRegistry) Clone(from, to string) (string, error) {
	return from, nil
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

func (fr fakeRegistry) GetRegistryAndRepo(image string) (string, string)    { return "", "" }
func (fr fakeRegistry) GetRepoNameAndTag(repo string) (string, string)      { return "", "" }
func (fr fakeRegistry) GetDevImageFromGlobal(imageWithDigest string) string { return "" }

var fakeManifest = &model.Manifest{
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
		Divert: &model.DivertDeploy{
			Namespace: "staging",
		},
	},
}

var fakeManifestWithDependency = &model.Manifest{
	Dependencies: deps.ManifestSection{
		"a": &deps.Dependency{},
		"b": &deps.Dependency{},
	},
}

var noDeployNorDependenciesManifest = &model.Manifest{
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

	mock.Mock
}

func (b *fakeV2Builder) Build(ctx context.Context, buildOptions *types.BuildOptions) error {
	if b.buildErr != nil {
		return b.buildErr
	}
	b.buildOptionsStorage = buildOptions

	b.Called(ctx, buildOptions)
	return nil
}

func (b *fakeV2Builder) GetServicesToBuildDuringExecution(_ context.Context, manifest *model.Manifest, servicesToDeploy []string) ([]string, error) {
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
	conn buildCmd.BuildkitConnector,
) (Deployer, error) {
	args := f.Called(ctx, opts, buildEnvVarsGetter, cmapHandler, k8sProvider, ioCtrl, k8Logger, dependencyEnvVarsGetter, conn)
	return args.Get(0).(Deployer), args.Error(1)
}

func (f *fakeDeployer) Deploy(ctx context.Context, opts *Options) error {
	args := f.Called(ctx, opts)
	return args.Error(0)
}

func (f *fakeDeployer) CleanUp(ctx context.Context, err error) {
	f.Called(ctx, err)
}

type fakeDivert struct {
	mock.Mock
}

func (f *fakeDivert) Deploy(ctx context.Context) error {
	args := f.Called(ctx)
	return args.Error(0)
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
		AnalyticsTracker:  &fakeTracker{},
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
		AnalyticsTracker:  &fakeTracker{},
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

func TestCreateConfigMapWithBuildError(t *testing.T) {
	fakeNamespace := "test"
	fakeK8sClientProvider := test.NewFakeK8sProvider()
	opts := &Options{
		Name:         "testErr",
		Namespace:    fakeNamespace,
		ManifestPath: "",
		Variables:    []string{},
		NoBuild:      false,
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
		AnalyticsTracker:  &fakeTracker{},
		GetManifest:       getErrorManifest,
		Builder:           builderV2,
		K8sClientProvider: fakeK8sClientProvider,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		Fs:                afero.NewMemMapFs(),
		IoCtrl:            io.NewIOController(),
	}

	ctx := context.Background()

	err := c.Run(ctx, opts)

	// we should get a build error because Dockerfile does not exist
	assert.True(t, strings.Contains(err.Error(), oktetoErrors.ErrManifestPathNotFound.Error()))

	fakeClient, _, err := c.K8sClientProvider.ProvideWithLogger(clientcmdapi.NewConfig(), nil)
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}

	// sanitizeName is needed to check the CFGmap - this sanitization is done at RunDeploy, labels and cfg name
	sanitizedName := format.ResourceK8sMetaString(opts.Name)

	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(sanitizedName), fakeNamespace, fakeClient)
	assert.NoError(t, err)

	expectedCfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("okteto-git-%s", sanitizedName),
			Namespace: fakeNamespace,
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
	fakeNamespace := "test"
	fakeOs := afero.NewMemMapFs()
	fakeK8sClientProvider := test.NewFakeK8sProvider()
	fakeDeployer := &fakeDeployer{}
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: fakeNamespace,
				Cfg:       &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	c := &Command{
		AnalyticsTracker:  &fakeTracker{},
		GetManifest:       getFakeManifest,
		GetDeployer:       fakeDeployer.Get,
		K8sClientProvider: fakeK8sClientProvider,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		Fs:                fakeOs,
		Builder:           &fakeV2Builder{},
		IoCtrl:            io.NewIOController(),
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		Namespace:    fakeNamespace,
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
		mock.Anything,
	).Return(fakeDeployer, nil)

	expectedOpts := &Options{
		Name:         "movies",
		Namespace:    fakeNamespace,
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
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), fakeNamespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, pipeline.ErrorStatus, cfg.Data["status"])

	fakeDeployer.AssertExpectations(t)
}

func TestDeployWithErrorBecauseOtherPipelineRunning(t *testing.T) {
	fakeNamespace := "test"
	opts := &Options{
		Name:         "movies",
		Namespace:    fakeNamespace,
		ManifestPath: "",
		Variables:    []string{},
	}
	fakeK8sClientProvider := test.NewFakeK8sProvider(&apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.TranslatePipelineName(opts.Name),
			Namespace: fakeNamespace,
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
		AnalyticsTracker:  &fakeTracker{},
		GetManifest:       getFakeManifest,
		GetDeployer:       fakeDeployer.Get,
		K8sClientProvider: fakeK8sClientProvider,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		Fs:                afero.NewMemMapFs(),
		IoCtrl:            io.NewIOController(),
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
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), fakeNamespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
}

func TestDeployWithoutErrors(t *testing.T) {
	fakeNamespace := "test"
	fakeOs := afero.NewMemMapFs()
	fakeK8sClientProvider := test.NewFakeK8sProvider(&v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				model.DeployedByLabel: "movies",
			},
			Namespace: fakeNamespace,
		},
	})
	fakeDeployer := &fakeDeployer{}
	divert := &fakeDivert{}

	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: fakeNamespace,
				Cfg:       &api.Config{},
			},
		},
		CurrentContext: "test",
	}

	c := &Command{
		AnalyticsTracker:  &fakeTracker{},
		GetManifest:       getFakeManifest,
		K8sClientProvider: fakeK8sClientProvider,
		EndpointGetter:    getFakeEndpoint,
		Fs:                fakeOs,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		GetDeployer:       fakeDeployer.Get,
		Builder:           &fakeV2Builder{},
		IoCtrl:            io.NewIOController(),
		DivertDeployerGetter: func(_ *model.DivertDeploy, _, _ string, _ kubernetes.Interface, _ *io.Controller) (DivertDeployer, error) {
			return divert, nil
		},
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		Namespace:    fakeNamespace,
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
		mock.Anything,
	).Return(fakeDeployer, nil)

	expectedOpts := &Options{
		Name:         "movies",
		Namespace:    fakeNamespace,
		ManifestPath: "",
		Variables:    []string{},
		Manifest:     fakeManifest,
	}
	fakeDeployer.On("Deploy", mock.Anything, expectedOpts).Return(nil)
	divert.On("Deploy", mock.Anything).Return(nil)

	err := c.Run(ctx, opts)

	assert.NoError(t, err)

	// check if configmap has been created
	fakeClient, _, err := c.K8sClientProvider.ProvideWithLogger(clientcmdapi.NewConfig(), nil)
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), fakeNamespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, pipeline.DeployedStatus, cfg.Data["status"])
	divert.AssertExpectations(t)
}

func TestDeployWithErrorGettingDivertDriver(t *testing.T) {
	fakeNamespace := "test"
	fakeOs := afero.NewMemMapFs()
	fakeK8sClientProvider := test.NewFakeK8sProvider(&v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				model.DeployedByLabel: "movies",
			},
			Namespace: fakeNamespace,
		},
	})
	fakeDeployer := &fakeDeployer{}
	divert := &fakeDivert{}

	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: fakeNamespace,
				Cfg:       &api.Config{},
			},
		},
		CurrentContext: "test",
	}

	c := &Command{
		AnalyticsTracker:  &fakeTracker{},
		GetManifest:       getFakeManifest,
		K8sClientProvider: fakeK8sClientProvider,
		EndpointGetter:    getFakeEndpoint,
		Fs:                fakeOs,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		GetDeployer:       fakeDeployer.Get,
		Builder:           &fakeV2Builder{},
		IoCtrl:            io.NewIOController(),
		DivertDeployerGetter: func(_ *model.DivertDeploy, _, _ string, _ kubernetes.Interface, _ *io.Controller) (DivertDeployer, error) {
			return divert, fmt.Errorf("error getting divert driver")
		},
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		Namespace:    fakeNamespace,
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
		mock.Anything,
	).Return(fakeDeployer, nil)

	expectedOpts := &Options{
		Name:         "movies",
		Namespace:    fakeNamespace,
		ManifestPath: "",
		Variables:    []string{},
		Manifest:     fakeManifest,
	}
	fakeDeployer.On("Deploy", mock.Anything, expectedOpts).Return(nil)

	err := c.Run(ctx, opts)

	assert.Error(t, err)
	assert.Equal(t, "error getting divert driver", err.Error())

	// check if configmap has been created
	fakeClient, _, err := c.K8sClientProvider.ProvideWithLogger(clientcmdapi.NewConfig(), nil)
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), fakeNamespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, pipeline.ErrorStatus, cfg.Data["status"])
	divert.AssertNotCalled(t, "Deploy", mock.Anything)
}

func TestDeployWithErrorDeployingDivertDriver(t *testing.T) {
	fakeNamespace := "test"
	fakeOs := afero.NewMemMapFs()
	fakeK8sClientProvider := test.NewFakeK8sProvider(&v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				model.DeployedByLabel: "movies",
			},
			Namespace: fakeNamespace,
		},
	})
	fakeDeployer := &fakeDeployer{}
	divert := &fakeDivert{}

	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: fakeNamespace,
				Cfg:       &api.Config{},
			},
		},
		CurrentContext: "test",
	}

	c := &Command{
		AnalyticsTracker:  &fakeTracker{},
		GetManifest:       getFakeManifest,
		K8sClientProvider: fakeK8sClientProvider,
		EndpointGetter:    getFakeEndpoint,
		Fs:                fakeOs,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		GetDeployer:       fakeDeployer.Get,
		Builder:           &fakeV2Builder{},
		IoCtrl:            io.NewIOController(),
		DivertDeployerGetter: func(_ *model.DivertDeploy, _, _ string, _ kubernetes.Interface, _ *io.Controller) (DivertDeployer, error) {
			return divert, nil
		},
	}
	ctx := context.Background()
	opts := &Options{
		Name:         "movies",
		Namespace:    fakeNamespace,
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
		mock.Anything,
	).Return(fakeDeployer, nil)
	divert.On("Deploy", mock.Anything).Return(fmt.Errorf("error getting deploying driver"))

	expectedOpts := &Options{
		Name:         "movies",
		Namespace:    fakeNamespace,
		ManifestPath: "",
		Variables:    []string{},
		Manifest:     fakeManifest,
	}
	fakeDeployer.On("Deploy", mock.Anything, expectedOpts).Return(nil)

	err := c.Run(ctx, opts)

	assert.Error(t, err)
	assert.Equal(t, "error getting deploying driver", err.Error())

	// check if configmap has been created
	fakeClient, _, err := c.K8sClientProvider.ProvideWithLogger(clientcmdapi.NewConfig(), nil)
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), fakeNamespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, pipeline.ErrorStatus, cfg.Data["status"])
	divert.AssertExpectations(t)
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

// capturingPipelineDeployer records the options of every dependency deployed and returns the error configured for it.
// Dependencies run in parallel, so it is safe for concurrent use
type capturingPipelineDeployer struct {
	errs     map[string]error
	captured map[string]*pipelineCMD.DeployOptions
	mu       sync.Mutex
}

func (fd *capturingPipelineDeployer) ExecuteDeployPipeline(_ context.Context, opts *pipelineCMD.DeployOptions) error {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	if fd.captured == nil {
		fd.captured = map[string]*pipelineCMD.DeployOptions{}
	}
	fd.captured[opts.Name] = opts
	return fd.errs[opts.Name]
}

func TestStartDependenciesNotAvailableInVanilla(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts:       map[string]*okteto.Context{"test": {Namespace: "test", Cfg: &api.Config{}}},
		CurrentContext: "test",
	}
	dc := &Command{PipelineCMD: &capturingPipelineDeployer{}}

	_, err := dc.startDependencies(context.Background(), &Options{Manifest: fakeManifestWithDependency})

	require.ErrorIs(t, err, errDepenNotAvailableInVanilla)
}

func TestStartDependenciesSharesParentWorkflowID(t *testing.T) {
	setTestOktetoContext()
	deployer := &capturingPipelineDeployer{}
	dc := &Command{PipelineCMD: deployer}

	scheduler, err := dc.startDependencies(context.Background(), &Options{Manifest: fakeManifestWithDependency})
	require.NoError(t, err)
	require.NoError(t, scheduler.Wait())

	require.Len(t, deployer.captured, 2)
	require.NotEmpty(t, deployer.captured["a"].ParentWorkflowID)
	require.Equal(t, deployer.captured["a"].ParentWorkflowID, deployer.captured["b"].ParentWorkflowID)
	require.True(t, deployer.captured["a"].IsDependency)
	require.True(t, deployer.captured["b"].IsDependency)
}

func TestExportDependencyEnvVars(t *testing.T) {
	setTestOktetoContext()
	t.Setenv("OKTETO_DEPENDENCY_DB_VARIABLE_PASSWORD", "")
	cmap := dependencyConfigMap(t, "db", map[string]string{"PASSWORD": "secret"})
	dc := &Command{CfgMapHandler: newDefaultConfigMapHandler(test.NewFakeK8sProvider(cmap), nil)}

	err := dc.exportDependencyEnvVars(context.Background(), []string{"db"}, "test")

	require.NoError(t, err)
	require.Equal(t, "secret", os.Getenv("OKTETO_DEPENDENCY_DB_VARIABLE_PASSWORD"))
}

func TestExportDependencyEnvVarsFailsWhenEnvVarsCannotBeRead(t *testing.T) {
	setTestOktetoContext()
	dc := &Command{CfgMapHandler: newDefaultConfigMapHandler(test.NewFakeK8sProvider(), nil)}

	err := dc.exportDependencyEnvVars(context.Background(), []string{"db"}, "test")

	require.ErrorContains(t, err, "could not get environment variables generated by dependency 'db'")
}

// newRunWithDependenciesCommand returns a Command whose manifest has deploy commands and the given dependencies.
// Every dependency has its configmap, exposing the variable DEP_VAR=<name>, as if it had been deployed
func newRunWithDependenciesCommand(t *testing.T, section deps.ManifestSection, pipelineDeployer *capturingPipelineDeployer, deployer *fakeDeployer) *Command {
	t.Helper()
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {Namespace: "test", IsOkteto: true, Cfg: &api.Config{}},
		},
		CurrentContext: "test",
	}
	manifest := &model.Manifest{
		Deploy:       &model.DeployInfo{Commands: []model.DeployCommand{{Name: "echo", Command: "echo"}}},
		Dependencies: section,
	}
	objects := []runtime.Object{&v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    map[string]string{model.DeployedByLabel: "movies"},
			Namespace: "test",
		},
	}}
	for name := range section {
		objects = append(objects, dependencyConfigMap(t, name, map[string]string{"DEP_VAR": name}))
	}
	k8sProvider := test.NewFakeK8sProvider(objects...)
	return &Command{
		AnalyticsTracker:  &fakeTracker{},
		PipelineCMD:       pipelineDeployer,
		GetManifest:       func(string, afero.Fs) (*model.Manifest, error) { return manifest, nil },
		K8sClientProvider: k8sProvider,
		EndpointGetter:    getFakeEndpoint,
		Fs:                afero.NewMemMapFs(),
		CfgMapHandler:     newDefaultConfigMapHandler(k8sProvider, nil),
		GetDeployer:       deployer.Get,
		Builder:           &fakeV2Builder{},
		IoCtrl:            io.NewIOController(),
	}
}

// resetLogBuffer starts the test with an empty deploy output
func resetLogBuffer(t *testing.T) {
	t.Helper()
	oktetoLog.Init(logrus.WarnLevel)
	t.Cleanup(func() { oktetoLog.Init(logrus.WarnLevel) })
}

func requireDeployStatus(t *testing.T, c *Command, expected string) {
	t.Helper()
	fakeClient, _, err := c.K8sClientProvider.ProvideWithLogger(clientcmdapi.NewConfig(), nil)
	require.NoError(t, err)
	cfg, err := configmaps.Get(context.Background(), pipeline.TranslatePipelineName("movies"), "test", fakeClient)
	require.NoError(t, err)
	require.Equal(t, expected, cfg.Data["status"])
}

func TestRunDependencyFailureSkipsMainDeploy(t *testing.T) {
	pipelineDeployer := &capturingPipelineDeployer{errs: map[string]error{"db": assert.AnError}}
	// no expectations: the main deploy must not run
	deployer := &fakeDeployer{}
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"db": &deps.Dependency{Repository: "https://github.com/okteto/db", Wait: true},
	}, pipelineDeployer, deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.ErrorContains(t, err, "dependency 'db' failed")
	requireDeployStatus(t, c, pipeline.ErrorStatus)
	deployer.AssertNotCalled(t, "Get")
}

func TestRunDependencyFailureIsReportedUnderTheDependencyStage(t *testing.T) {
	t.Cleanup(func() { oktetoLog.SetStage("") })
	pipelineDeployer := &capturingPipelineDeployer{errs: map[string]error{"messaging": assert.AnError}}
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"db":        &deps.Dependency{Repository: "https://github.com/okteto/db", Wait: true},
		"messaging": &deps.Dependency{Repository: "https://github.com/okteto/messaging", Wait: true},
	}, pipelineDeployer, &fakeDeployer{})

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.Error(t, err)
	require.Contains(t, deployOutput(t, c), `"level":"error","stage":"Deploying dependency messaging","message":"dependency 'messaging' failed`)
}

// deployOutput returns the log of the deploy as stored in its configmap, which is what the UI shows
func deployOutput(t *testing.T, c *Command) string {
	t.Helper()
	fakeClient, _, err := c.K8sClientProvider.ProvideWithLogger(clientcmdapi.NewConfig(), nil)
	require.NoError(t, err)
	cfg, err := configmaps.Get(context.Background(), pipeline.TranslatePipelineName("movies"), "test", fakeClient)
	require.NoError(t, err)
	output, err := base64.StdEncoding.DecodeString(cfg.Data["output"])
	require.NoError(t, err)
	return string(output)
}

func TestRunNotAwaitedDependencyFailureFailsTheDeploy(t *testing.T) {
	pipelineDeployer := &capturingPipelineDeployer{errs: map[string]error{"reports": assert.AnError}}
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(nil)
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, pipelineDeployer, deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.ErrorContains(t, err, "dependency 'reports' failed")
	requireDeployStatus(t, c, pipeline.ErrorStatus)
	deployer.AssertCalled(t, "Deploy", mock.Anything, mock.Anything)
}

func TestRunNotAwaitedDependencyFailureDoesNotBlockTheMainDeploy(t *testing.T) {
	t.Setenv("OKTETO_DEPENDENCY_DB_VARIABLE_DEP_VAR", "")
	pipelineDeployer := &capturingPipelineDeployer{errs: map[string]error{"reports": assert.AnError}}
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(nil)
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"db":      &deps.Dependency{Repository: "https://github.com/okteto/db", Wait: true},
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, pipelineDeployer, deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.ErrorContains(t, err, "dependency 'reports' failed")
	requireDeployStatus(t, c, pipeline.ErrorStatus)
	deployer.AssertCalled(t, "Deploy", mock.Anything, mock.Anything)
}

func TestRunWithDependenciesSucceeds(t *testing.T) {
	t.Setenv("OKTETO_DEPENDENCY_DB_VARIABLE_DEP_VAR", "")
	t.Setenv("OKTETO_DEPENDENCY_REPORTS_VARIABLE_DEP_VAR", "")
	pipelineDeployer := &capturingPipelineDeployer{}
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(nil)
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"db":      &deps.Dependency{Repository: "https://github.com/okteto/db", Wait: true},
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports", DependsOn: deps.DependsOn{"db"}},
	}, pipelineDeployer, deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.NoError(t, err)
	requireDeployStatus(t, c, pipeline.DeployedStatus)
	require.Len(t, pipelineDeployer.captured, 2)
	// db is a predecessor of the main deploy: its variables are exported. reports is not awaited
	require.Equal(t, "db", os.Getenv("OKTETO_DEPENDENCY_DB_VARIABLE_DEP_VAR"))
	require.Empty(t, os.Getenv("OKTETO_DEPENDENCY_REPORTS_VARIABLE_DEP_VAR"))
}

func TestRunRecordsEveryFailedDependencyUnderItsStage(t *testing.T) {
	pipelineDeployer := &capturingPipelineDeployer{errs: map[string]error{
		"db":      errors.New("db exploded"),
		"reports": errors.New("reports exploded"),
	}}
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"db":      &deps.Dependency{Repository: "https://github.com/okteto/db", Wait: true},
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, pipelineDeployer, &fakeDeployer{})

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.Error(t, err)
	output := deployOutput(t, c)
	require.Contains(t, output, `"level":"error","stage":"Deploying dependency db","message":"dependency 'db' failed: db exploded"`)
	require.Contains(t, output, `"level":"error","stage":"Deploying dependency reports","message":"dependency 'reports' failed: reports exploded"`)
}

func TestRunRecordsFailedDependenciesWhenTheMainDeployFails(t *testing.T) {
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(errors.New("main exploded"))
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, &capturingPipelineDeployer{errs: map[string]error{"reports": errors.New("reports exploded")}}, deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.ErrorContains(t, err, "main exploded")
	require.Contains(t, deployOutput(t, c), `"level":"error","stage":"Deploying dependency reports","message":"dependency 'reports' failed: reports exploded"`)
}

// lateLoggingPipelineDeployer deploys a dependency once the main deploy has failed, logging a line under its stage
type lateLoggingPipelineDeployer struct {
	mainFailed chan struct{}
}

func (d *lateLoggingPipelineDeployer) ExecuteDeployPipeline(_ context.Context, opts *pipelineCMD.DeployOptions) error {
	<-d.mainFailed
	oktetoLog.PrintlnWithStage(pipelineCMD.DependencyStage(opts.Name), "late line")
	return nil
}

// lastLineIndex returns the index of the last line that contains text, or -1
func lastLineIndex(lines []string, text string) int {
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], text) {
			return i
		}
	}
	return -1
}

func TestRunMainDeployFailureEndsTheOutputUnderItsStage(t *testing.T) {
	mainFailed := make(chan struct{})
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Run(func(mock.Arguments) {
		oktetoLog.SetStage("echo")
		close(mainFailed)
	}).Return(errors.New("main exploded"))
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, &capturingPipelineDeployer{}, deployer)
	c.PipelineCMD = &lateLoggingPipelineDeployer{mainFailed: mainFailed}

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.ErrorContains(t, err, "main exploded")
	lines := strings.Split(strings.TrimSpace(deployOutput(t, c)), "\n")
	end := lastLineIndex(lines, `"stage":"done","message":"EOF"`)
	// the line the dependency logged after the main deploy failed is not left after the end
	require.Less(t, lastLineIndex(lines, `"message":"late line"`), end)
	// and the last stage before the end is the one of the main deploy, with its error
	require.Contains(t, lines[end-1], `"level":"error","stage":"echo","message":"main exploded"`)
}

func TestRunMainDeployFailureWaitsForDependencies(t *testing.T) {
	pipelineDeployer := &capturingPipelineDeployer{errs: map[string]error{"reports": errors.New("reports failed")}}
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(errors.New("main failed"))
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, pipelineDeployer, deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.ErrorContains(t, err, "main failed")
	require.ErrorContains(t, err, "dependency 'reports' failed: reports failed")
	requireDeployStatus(t, c, pipeline.ErrorStatus)
}

func TestRunDependencyFailureAfterMainDeployHasNoEOFMarker(t *testing.T) {
	resetLogBuffer(t)
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(nil)
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, &capturingPipelineDeployer{errs: map[string]error{"reports": assert.AnError}}, deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.Error(t, err)
	// as when a dependency fails before the main deploy: the error is the last line, under the dependency stage
	output := deployOutput(t, c)
	require.NotContains(t, output, `"message":"EOF"`)
	require.Contains(t, output, `"level":"error","stage":"Deploying dependency reports"`)
}

func TestRunMainDeploySuccessHasEOFMarker(t *testing.T) {
	resetLogBuffer(t)
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(nil)
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, &capturingPipelineDeployer{}, deployer)

	require.NoError(t, c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"}))

	require.Contains(t, deployOutput(t, c), `"message":"EOF"`)
}

func TestRunWithDependenciesRecordsTheInterruptionOnCleanUp(t *testing.T) {
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(nil)
	deployer.On("CleanUp", mock.Anything, mock.Anything).Return()
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, &capturingPipelineDeployer{}, deployer)
	require.NoError(t, c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"}))

	// what the deploy command runs on Ctrl+C, without waiting for Run
	c.cleanUp(context.Background(), oktetoErrors.ErrIntSig)

	requireDeployStatus(t, c, pipeline.ErrorStatus)
}

// capturingBuilder records the output mode of the build
type capturingBuilder struct {
	outputMode string
}

func (b *capturingBuilder) Build(_ context.Context, options *types.BuildOptions) error {
	b.outputMode = options.OutputMode
	return nil
}

func (*capturingBuilder) GetServicesToBuildDuringExecution(_ context.Context, _ *model.Manifest, svcsToDeploy []string) ([]string, error) {
	return svcsToDeploy, nil
}

func (*capturingBuilder) GetBuildEnvVars() map[string]string { return nil }

// withBuildSection makes the manifest of c build an image, keeping its deploy and dependencies
func withBuildSection(c *Command) {
	getManifest := c.GetManifest
	c.GetManifest = func(path string, fs afero.Fs) (*model.Manifest, error) {
		m, err := getManifest(path, fs)
		if err != nil {
			return nil, err
		}
		m.Build = build.ManifestBuild{"api": &build.Info{Context: "api"}}
		return m, nil
	}
}

func TestRunWithDependenciesForcesPlainBuildProgress(t *testing.T) {
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(nil)
	builder := &capturingBuilder{}
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, &capturingPipelineDeployer{}, deployer)
	c.Builder = builder
	withBuildSection(c)
	opts := &Options{Name: "movies", Namespace: "test"}

	require.NoError(t, c.Run(context.Background(), opts))

	// reports runs at the same time as the build: its logs would break the TTY progress
	require.Equal(t, oktetoLog.PlainFormat, builder.outputMode)
}

func TestRunWithAwaitedDependenciesKeepsBuildProgress(t *testing.T) {
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(nil)
	builder := &capturingBuilder{}
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"db": &deps.Dependency{Repository: "https://github.com/okteto/db", Wait: true},
	}, &capturingPipelineDeployer{}, deployer)
	c.Builder = builder
	withBuildSection(c)

	require.NoError(t, c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"}))

	require.Empty(t, builder.outputMode)
}

func TestRunMainDeployInterruptedReturnsNil(t *testing.T) {
	pipelineDeployer := &capturingPipelineDeployer{}
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(oktetoErrors.ErrIntSig)
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, pipelineDeployer, deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.NoError(t, err)
	// the dependency was waited for before returning
	require.Len(t, pipelineDeployer.captured, 1)
}

func TestRunMainDeployErrorIsUserErrorWhenDependenciesSucceed(t *testing.T) {
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(errors.New("main failed"))
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, &capturingPipelineDeployer{}, deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.IsType(t, oktetoErrors.UserError{}, err)
}

func TestRunMainDeployErrorIsUserErrorWhenDependenciesFail(t *testing.T) {
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.Anything).Return(errors.New("main failed"))
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}, &capturingPipelineDeployer{errs: map[string]error{"reports": assert.AnError}}, deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.ErrorAs(t, err, &oktetoErrors.UserError{})
}

func TestRunOnlyDependenciesFailureSetsErrorStatus(t *testing.T) {
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"db": &deps.Dependency{Repository: "https://github.com/okteto/db"},
	}, &capturingPipelineDeployer{errs: map[string]error{"db": assert.AnError}}, &fakeDeployer{})
	manifest := &model.Manifest{Dependencies: deps.ManifestSection{
		"db": &deps.Dependency{Repository: "https://github.com/okteto/db"},
	}}
	c.GetManifest = func(string, afero.Fs) (*model.Manifest, error) { return manifest, nil }

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.ErrorContains(t, err, "dependency 'db' failed")
	requireDeployStatus(t, c, pipeline.ErrorStatus)
}

func TestRunExportFailureWaitsForDependencies(t *testing.T) {
	deployer := &fakeDeployer{}
	c := newRunWithDependenciesCommand(t, deps.ManifestSection{
		"db": &deps.Dependency{Repository: "https://github.com/okteto/db", Wait: true},
	}, &capturingPipelineDeployer{}, deployer)
	// the dependency configmap is missing, so its env vars can't be exported
	k8sProvider := test.NewFakeK8sProvider(&v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{model.DeployedByLabel: "movies"}, Namespace: "test"},
	})
	c.K8sClientProvider = k8sProvider
	c.CfgMapHandler = newDefaultConfigMapHandler(k8sProvider, nil)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.ErrorContains(t, err, "could not get environment variables generated by dependency 'db'")
	requireDeployStatus(t, c, pipeline.ErrorStatus)
	deployer.AssertNotCalled(t, "Get")
	require.Contains(t, deployOutput(t, c), `"level":"error","stage":"Deploying dependency db","message":"could not get environment variables generated by dependency 'db'`)
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
		AnalyticsTracker:  &fakeTracker{},
		PipelineCMD:       &capturingPipelineDeployer{},
		GetManifest:       getFakeManifestWithDependency,
		K8sClientProvider: fakeK8sClientProvider,
		Fs:                fakeOs,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		GetDeployer:       fakeDeployer.Get,
		IoCtrl:            io.NewIOController(),
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
func (*fakeTracker) TrackBuildkitConnection(*analytics.BuildkitConnectorMetadata)   {}
func (*fakeTracker) TrackDeployStarted(analytics.DeployStartedMetadata)             {}
func (*fakeTracker) TrackDeploy(analytics.DeployMetadata)                           {}
func (*fakeTracker) TrackDeployPipelineTriggered(context.Context, analytics.DeployPipelineTriggeredMetadata) {
}

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

			dc.TrackDeploy(tc.manifest, tc.remoteFlag, time.Now(), tc.commandErr, "test-ns")
		})
	}
}

type capturingDeployTracker struct {
	started   analytics.DeployStartedMetadata
	completed analytics.DeployMetadata
}

func (*capturingDeployTracker) TrackImageBuild(context.Context, *analytics.ImageBuildMetadata) {}
func (*capturingDeployTracker) TrackBuildkitConnection(*analytics.BuildkitConnectorMetadata)   {}
func (c *capturingDeployTracker) TrackDeployStarted(m analytics.DeployStartedMetadata)         { c.started = m }
func (c *capturingDeployTracker) TrackDeploy(m analytics.DeployMetadata)                       { c.completed = m }
func (*capturingDeployTracker) TrackDeployPipelineTriggered(context.Context, analytics.DeployPipelineTriggeredMetadata) {
}

func newWorkflowIDTestCommand(t *testing.T) (*Command, *capturingDeployTracker, *Options) {
	t.Helper()
	fakeNamespace := "test"
	fakeK8sClientProvider := test.NewFakeK8sProvider(&v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    map[string]string{model.DeployedByLabel: "movies"},
			Namespace: fakeNamespace,
		},
	})
	fakeDeployer := &fakeDeployer{}
	divert := &fakeDivert{}
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts:       map[string]*okteto.Context{"test": {Namespace: fakeNamespace, Cfg: &api.Config{}}},
		CurrentContext: "test",
	}
	tracker := &capturingDeployTracker{}
	c := &Command{
		AnalyticsTracker:  tracker,
		GetManifest:       getFakeManifest,
		K8sClientProvider: fakeK8sClientProvider,
		EndpointGetter:    getFakeEndpoint,
		Fs:                afero.NewMemMapFs(),
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sClientProvider, nil),
		GetDeployer:       fakeDeployer.Get,
		Builder:           &fakeV2Builder{},
		IoCtrl:            io.NewIOController(),
		DivertDeployerGetter: func(_ *model.DivertDeploy, _, _ string, _ kubernetes.Interface, _ *io.Controller) (DivertDeployer, error) {
			return divert, nil
		},
	}
	fakeDeployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fakeDeployer, nil)
	fakeDeployer.On("Deploy", mock.Anything, mock.Anything).Return(nil)
	divert.On("Deploy", mock.Anything).Return(nil)
	opts := &Options{Name: "movies", Namespace: fakeNamespace, Variables: []string{}}
	return c, tracker, opts
}

func TestDeployWorkflowID_GeneratedAndSharedWhenEnvUnset(t *testing.T) {
	t.Setenv(constants.OktetoWorkflowIDEnvVar, "")
	c, tracker, opts := newWorkflowIDTestCommand(t)

	require.NoError(t, c.Run(context.Background(), opts))
	require.NotEmpty(t, tracker.started.WorkflowID)

	c.TrackDeploy(opts.Manifest, false, time.Now(), nil, opts.Namespace)
	require.Equal(t, tracker.started.WorkflowID, tracker.completed.WorkflowID)
}

func TestDeployWorkflowID_UsesEnvVarWhenSet(t *testing.T) {
	t.Setenv(constants.OktetoWorkflowIDEnvVar, "wf-from-env")
	c, tracker, opts := newWorkflowIDTestCommand(t)

	require.NoError(t, c.Run(context.Background(), opts))
	require.Equal(t, "wf-from-env", tracker.started.WorkflowID)

	c.TrackDeploy(opts.Manifest, false, time.Now(), nil, opts.Namespace)
	require.Equal(t, "wf-from-env", tracker.completed.WorkflowID)
}

func TestManifestSyntax(t *testing.T) {
	tt := []struct {
		manifest *model.Manifest
		name     string
		expected string
	}{
		{
			name:     "nil manifest",
			manifest: nil,
			expected: "",
		},
		{
			name:     "nil deploy section",
			manifest: &model.Manifest{},
			expected: "",
		},
		{
			name: "commands only",
			manifest: &model.Manifest{Deploy: &model.DeployInfo{
				Commands: []model.DeployCommand{{Name: "x", Command: "x"}},
			}},
			expected: "",
		},
		{
			name: "compose only",
			manifest: &model.Manifest{Deploy: &model.DeployInfo{
				ComposeSection: &model.ComposeSectionInfo{ComposesInfo: model.ComposeInfoList{}},
			}},
			expected: "compose",
		},
		{
			name: "mixed: commands and compose",
			manifest: &model.Manifest{Deploy: &model.DeployInfo{
				Commands:       []model.DeployCommand{{Name: "x", Command: "x"}},
				ComposeSection: &model.ComposeSectionInfo{ComposesInfo: model.ComposeInfoList{}},
			}},
			expected: "mixed",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, manifestSyntax(tc.manifest))
		})
	}
}

func TestShouldRunInRemoteDeploy(t *testing.T) {
	tempManifest := &model.Manifest{
		Deploy: &model.DeployInfo{
			Remote: boolPointer(true),
			Image:  "some-image",
		},
	}
	var tests = []struct {
		Name        string
		opts        *Options
		remoteForce string
		expected    bool
	}{
		{
			Name: "Cluster default=local and --remote=true",
			opts: &Options{
				RunInRemoteSet: true,
				RunInRemote:    true,
			},
			remoteForce: "",
			expected:    true,
		},
		{
			Name: "Cluster default=local and --remote=false",
			opts: &Options{
				RunInRemoteSet: true,
				RunInRemote:    false,
			},
			remoteForce: "",
			expected:    false,
		},
		{
			Name: "Cluster default=local and deploy.remote=true + deploy.image set",
			opts: &Options{
				Manifest: tempManifest,
			},
			remoteForce: "",
			expected:    true,
		},
		{
			Name: "Cluster default=local and deploy.remote=true + deploy.image not set",
			opts: &Options{
				Manifest: &model.Manifest{
					Deploy: &model.DeployInfo{
						Image:  "",
						Remote: boolPointer(true),
					},
				},
			},
			remoteForce: "",
			expected:    true,
		},
		{
			Name: "Cluster default=local and deploy.remote=false + deploy.image not set",
			opts: &Options{
				Manifest: &model.Manifest{
					Deploy: &model.DeployInfo{
						Image:  "",
						Remote: boolPointer(false),
					},
				},
			},
			remoteForce: "",
			expected:    false,
		},
		{
			Name: "Cluster default=local and deploy.remote=nil + deploy.image set should run remote",
			opts: &Options{
				Manifest: &model.Manifest{
					Deploy: &model.DeployInfo{
						Image: "image",
					},
				},
			},
			remoteForce: "",
			expected:    true,
		},
		{
			Name: "Cluster default=remote, no flag or manifest set",
			opts: &Options{
				Manifest: &model.Manifest{
					Deploy: &model.DeployInfo{
						Commands: []model.DeployCommand{
							{
								Name:    "test",
								Command: "test",
							},
						},
					},
				},
			},
			remoteForce: "true",
			expected:    true,
		},
		{
			Name: "Cluster default=remote, --remote=false",
			opts: &Options{
				RunInRemoteSet: true,
				RunInRemote:    false,
			},
			remoteForce: "true",
			expected:    false,
		},
		{
			Name: "Cluster default=remote, manifest.deploy.remote=false",
			opts: &Options{
				Manifest: &model.Manifest{
					Deploy: &model.DeployInfo{
						Remote: boolPointer(false),
					},
				},
			},
			remoteForce: "true",
			expected:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Setenv(constants.OktetoForceRemote, tt.remoteForce)
			result := ShouldRunInRemote(tt.opts)
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
			expectedErr: oktetoErrors.ErrManifestPathNotFound,
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

func TestCalculateManifestPathToBeStored(t *testing.T) {
	dc := &Command{
		IoCtrl: io.NewIOController(),
	}

	wd, err := os.Getwd()
	require.NoError(t, err)

	tempDir := t.TempDir()

	tests := []struct {
		name           string
		topLevelGitDir string
		manifestPath   string
		expected       string
	}{
		{
			name:           "manifest path in same directory as git directory",
			topLevelGitDir: wd,
			manifestPath:   filepath.Join(wd, "okteto.yml"),
			expected:       "okteto.yml",
		},
		{
			name:           "manifest path within a subdirectory of git directory",
			topLevelGitDir: wd,
			manifestPath:   filepath.Join(wd, "subdir", "okteto.yml"),
			expected:       filepath.Join("subdir", "okteto.yml"),
		},
		{
			name:           "manifest path within a deeper subdirectory of git directory",
			topLevelGitDir: wd,
			manifestPath:   filepath.Join(wd, "subdir", "subdir2", "okteto.yml"),
			expected:       filepath.Join("subdir", "subdir2", "okteto.yml"),
		},
		{
			name:           "manifest path from git repo's parent directory",
			topLevelGitDir: wd,
			manifestPath:   filepath.Join(wd, "..", "okteto.yml"),
			expected:       "",
		},
		{
			name:           "absolute manifest path within repository",
			topLevelGitDir: wd,
			manifestPath:   filepath.Join(wd, "subdir", "okteto.yml"),
			expected:       filepath.Join("subdir", "okteto.yml"),
		},
		{
			name:           "manifest path completely different from git directory",
			topLevelGitDir: wd,
			manifestPath:   filepath.Join(tempDir, "okteto.yml"),
			expected:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dc.calculateManifestPathToBeStored(tt.topLevelGitDir, tt.manifestPath)

			require.Equal(t, tt.expected, result)
		})
	}
}

func boolPointer(v bool) *bool {
	return &v
}

func Test_resolveIsRedeploy_EnvVarTakesPrecedence(t *testing.T) {
	ctx := context.Background()
	namespace := "test"
	name := "movies"

	// A configmap in "deployed" status is present so IsDeployed would return true. The env var must
	// take precedence over it, proving the fallback is not used when the variable is set.
	deployedCfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.TranslatePipelineName(name),
			Namespace: namespace,
		},
		Data: map[string]string{
			"status": pipeline.DeployedStatus,
		},
	}

	var tests = []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "env var true",
			envValue: "true",
			expected: true,
		},
		{
			name:     "env var false overrides deployed configmap",
			envValue: "false",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(constants.OktetoIsRedeployEnvVar, tt.envValue)
			c := fake.NewSimpleClientset(deployedCfg)
			result := resolveIsRedeploy(ctx, name, namespace, c)
			require.Equal(t, tt.expected, result)
		})
	}
}

func Test_resolveIsRedeploy_FallbackToConfigmap(t *testing.T) {
	ctx := context.Background()
	namespace := "test"
	name := "movies"

	deployedCfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.TranslatePipelineName(name),
			Namespace: namespace,
		},
		Data: map[string]string{
			"status": pipeline.DeployedStatus,
		},
	}

	var tests = []struct {
		name     string
		objects  []runtime.Object
		expected bool
	}{
		{
			name:     "no configmap - first deploy",
			objects:  nil,
			expected: false,
		},
		{
			name:     "deployed configmap - redeploy",
			objects:  []runtime.Object{deployedCfg},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewSimpleClientset(tt.objects...)
			result := resolveIsRedeploy(ctx, name, namespace, c)
			require.Equal(t, tt.expected, result)
		})
	}
}
