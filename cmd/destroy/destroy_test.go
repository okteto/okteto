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

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd/api"
)

var fakeManifest *model.Manifest = &model.Manifest{
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

func (*fakeExecutor) CleanUp(_ error) {}

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

func TestDestroyWithErrorDeletingVolumes(t *testing.T) {
	ctx := context.Background()
	executor := &fakeExecutor{}
	opts := &Options{
		Name: "test-app",
	}
	k8sClientProvider := test.NewFakeK8sProvider()
	fakeClient, _, err := k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	destroyer := &fakeDestroyer{
		errOnVolumes: assert.AnError,
	}
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}

	ld := localDestroyCommand{
		&localDestroyAllCommand{
			ConfigMapHandler:  NewConfigmapHandler(fakeClient),
			nsDestroyer:       destroyer,
			executor:          executor,
			k8sClientProvider: k8sClientProvider,
		},
		fakeManifest,
	}

	err = ld.runDestroy(ctx, opts)

	assert.Error(t, err)
	assert.Equal(t, 3, len(executor.executed))
	assert.False(t, destroyer.destroyed)
	assert.False(t, destroyer.destroyedVolumes)

	// check if configmap has been created
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.GetContext().Namespace, fakeClient)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestDestroyWithErrorListingSecrets(t *testing.T) {
	ctx := context.Background()
	secretHandler := fakeSecretHandler{
		err: assert.AnError,
	}
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	tests := []struct {
		manifest *model.Manifest
		name     string
		want     int
	}{
		{
			name: "AndWithoutManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
			want: 0,
		},
		{
			name:     "AndWithManifest",
			manifest: fakeManifest,
			want:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &fakeExecutor{}
			opts := &Options{
				Name: "test-app",
			}
			k8sClientProvider := test.NewFakeK8sProvider()
			fakeClient, _, err := k8sClientProvider.Provide(api.NewConfig())
			if err != nil {
				t.Fatal("could not create fake k8s client")
			}

			ld := localDestroyCommand{
				&localDestroyAllCommand{
					ConfigMapHandler:  NewConfigmapHandler(fakeClient),
					nsDestroyer:       &fakeDestroyer{},
					executor:          executor,
					k8sClientProvider: k8sClientProvider,
					secrets:           &secretHandler,
				},
				tt.manifest,
			}

			err = ld.runDestroy(ctx, opts)

			assert.Error(t, err)
			assert.Equal(t, tt.want, len(executor.executed))

			// check if configmap has been created
			cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.GetContext().Namespace, fakeClient)
			assert.NoError(t, err)
			assert.NotNil(t, cfg)
		})
	}
}

func TestDestroyWithError(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	tests := []struct {
		name     string
		manifest *model.Manifest
		secrets  []v1.Secret
		want     []model.DeployCommand
	}{
		{
			name: "WithoutSecretsWithoutManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
			secrets: []v1.Secret{},
			want:    []model.DeployCommand{},
		},
		{
			name:     "WithoutSecretsWithManifest",
			manifest: fakeManifest,
			secrets:  []v1.Secret{},
			want:     fakeManifest.Destroy.Commands,
		},
		{
			name: "WithSecretsWithoutManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
			secrets: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secret-1",
					},
				},
			},
			want: []model.DeployCommand{},
		},
		{
			name:     "WithSecretsWithManifest",
			manifest: fakeManifest,
			secrets: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secret-1",
					},
				},
			},
			want: fakeManifest.Destroy.Commands,
		},
		{
			name: "WithHelmSecretsWithoutManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
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
			},
			want: []model.DeployCommand{
				{
					Name:    fmt.Sprintf(helmUninstallCommand, "helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "helm-app"),
				},
			},
		},
		{
			name:     "WithHelmSecretsWithManifest",
			manifest: fakeManifest,
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
			},
			want: append(fakeManifest.Destroy.Commands, model.DeployCommand{
				Name:    fmt.Sprintf(helmUninstallCommand, "helm-app"),
				Command: fmt.Sprintf(helmUninstallCommand, "helm-app"),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &fakeExecutor{}
			opts := &Options{
				Name: "test-app",
			}
			destroyer := &fakeDestroyer{
				err: assert.AnError,
			}
			secretHandler := fakeSecretHandler{
				secrets: tt.secrets,
			}
			k8sClientProvider := test.NewFakeK8sProvider()
			fakeClient, _, err := k8sClientProvider.Provide(api.NewConfig())
			if err != nil {
				t.Fatal("could not create fake k8s client")
			}

			ld := localDestroyCommand{
				&localDestroyAllCommand{
					ConfigMapHandler:  NewConfigmapHandler(fakeClient),
					nsDestroyer:       destroyer,
					executor:          executor,
					k8sClientProvider: k8sClientProvider,
					secrets:           &secretHandler,
				},
				tt.manifest,
			}

			err = ld.runDestroy(ctx, opts)

			assert.Error(t, err)
			assert.ElementsMatch(t, tt.want, executor.executed)
			assert.False(t, destroyer.destroyed)
			assert.True(t, destroyer.destroyedVolumes)

			// check if configmap has been created
			cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.GetContext().Namespace, fakeClient)
			assert.NoError(t, err)
			assert.NotNil(t, cfg)
		})
	}
}

