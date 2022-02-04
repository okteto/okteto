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
	"fmt"
	"os"
	"testing"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/cmd/app"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

var fakeManifest *model.Manifest = &model.Manifest{
	Deploy: &model.DeployInfo{
		Commands: []string{
			"printenv",
			"ls -la",
			"cat /tmp/test.txt",
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
	executed []string
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

func (fk *fakeProxy) Start() {
	fk.started = true
}

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

func (fe *fakeExecutor) Execute(command string, _ []string) error {
	fe.executed = append(fe.executed, command)
	if fe.err != nil {
		return fe.err
	}

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
	c := &deployCommand{
		proxy:    p,
		executor: e,
		kubeconfig: &fakeKubeConfig{
			errOnModify: assert.AnError,
		},
		k8sClientProvider: test.NewFakeK8sProvider(),
	}
	ctx := context.Background()
	cwd := "/tmp"
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	err := c.runDeploy(ctx, cwd, opts)

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
	c := &deployCommand{
		getManifest:       getManifestWithError,
		proxy:             p,
		executor:          e,
		kubeconfig:        &fakeKubeConfig{},
		k8sClientProvider: test.NewFakeK8sProvider(),
	}
	ctx := context.Background()
	cwd := "/tmp"
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	err := c.runDeploy(ctx, cwd, opts)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, e.executed, 0)
	// Proxy wasn't started
	assert.False(t, p.started)
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
	c := &deployCommand{
		getManifest:       getFakeManifest,
		proxy:             p,
		executor:          e,
		kubeconfig:        &fakeKubeConfig{},
		k8sClientProvider: test.NewFakeK8sProvider(),
	}
	ctx := context.Background()
	cwd := "/tmp"
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	err := c.runDeploy(ctx, cwd, opts)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, e.executed, 1)
	// Check expected commands were executed
	assert.Equal(t, fakeManifest.Deploy.Commands[0], e.executed[0])
	// Proxy started
	assert.True(t, p.started)
	// Proxy shutdown
	assert.True(t, p.shutdown)

	//check if configmap has been created
	fakeClient, _, err := c.k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, app.TranslateAppName(opts.Name), okteto.Context().Namespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, app.ErrorStatus, cfg.Data["status"])
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
			Name:      app.TranslateAppName(opts.Name),
			Namespace: "test",
		},
		Data: map[string]string{
			"actionLock": "test",
		},
	}
	c := &deployCommand{
		getManifest:       getFakeManifest,
		proxy:             p,
		executor:          e,
		kubeconfig:        &fakeKubeConfig{},
		k8sClientProvider: test.NewFakeK8sProvider(cmap),
	}
	ctx := context.Background()
	cwd := "/tmp"

	err := c.runDeploy(ctx, cwd, opts)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, e.executed, 0)
	// Proxy started
	assert.True(t, p.started)

	//check if configmap has been created
	fakeClient, _, err := c.k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, app.TranslateAppName(opts.Name), okteto.Context().Namespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
}

