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

package destroy

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd/api"
)

var fakeManifest *model.Manifest = &model.Manifest{
	Destroy: []model.DeployCommand{
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
}

type fakeDestroyer struct {
	destroyed        bool
	destroyedVolumes bool
	err              error
	errOnVolumes     error
}

type fakeSecretHandler struct {
	secrets []v1.Secret
	err     error
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
func getManifestWithError(_ string) (*model.Manifest, error) {
	return nil, assert.AnError
}

func getFakeManifest(_ string) (*model.Manifest, error) {
	return fakeManifest, nil
}

func TestMain(m *testing.M) {
	okteto.CurrentStore = &okteto.OktetoContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.OktetoContext{
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
	destroyer := &fakeDestroyer{
		errOnVolumes: assert.AnError,
	}
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	cmd := &destroyCommand{
		getManifest:       getFakeManifest,
		nsDestroyer:       destroyer,
		executor:          executor,
		k8sClientProvider: test.NewFakeK8sProvider(),
	}

	err := cmd.runDestroy(ctx, opts)

	assert.Error(t, err)
	assert.Equal(t, 3, len(executor.executed))
	assert.False(t, destroyer.destroyed)
	assert.False(t, destroyer.destroyedVolumes)

	//check if configmap has been created
	fakeClient, _, err := cmd.k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, _ := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.Context().Namespace, fakeClient)
	assert.NotNil(t, cfg)
}

func TestDestroyWithErrorListingSecrets(t *testing.T) {
	ctx := context.Background()
	secretHandler := fakeSecretHandler{
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
	tests := []struct {
		name        string
		getManifest func(path string) (*model.Manifest, error)
		want        int
	}{
		{
			name:        "AndWithoutManifest",
			getManifest: getManifestWithError,
			want:        0,
		},
		{
			name:        "AndWithManifest",
			getManifest: getFakeManifest,
			want:        3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &fakeExecutor{}
			opts := &Options{
				Name: "test-app",
			}
			cmd := &destroyCommand{
				getManifest:       tt.getManifest,
				secrets:           &secretHandler,
				nsDestroyer:       &fakeDestroyer{},
				executor:          executor,
				k8sClientProvider: test.NewFakeK8sProvider(),
			}

			err := cmd.runDestroy(ctx, opts)

			assert.Error(t, err)
			assert.Equal(t, tt.want, len(executor.executed))

			//check if configmap has been created
			fakeClient, _, err := cmd.k8sClientProvider.Provide(api.NewConfig())
			if err != nil {
				t.Fatal("could not create fake k8s client")
			}
			cfg, _ := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.Context().Namespace, fakeClient)
			assert.NotNil(t, cfg)
		})
	}
}

func TestDestroyWithError(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	tests := []struct {
		name        string
		getManifest func(path string) (*model.Manifest, error)
		secrets     []v1.Secret
		want        []model.DeployCommand
	}{
		{
			name:        "WithoutSecretsWithoutManifest",
			getManifest: getManifestWithError,
			secrets:     []v1.Secret{},
			want:        []model.DeployCommand{},
		},
		{
			name:        "WithoutSecretsWithManifest",
			getManifest: getFakeManifest,
			secrets:     []v1.Secret{},
			want:        fakeManifest.Destroy,
		},
		{
			name:        "WithSecretsWithoutManifest",
			getManifest: getManifestWithError,
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
			name:        "WithSecretsWithManifest",
			getManifest: getFakeManifest,
			secrets: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secret-1",
					},
				},
			},
			want: fakeManifest.Destroy,
		},
		{
			name:        "WithHelmSecretsWithoutManifest",
			getManifest: getManifestWithError,
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
			name:        "WithHelmSecretsWithManifest",
			getManifest: getFakeManifest,
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
			want: append(fakeManifest.Destroy, model.DeployCommand{
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
			cmd := &destroyCommand{
				getManifest:       tt.getManifest,
				secrets:           &secretHandler,
				executor:          executor,
				nsDestroyer:       destroyer,
				k8sClientProvider: test.NewFakeK8sProvider(),
			}

			err := cmd.runDestroy(ctx, opts)

			assert.Error(t, err)
			assert.ElementsMatch(t, tt.want, executor.executed)
			assert.False(t, destroyer.destroyed)
			assert.True(t, destroyer.destroyedVolumes)

			//check if configmap has been created
			fakeClient, _, err := cmd.k8sClientProvider.Provide(api.NewConfig())
			if err != nil {
				t.Fatal("could not create fake k8s client")
			}
			cfg, _ := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.Context().Namespace, fakeClient)
			assert.NotNil(t, cfg)
		})
	}
}