func TestDestroyWithoutError(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	tests := []struct {
		name     string
		manifest *model.Manifest
		secrets  []v1.Secret
		want     []model.DeployCommand
	}{
		{
			name: "WithoutSecretsWithoutManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
			secrets: []v1.Secret{},
			want:    []model.DeployCommand{},
		},
		{
			name:     "WithoutSecretsWithManifest",
			manifest: fakeManifest,
			secrets:  []v1.Secret{},
			want:     fakeManifest.Destroy.Commands,
		},
		{
			name: "WithSecretsWithoutManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
			secrets: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secret-1",
					},
				},
			},
			want: []model.DeployCommand{},
		},
		{
			name:     "WithSecretsWithManifest",
			manifest: fakeManifest,
			secrets: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secret-1",
					},
				},
			},
			want: fakeManifest.Destroy.Commands,
		},
		{
			name: "WithHelmSecretsWithoutManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
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
			},
			want: []model.DeployCommand{
				{
					Name:    fmt.Sprintf(helmUninstallCommand, "helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "helm-app"),
				},
			},
		},
		{
			name: "WithSeveralHelmSecretsWithoutManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
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
							nameLabel:  "another-helm-app",
						},
					},
					Type: model.HelmSecretType,
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secret-3",
						Labels: map[string]string{
							ownerLabel: helmOwner,
							nameLabel:  "last-helm-app",
						},
					},
					Type: model.HelmSecretType,
				},
			},
			want: []model.DeployCommand{
				{
					Name:    fmt.Sprintf(helmUninstallCommand, "helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "helm-app"),
				},
				{
					Name:    fmt.Sprintf(helmUninstallCommand, "another-helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "another-helm-app"),
				},
				{
					Name:    fmt.Sprintf(helmUninstallCommand, "last-helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "last-helm-app"),
				},
			},
		},
		{
			name:     "WithHelmSecretsWithManifest",
			manifest: fakeManifest,
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
			},
			want: append(fakeManifest.Destroy.Commands, model.DeployCommand{Name: fmt.Sprintf(helmUninstallCommand, "helm-app"), Command: fmt.Sprintf(helmUninstallCommand, "helm-app")}),
		},
		{
			name:     "WithSeveralHelmSecretsWithManifest",
			manifest: fakeManifest,
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
							nameLabel:  "another-helm-app",
						},
					},
					Type: model.HelmSecretType,
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secret-3",
						Labels: map[string]string{
							ownerLabel: helmOwner,
							nameLabel:  "last-helm-app",
						},
					},
					Type: model.HelmSecretType,
				},
			},
			want: append(
				fakeManifest.Destroy.Commands,
				model.DeployCommand{
					Name:    fmt.Sprintf(helmUninstallCommand, "helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "helm-app"),
				},
				model.DeployCommand{
					Name:    fmt.Sprintf(helmUninstallCommand, "another-helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "another-helm-app"),
				},
				model.DeployCommand{
					Name:    fmt.Sprintf(helmUninstallCommand, "last-helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "last-helm-app"),
				},
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &fakeExecutor{}
			opts := &Options{
				Name: "test-app",
			}
			destroyer := &fakeDestroyer{}
			secretHandler := fakeSecretHandler{
				secrets: tt.secrets,
			}
			k8sClientProvider := test.NewFakeK8sProvider()
			fakeClient, _, err := k8sClientProvider.Provide(api.NewConfig())
			if err != nil {
				t.Fatal("could not create fake k8s client")
			}

			ld := localDestroyCommand{
				&localDestroyAllCommand{
					ConfigMapHandler:  NewConfigmapHandler(fakeClient),
					nsDestroyer:       destroyer,
					executor:          executor,
					k8sClientProvider: k8sClientProvider,
					secrets:           &secretHandler,
				},
				tt.manifest,
			}

			err = ld.runDestroy(ctx, opts)

			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.want, executor.executed)
			assert.True(t, destroyer.destroyed)
			assert.True(t, destroyer.destroyedVolumes)

			// check if configmap has been created
			cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.GetContext().Namespace, fakeClient)
			assert.Error(t, err)
			assert.Nil(t, cfg)
		})
	}
}

