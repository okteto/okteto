package destroy

import (
	"context"
	"fmt"
	"testing"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var fakeManifest *utils.Manifest = &utils.Manifest{
	Destroy: []string{
		"printenv",
		"ls -la",
		"cat /tmp/test.txt",
	},
}

type fakeDestroyer struct {
	destroyed bool
	err       error
}

type fakeSecretHandler struct {
	secrets []v1.Secret
	err     error
}

type fakeExecutor struct {
	err      error
	executed []string
}

func (fd *fakeDestroyer) DestroyWithLabel(ctx context.Context, ns, labelSelector string, destroyVolumes bool) error {
	if fd.err != nil {
		return fd.err
	}

	fd.destroyed = true
	return nil
}

func (fd *fakeSecretHandler) List(ctx context.Context, ns, labelSelector string) ([]v1.Secret, error) {
	if fd.err != nil {
		return nil, fd.err
	}

	return fd.secrets, nil
}

func (fe *fakeExecutor) Execute(command string, env []string) error {
	fe.executed = append(fe.executed, command)
	if fe.err != nil {
		return fe.err
	}

	return nil
}

func getManifestWithError(_, _, _ string) (*utils.Manifest, error) {
	return nil, assert.AnError
}

func getFakeManifest(_, _, _ string) (*utils.Manifest, error) {
	return fakeManifest, nil
}

func TestDestroyWithErrorListingSecrets(t *testing.T) {
	ctx := context.Background()
	cwd := "/okteto/src"
	secretHandler := fakeSecretHandler{
		err: assert.AnError,
	}
	tests := []struct {
		name        string
		getManifest func(cwd, name, filename string) (*utils.Manifest, error)
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
				getManifest: tt.getManifest,
				secrets:     &secretHandler,
				executor:    executor,
			}

			err := cmd.runDestroy(ctx, cwd, opts)

			assert.Error(t, err)
			assert.Equal(t, tt.want, len(executor.executed))
		})
	}
}

func TestDestroyWithError(t *testing.T) {
	ctx := context.Background()
	cwd := "/okteto/src"
	tests := []struct {
		name        string
		getManifest func(cwd, name, filename string) (*utils.Manifest, error)
		secrets     []v1.Secret
		want        []string
	}{
		{
			name:        "WithoutSecretsWithoutManifest",
			getManifest: getManifestWithError,
			secrets:     []v1.Secret{},
			want:        []string{},
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
			want: []string{},
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
			want: []string{
				fmt.Sprintf(helmUninstallCommand, "helm-app"),
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
			want: append(fakeManifest.Destroy, fmt.Sprintf(helmUninstallCommand, "helm-app")),
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
				getManifest: tt.getManifest,
				secrets:     &secretHandler,
				executor:    executor,
				nsDestroyer: destroyer,
			}

			err := cmd.runDestroy(ctx, cwd, opts)

			assert.Error(t, err)
			assert.ElementsMatch(t, tt.want, executor.executed)
			assert.False(t, destroyer.destroyed)
		})
	}
}

func TestDestroyWithoutError(t *testing.T) {
	ctx := context.Background()
	cwd := "/okteto/src"
	tests := []struct {
		name        string
		getManifest func(cwd, name, filename string) (*utils.Manifest, error)
		secrets     []v1.Secret
		want        []string
	}{
		{
			name:        "WithoutSecretsWithoutManifest",
			getManifest: getManifestWithError,
			secrets:     []v1.Secret{},
			want:        []string{},
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
			want: []string{},
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
			want: []string{
				fmt.Sprintf(helmUninstallCommand, "helm-app"),
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
			want: []string{
				fmt.Sprintf(helmUninstallCommand, "helm-app"),
				fmt.Sprintf(helmUninstallCommand, "another-helm-app"),
				fmt.Sprintf(helmUninstallCommand, "last-helm-app"),
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
			want: append(fakeManifest.Destroy, fmt.Sprintf(helmUninstallCommand, "helm-app")),
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
				fmt.Sprintf(helmUninstallCommand, "helm-app"),
				fmt.Sprintf(helmUninstallCommand, "another-helm-app"),
				fmt.Sprintf(helmUninstallCommand, "last-helm-app"),
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
				getManifest: tt.getManifest,
				secrets:     &secretHandler,
				executor:    executor,
				nsDestroyer: destroyer,
			}

			err := cmd.runDestroy(ctx, cwd, opts)

			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.want, executor.executed)
			assert.True(t, destroyer.destroyed)
		})
	}
}

func TestDestroyWithoutForceOptionAndFailedCommands(t *testing.T) {
	ctx := context.Background()
	cwd := "/okteto/src"
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
		getManifest: getFakeManifest,
		secrets:     &secretHandler,
		executor:    executor,
		nsDestroyer: destroyer,
	}

	err := cmd.runDestroy(ctx, cwd, opts)

	assert.Error(t, err)
	assert.Equal(t, 1, len(executor.executed))
	assert.False(t, destroyer.destroyed)
}

func TestDestroyWithForceOptionAndFailedCommands(t *testing.T) {
	ctx := context.Background()
	cwd := "/okteto/src"
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
		getManifest: getFakeManifest,
		secrets:     &secretHandler,
		executor:    executor,
		nsDestroyer: destroyer,
	}

	err := cmd.runDestroy(ctx, cwd, opts)

	assert.Error(t, err)
	assert.ElementsMatch(t, fakeManifest.Destroy, executor.executed)
	assert.True(t, destroyer.destroyed)
}