func TestDestroyWithoutError(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	tests := []struct {
		name        string
		getManifest func(path string) (*model.Manifest, error)
		secrets     []v1.Secret
		want        []model.DeployCommand
	}{
		{
			name:        "WithoutSecretsWithoutManifest",
			getManifest: getManifestWithError,
			secrets:     []v1.Secret{},
			want:        []model.DeployCommand{},
		},
		{
			name:        "WithoutSecretsWithManifest",
			getManifest: getFakeManifest,
			secrets:     []v1.Secret{},
			want:        fakeManifest.Destroy,
		},
		{
			name:        "WithSecretsWithoutManifest",
			getManifest: getManifestWithError,
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
			name:        "WithSecretsWithManifest",
			getManifest: getFakeManifest,
			secrets: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secret-1",
					},
				},
			},
			want: fakeManifest.Destroy,
		},
		{
			name:        "WithHelmSecretsWithoutManifest",
			getManifest: getManifestWithError,
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
			name:        "WithSeveralHelmSecretsWithoutManifest",
			getManifest: getManifestWithError,
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
			name:        "WithHelmSecretsWithManifest",
			getManifest: getFakeManifest,
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
			want: append(fakeManifest.Destroy, model.DeployCommand{Name: fmt.Sprintf(helmUninstallCommand, "helm-app"), Command: fmt.Sprintf(helmUninstallCommand, "helm-app")}),
		},
		{
			name:        "WithSeveralHelmSecretsWithManifest",
			getManifest: getFakeManifest,
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
				fakeManifest.Destroy,
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
			cmd := &destroyCommand{
				getManifest:       tt.getManifest,
				secrets:           &secretHandler,
				executor:          executor,
				nsDestroyer:       destroyer,
				k8sClientProvider: test.NewFakeK8sProvider(),
			}

			err := cmd.runDestroy(ctx, opts)

			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.want, executor.executed)
			assert.True(t, destroyer.destroyed)
			assert.True(t, destroyer.destroyedVolumes)

			//check if configmap has been created
			fakeClient, _, err := cmd.k8sClientProvider.Provide(api.NewConfig())
			if err != nil {
				t.Fatal("could not create fake k8s client")
			}
			cfg, _ := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.Context().Namespace, fakeClient)
			assert.Nil(t, cfg)
		})
	}
}

func TestDestroyWithoutForceOptionAndFailedCommands(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
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
	cmd := &destroyCommand{
		getManifest:       getFakeManifest,
		secrets:           &secretHandler,
		executor:          executor,
		nsDestroyer:       destroyer,
		k8sClientProvider: test.NewFakeK8sProvider(),
	}

	err := cmd.runDestroy(ctx, opts)

	assert.Error(t, err)
	assert.Equal(t, 1, len(executor.executed))
	assert.False(t, destroyer.destroyed)
	assert.False(t, destroyer.destroyedVolumes)

	//check if configmap has been created
	fakeClient, _, err := cmd.k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, _ := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.Context().Namespace, fakeClient)
	assert.NotNil(t, cfg)
}

func TestDestroyWithForceOptionAndFailedCommands(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
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
	cmd := &destroyCommand{
		getManifest:       getFakeManifest,
		secrets:           &secretHandler,
		executor:          executor,
		nsDestroyer:       destroyer,
		k8sClientProvider: test.NewFakeK8sProvider(),
	}

	err := cmd.runDestroy(ctx, opts)

	assert.Error(t, err)
	assert.ElementsMatch(t, fakeManifest.Destroy, executor.executed)
	assert.True(t, destroyer.destroyed)
	assert.True(t, destroyer.destroyedVolumes)

	//check if configmap has been created
	fakeClient, _, err := cmd.k8sClientProvider.Provide(api.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}
	cfg, _ := configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.Context().Namespace, fakeClient)
	assert.Nil(t, cfg)
}