func TestDestroyWithoutErrorInsideOktetoDeploy(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	tests := []struct {
		name     string
		manifest *model.Manifest
		secrets  []v1.Secret
		want     []model.DeployCommand
	}{
		{
			name: "WithoutSecretsWithoutManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
			secrets: []v1.Secret{},
			want:    []model.DeployCommand{},
		},
		{
			name: "WithoutSecretsWithManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
			secrets: []v1.Secret{},
			want:    []model.DeployCommand{},
		},
		{
			name: "WithSecretsWithoutManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
			secrets: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secret-1",
					},
				},
			},
			want: []model.DeployCommand{},
		},
		{
			name: "WithSecretsWithManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
			secrets: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secret-1",
					},
				},
			},
			want: []model.DeployCommand{},
		},
		{
			name: "WithHelmSecretsWithoutManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
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
			},
			want: []model.DeployCommand{
				{
					Name:    fmt.Sprintf(helmUninstallCommand, "helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "helm-app"),
				},
			},
		},
		{
			name: "WithSeveralHelmSecretsWithoutManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
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
							nameLabel:  "another-helm-app",
						},
					},
					Type: model.HelmSecretType,
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secret-3",
						Labels: map[string]string{
							ownerLabel: helmOwner,
							nameLabel:  "last-helm-app",
						},
					},
					Type: model.HelmSecretType,
				},
			},
			want: []model.DeployCommand{
				{
					Name:    fmt.Sprintf(helmUninstallCommand, "helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "helm-app"),
				},
				{
					Name:    fmt.Sprintf(helmUninstallCommand, "another-helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "another-helm-app"),
				},
				{
					Name:    fmt.Sprintf(helmUninstallCommand, "last-helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "last-helm-app"),
				},
			},
		},
		{
			name: "WithHelmSecretsWithManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
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
			},
			want: append([]model.DeployCommand{}, model.DeployCommand{Name: fmt.Sprintf(helmUninstallCommand, "helm-app"), Command: fmt.Sprintf(helmUninstallCommand, "helm-app")}),
		},
		{
			name: "WithSeveralHelmSecretsWithManifest",
			manifest: &model.Manifest{
				Destroy: &model.DestroyInfo{},
			},
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
							nameLabel:  "another-helm-app",
						},
					},
					Type: model.HelmSecretType,
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secret-3",
						Labels: map[string]string{
							ownerLabel: helmOwner,
							nameLabel:  "last-helm-app",
						},
					},
					Type: model.HelmSecretType,
				},
			},
			want: append(
				[]model.DeployCommand{},
				model.DeployCommand{
					Name:    fmt.Sprintf(helmUninstallCommand, "helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "helm-app"),
				},
				model.DeployCommand{
					Name:    fmt.Sprintf(helmUninstallCommand, "another-helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "another-helm-app"),
				},
				model.DeployCommand{
					Name:    fmt.Sprintf(helmUninstallCommand, "last-helm-app"),
					Command: fmt.Sprintf(helmUninstallCommand, "last-helm-app"),
				},
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &fakeExecutor{}
			opts := &Options{
				Name: "test-app",
			}
			destroyer := &fakeDestroyer{}
			secretHandler := fakeSecretHandler{
				secrets: tt.secrets,
			}
			// Set env var destroy inside deploy
			t.Setenv(constants.OktetoWithinDeployCommandContextEnvVar, "true")

			ld := localDestroyCommand{
				&localDestroyAllCommand{
					ConfigMapHandler:  NewConfigmapHandler(nil),
					nsDestroyer:       destroyer,
					executor:          executor,
					k8sClientProvider: test.NewFakeK8sProvider(),
					secrets:           &secretHandler,
				},
				tt.manifest,
			}

			err := ld.runDestroy(ctx, opts)

			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.want, executor.executed)
			assert.True(t, destroyer.destroyed)
			assert.True(t, destroyer.destroyedVolumes)

			fakeClient, _, err := ld.k8sClientProvider.Provide(api.NewConfig())
			if err != nil {
				t.Fatal("could not create fake k8s client")
			}
			cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.GetContext().Namespace, fakeClient)
			assert.True(t, k8sErrors.IsNotFound(err))
			assert.Nil(t, cfg)
		})
	}
}

