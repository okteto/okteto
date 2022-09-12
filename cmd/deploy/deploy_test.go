// Copyright 2022 The Okteto Authors
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
	"strings"
	"testing"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
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
	errOnModify error
}

func (*fakeKubeConfig) Read() (*rest.Config, error) {
	return nil, nil
}

func (fc *fakeKubeConfig) Modify(_ int, _, _ string) error {
	return fc.errOnModify
}
func (*fakeKubeConfig) GetModifiedCMDAPIConfig() (*clientcmdapi.Config, error) {
	return nil, nil
}

func (fk *fakeProxy) Start() {
	fk.started = true
}

func (fk *fakeProxy) SetName(_ string) {}

func (fk *fakeProxy) SetDivert(_ string) {}

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
	c := &DeployCommand{
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

	err := c.RunDeploy(ctx, opts)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, e.executed, 0)
	// Proxy wasn't started
	assert.False(t, p.started)
}

func TestDeployWithErrorReadingManifestFile(t *testing.T) {
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
	c := &DeployCommand{
		GetManifest:       getManifestWithError,
		Proxy:             p,
		Executor:          e,
		Kubeconfig:        &fakeKubeConfig{},
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
	assert.Len(t, e.executed, 0)
	// Proxy wasn't started
	assert.False(t, p.started)
}

func TestCreateConfigMapWithBuildError(t *testing.T) {
	p := &fakeProxy{}
	e := &fakeExecutor{
		err: assert.AnError,
	}
	opts := &Options{
		Name:         "testErr",
		ManifestPath: "",
		Variables:    []string{},
		Build:        true,
	}

	registry := test.NewFakeOktetoRegistry(nil)
	builder := test.NewFakeOktetoBuilder(registry)

	c := &DeployCommand{
		GetManifest:       getErrorManifest,
		Proxy:             p,
		Executor:          e,
		Kubeconfig:        &fakeKubeConfig{},
		K8sClientProvider: test.NewFakeK8sProvider(),
		Builder:           buildv2.NewBuilder(builder, registry),
	}

	ctx := context.Background()

	err := c.RunDeploy(ctx, opts)

	// we should get a build error because Dockerfile does not exist
	assert.Error(t, err)

	fakeClient, _, err := c.K8sClientProvider.Provide(clientcmdapi.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, _ := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.Context().Namespace, fakeClient)

	expectedCfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "okteto-git-testErr",
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

	assert.True(t, strings.Contains(oktetoLog.GetOutputBuffer().String(), errors.InvalidDockerfile))
	assert.Equal(t, expectedCfg, cfg)
}

func TestDeployWithErrorExecutingCommands(t *testing.T) {
	p := &fakeProxy{}
	e := &fakeExecutor{
		err: assert.AnError,
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
		Proxy:             p,
		Executor:          e,
		Kubeconfig:        &fakeKubeConfig{},
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
	assert.Len(t, e.executed, 1)
	// Check expected commands were executed
	assert.Equal(t, fakeManifest.Deploy.Commands[0], e.executed[0])
	// Proxy started
	assert.True(t, p.started)
	// Proxy shutdown
	assert.True(t, p.shutdown)

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
	p := &fakeProxy{
		errOnShutdown: assert.AnError,
	}
	e := &fakeExecutor{}
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}
	cmap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.TranslatePipelineName(opts.Name),
			Namespace: "test",
		},
		Data: map[string]string{
			"actionLock": "test",
		},
	}
	deployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				model.DeployedByLabel: "movies",
			},
		},
	}
	c := &DeployCommand{
		GetManifest:       getFakeManifest,
		Proxy:             p,
		Executor:          e,
		Kubeconfig:        &fakeKubeConfig{},
		K8sClientProvider: test.NewFakeK8sProvider(cmap, deployment),
	}
	ctx := context.Background()

	err := c.RunDeploy(ctx, opts)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, e.executed, 0)
	// Proxy started
	assert.True(t, p.started)

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
	p := &fakeProxy{
		errOnShutdown: assert.AnError,
	}
	deployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				model.DeployedByLabel: "movies",
			},
			Namespace: "test",
		},
	}
	e := &fakeExecutor{}
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
		Proxy:             p,
		Executor:          e,
		Kubeconfig:        &fakeKubeConfig{},
		K8sClientProvider: test.NewFakeK8sProvider(deployment),
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
	assert.Len(t, e.executed, 3)
	// Check expected commands were executed
	assert.Equal(t, fakeManifest.Deploy.Commands, e.executed)
	// Proxy started
	assert.True(t, p.started)
	// Proxy wasn't shutdown
	assert.False(t, p.shutdown)

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
	deployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				model.DeployedByLabel: "movies",
			},
			Namespace: "test",
		},
	}
	c := &DeployCommand{
		GetManifest:       getFakeManifest,
		Proxy:             p,
		Executor:          e,
		Kubeconfig:        &fakeKubeConfig{},
		K8sClientProvider: test.NewFakeK8sProvider(deployment),
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
	assert.Len(t, e.executed, 3)
	// Check expected commands were executed
	assert.Equal(t, fakeManifest.Deploy.Commands, e.executed)
	// Proxy started
	assert.True(t, p.started)
	// Proxy was shutdown
	assert.True(t, p.shutdown)

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

func TestBuildImages(t *testing.T) {
	testCases := []struct {
		name                 string
		build                bool
		buildServices        []string
		stack                *model.Stack
		servicesToDeploy     []string
		servicesAlreadyBuilt []string
		expectedError        error
		expectedImages       []string
	}{
		{
			name:          "everything",
			build:         false,
			buildServices: []string{"manifest A", "manifest B", "stack A", "stack B"},
			stack: &model.Stack{Services: map[string]*model.Service{
				"stack A": {Build: &model.BuildInfo{}},
				"stack B": {Build: &model.BuildInfo{}},
			}},
			servicesAlreadyBuilt: []string{"manifest B", "stack A"},
			servicesToDeploy:     []string{"stack A"},
			expectedError:        nil,
			expectedImages:       []string{"manifest A"},
		},
		{
			name:             "nil stack",
			build:            false,
			buildServices:    []string{"manifest A", "manifest B"},
			stack:            nil,
			servicesToDeploy: []string{"manifest A"},
			expectedError:    nil,
			expectedImages:   []string{"manifest A", "manifest B"},
		},
		{
			name:          "no services to deploy",
			build:         false,
			buildServices: []string{"manifest", "stack"},
			stack: &model.Stack{Services: map[string]*model.Service{
				"stack": {Build: &model.BuildInfo{}},
			}},
			servicesAlreadyBuilt: []string{"stack"},
			servicesToDeploy:     []string{},
			expectedError:        nil,
			expectedImages:       []string{"manifest"},
		},
		{
			name:          "no services already built",
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			buildOptionsStorage := &types.BuildOptions{}

			build := func(ctx context.Context, buildOptions *types.BuildOptions) error {
				buildOptionsStorage = buildOptions
				return nil
			}

			getServicesToBuild := func(ctx context.Context, manifest *model.Manifest, servicesToDeploy []string) ([]string, error) {
				toBuild := make(map[string]bool, len(manifest.Build))
				for service := range manifest.Build {
					toBuild[service] = true
				}

				return setToSlice(setDifference(setIntersection(toBuild, sliceToSet(servicesToDeploy)), sliceToSet(testCase.servicesAlreadyBuilt))), nil
			}

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

			err := buildImages(context.Background(), build, getServicesToBuild, deployOptions)
			assert.Equal(t, testCase.expectedError, err)
			assert.Equal(t, sliceToSet(testCase.expectedImages), sliceToSet(buildOptionsStorage.CommandArgs))
		})
	}

}