func TestDeployWithErrorShuttingdownProxy(t *testing.T) {
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
	c := &deployCommand{
		getManifest:       getFakeManifest,
		proxy:             p,
		executor:          e,
		kubeconfig:        &fakeKubeConfig{},
		k8sClientProvider: test.NewFakeK8sProvider(),
	}
	ctx := context.Background()
	cwd := "/tmp"
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	err := c.runDeploy(ctx, cwd, opts)

	assert.NoError(t, err)
	// No command was executed
	assert.Len(t, e.executed, 3)
	// Check expected commands were executed
	assert.Equal(t, fakeManifest.Deploy.Commands, e.executed)
	// Proxy started
	assert.True(t, p.started)
	// Proxy wasn't shutdown
	assert.False(t, p.shutdown)

	//check if configmap has been created
	fakeClient, _, err := c.k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, app.TranslateAppName(opts.Name), okteto.Context().Namespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, app.DeployedStatus, cfg.Data["status"])
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
	c := &deployCommand{
		getManifest:       getFakeManifest,
		proxy:             p,
		executor:          e,
		kubeconfig:        &fakeKubeConfig{},
		k8sClientProvider: test.NewFakeK8sProvider(),
	}
	ctx := context.Background()
	cwd := "/tmp"
	opts := &Options{
		Name:         "movies",
		ManifestPath: "",
		Variables:    []string{},
	}

	err := c.runDeploy(ctx, cwd, opts)

	assert.NoError(t, err)
	// No command was executed
	assert.Len(t, e.executed, 3)
	// Check expected commands were executed
	assert.Equal(t, fakeManifest.Deploy.Commands, e.executed)
	// Proxy started
	assert.True(t, p.started)
	// Proxy was shutdown
	assert.True(t, p.shutdown)

	//check if configmap has been created
	fakeClient, _, err := c.k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, err := configmaps.Get(ctx, app.TranslateAppName(opts.Name), okteto.Context().Namespace, fakeClient)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, app.DeployedStatus, cfg.Data["status"])
}

func getManifestWithError(_ string, _ contextCMD.ManifestOptions) (*model.Manifest, error) {
	return nil, assert.AnError
}

func getFakeManifest(_ string, _ contextCMD.ManifestOptions) (*model.Manifest, error) {
	return fakeManifest, nil
}

func Test_setManifestEnvVars(t *testing.T) {
	tests := []struct {
		name          string
		service       string
		reference     string
		expRegistry   string
		expRepository string
		expImage      string
		expTag        string
	}{
		{
			name:          "setting-variables",
			service:       "frontend",
			reference:     "registry.url/namespace/frontend@sha256:7075f1094117e418764bb9b47a5dfc093466e714ec385223fb582d78220c7252",
			expRegistry:   "registry.url",
			expRepository: "namespace/frontend",
			expImage:      "registry.url/namespace/frontend@sha256:7075f1094117e418764bb9b47a5dfc093466e714ec385223fb582d78220c7252",
			expTag:        "sha256:7075f1094117e418764bb9b47a5dfc093466e714ec385223fb582d78220c7252",
		},
		{
			name:          "setting-variables-no-tag",
			service:       "frontend",
			reference:     "registry.url/namespace/frontend",
			expRegistry:   "registry.url",
			expRepository: "namespace/frontend",
			expImage:      "registry.url/namespace/frontend",
			expTag:        "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registryEnv := fmt.Sprintf("build.%s.registry", tt.service)
			imageEnv := fmt.Sprintf("build.%s.image", tt.service)
			repositoryEnv := fmt.Sprintf("build.%s.repository", tt.service)
			tagEnv := fmt.Sprintf("build.%s.tag", tt.service)

			envs := []string{registryEnv, imageEnv, repositoryEnv, tagEnv}
			for _, e := range envs {
				if err := os.Unsetenv(e); err != nil {
					t.Errorf("error unsetting var %s", err.Error())
				}
			}
			for _, e := range envs {
				if v := os.Getenv(e); v != "" {
					t.Errorf("env variable is already set [%v]", e)
				}
			}

			setManifestEnvVars(tt.service, tt.reference)

			registryEnvValue := os.Getenv(registryEnv)
			imageEnvValue := os.Getenv(imageEnv)
			repositoryEnvValue := os.Getenv(repositoryEnv)
			tagEnvValue := os.Getenv(tagEnv)

			if registryEnvValue != tt.expRegistry {
				t.Errorf("registry - expected %s , got %s", tt.expRegistry, registryEnvValue)
			}
			if imageEnvValue != tt.expImage {
				t.Errorf("image - expected %s , got %s", tt.expImage, imageEnvValue)

			}
			if repositoryEnvValue != tt.expRepository {
				t.Errorf("repository - expected %s , got %s", tt.expRepository, repositoryEnvValue)

			}
			if tagEnvValue != tt.expTag {
				t.Errorf("tag - expected %s , got %s", tt.expTag, tagEnvValue)

			}

		})
	}
}