func TestDestroyWithoutForceOptionAndFailedCommands(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	executor := &fakeExecutor{
		err: assert.AnError,
	}
	opts := &Options{
		Name:         "test-app",
		ForceDestroy: false,
	}
	destroyer := &fakeDestroyer{}
	secretHandler := fakeSecretHandler{
		secrets: []v1.Secret{},
	}
	k8sClientProvider := test.NewFakeK8sProvider()
	fakeClient, _, err := k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}

	ld := localDestroyCommand{
		&localDestroyAllCommand{
			ConfigMapHandler:  NewConfigmapHandler(fakeClient),
			nsDestroyer:       destroyer,
			executor:          executor,
			k8sClientProvider: k8sClientProvider,
			secrets:           &secretHandler,
		},
		fakeManifest,
	}

	err = ld.runDestroy(ctx, opts)

	assert.Error(t, err)
	assert.Equal(t, 1, len(executor.executed))
	assert.False(t, destroyer.destroyed)
	assert.False(t, destroyer.destroyedVolumes)

	// check if configmap has been created
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.GetContext().Namespace, fakeClient)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestDestroyWithForceOptionAndFailedCommands(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	executor := &fakeExecutor{
		err: assert.AnError,
	}
	opts := &Options{
		Name:         "test-app",
		ForceDestroy: true,
	}
	destroyer := &fakeDestroyer{}
	secretHandler := fakeSecretHandler{
		secrets: []v1.Secret{},
	}
	k8sClientProvider := test.NewFakeK8sProvider()
	fakeClient, _, err := k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}

	ld := localDestroyCommand{
		&localDestroyAllCommand{
			ConfigMapHandler:  NewConfigmapHandler(fakeClient),
			nsDestroyer:       destroyer,
			executor:          executor,
			k8sClientProvider: k8sClientProvider,
			secrets:           &secretHandler,
		},
		fakeManifest,
	}

	err = ld.runDestroy(ctx, opts)

	assert.Error(t, err)
	assert.ElementsMatch(t, fakeManifest.Destroy.Commands, executor.executed)
	assert.True(t, destroyer.destroyed)
	assert.True(t, destroyer.destroyedVolumes)

	// check if configmap has been created
	cfg, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.GetContext().Namespace, fakeClient)
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestShouldRunInRemoteDestroy(t *testing.T) {
	var tempManifest *model.Manifest = &model.Manifest{
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
