package up

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeGetter struct {
	envs []string
	err  error
}

func (f *fakeGetter) getEnvsFromConfigMap(ctx context.Context, name string, namespace string, client kubernetes.Interface) ([]string, error) {
	return f.envs, f.err
}

func (f *fakeGetter) getEnvsFromSecrets(context.Context) ([]string, error) {
	return f.envs, f.err
}

func (f *fakeGetter) getEnvsFromImage(string) ([]string, error) {
	return f.envs, f.err
}

func TestGetEnvs(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name                    string
		expectedEnvs            []string
		client                  *fake.Clientset
		fakeConfigMapEnvsGetter fakeGetter
		fakeSecretEnvsGetter    fakeGetter
		fakeImageEnvsGetter     fakeGetter
	}{
		{
			name:                    "only envs from config map",
			fakeConfigMapEnvsGetter: fakeGetter{envs: []string{"FROMCONFIGMAP=VALUE1"}},
			expectedEnvs:            []string{"FROMCONFIGMAP=VALUE1"},
			client: fake.NewSimpleClientset(&appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.StatefulSetSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Env: []v1.EnvVar{},
								},
							},
						},
					},
				},
			}),
		},
		{
			name:                    "only envs from secrets",
			fakeConfigMapEnvsGetter: fakeGetter{},
			fakeSecretEnvsGetter:    fakeGetter{envs: []string{"FROMSECRET=VALUE1"}},
			expectedEnvs:            []string{"FROMSECRET=VALUE1"},
			client: fake.NewSimpleClientset(&appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.StatefulSetSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Env: []v1.EnvVar{},
								},
							},
						},
					},
				},
			}),
		},
		{
			name: "only envs from image",
			client: fake.NewSimpleClientset(&appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.StatefulSetSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									VolumeMounts: []v1.VolumeMount{
										{
											MountPath: "/data",
										},
									},
								},
							},
						},
					},
				},
			}),
			fakeConfigMapEnvsGetter: fakeGetter{},
			fakeSecretEnvsGetter:    fakeGetter{},
			fakeImageEnvsGetter:     fakeGetter{envs: []string{"FROMIMAGE=VALUE1"}},
			expectedEnvs:            []string{"FROMIMAGE=VALUE1"},
		},
		{
			name: "only envs from pod",
			client: fake.NewSimpleClientset(&appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.StatefulSetSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Env: []v1.EnvVar{
										{
											Name:  "FROMPOD",
											Value: "VALUE1",
										},
									},
								},
							},
						},
					},
				},
			}),
			fakeConfigMapEnvsGetter: fakeGetter{},
			fakeSecretEnvsGetter:    fakeGetter{},
			fakeImageEnvsGetter:     fakeGetter{},
			expectedEnvs:            []string{"FROMPOD=VALUE1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eg := envsGetter{
				dev: &model.Dev{
					Name:      "test",
					Namespace: "test",
				},
				name:                "test",
				namespace:           "test",
				client:              tt.client,
				configMapEnvsGetter: &tt.fakeConfigMapEnvsGetter,
				secretsEnvsGetter:   &tt.fakeSecretEnvsGetter,
				imageEnvsGetter:     &tt.fakeImageEnvsGetter,
				getDefaultLocalEnvs: func() []string { return []string{} },
			}

			envs, err := eg.getEnvs(ctx)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expectedEnvs, envs)
		})
	}
}

func TestGetEnvsError(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name                    string
		client                  *fake.Clientset
		fakeConfigMapEnvsGetter fakeGetter
		fakeSecretEnvsGetter    fakeGetter
		fakeImageEnvsGetter     fakeGetter
	}{
		{
			name:                    "error retrieving envs from config map",
			fakeConfigMapEnvsGetter: fakeGetter{err: assert.AnError},
			client: fake.NewSimpleClientset(&appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.StatefulSetSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Env: []v1.EnvVar{},
								},
							},
						},
					},
				},
			}),
		},
		{
			name:                    "error retrieving envs from secrets",
			fakeConfigMapEnvsGetter: fakeGetter{},
			fakeSecretEnvsGetter:    fakeGetter{err: assert.AnError},
			client: fake.NewSimpleClientset(&appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.StatefulSetSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Env: []v1.EnvVar{},
								},
							},
						},
					},
				},
			}),
		},
		{
			name: "error retrieving envs from image",
			client: fake.NewSimpleClientset(&appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.StatefulSetSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									VolumeMounts: []v1.VolumeMount{
										{
											MountPath: "/data",
										},
									},
								},
							},
						},
					},
				},
			}),
			fakeConfigMapEnvsGetter: fakeGetter{},
			fakeSecretEnvsGetter:    fakeGetter{},
			fakeImageEnvsGetter:     fakeGetter{err: assert.AnError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eg := envsGetter{
				dev: &model.Dev{
					Name:      "test",
					Namespace: "test",
				},
				name:                "test",
				namespace:           "test",
				client:              tt.client,
				configMapEnvsGetter: &tt.fakeConfigMapEnvsGetter,
				secretsEnvsGetter:   &tt.fakeSecretEnvsGetter,
				imageEnvsGetter:     &tt.fakeImageEnvsGetter,
			}

			envs, err := eg.getEnvs(ctx)
			require.Error(t, err)
			require.Nil(t, envs)
		})
	}
}

type fakeImageGetter struct {
	imageMetadata registry.ImageMetadata
	err           error
}

func (fig *fakeImageGetter) GetImageMetadata(string) (registry.ImageMetadata, error) {
	return fig.imageMetadata, fig.err
}

func TestGetEnvsFromImage(t *testing.T) {

	tests := []struct {
		name          string
		expectedEnvs  []string
		imageMetadata registry.ImageMetadata
	}{
		{
			name: "envs in image",
			imageMetadata: registry.ImageMetadata{
				Envs: []string{
					"ONE=VALUE1",
					"TWO=VALUE2",
					"PATH=VALUE3",
				},
			},
			expectedEnvs: []string{
				"ONE=VALUE1",
				"TWO=VALUE2",
			},
		},
		{
			name:          "no envs in image",
			imageMetadata: registry.ImageMetadata{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imageEnvsGetter := imageEnvsGetter{
				imageGetter: &fakeImageGetter{
					imageMetadata: tt.imageMetadata,
				},
			}
			envs, err := imageEnvsGetter.getEnvsFromImage("")
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expectedEnvs, envs)
		})
	}
}

func TestGetEnvsFromImageError(t *testing.T) {
	imageEnvsGetter := imageEnvsGetter{
		imageGetter: &fakeImageGetter{
			err: assert.AnError,
		},
	}
	envs, err := imageEnvsGetter.getEnvsFromImage("")
	require.Error(t, err)
	require.Nil(t, envs)
}

func TestGetEnvsFromConfigMap(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name         string
		expectedEnvs []string
		client       kubernetes.Interface
	}{
		{
			name: "config map without envs",
			client: fake.NewSimpleClientset(&apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipeline.TranslatePipelineName("test"),
					Namespace: "test",
					Labels:    map[string]string{},
				},
			}),
		},
		{
			name: "config map with envs",
			client: fake.NewSimpleClientset(&apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipeline.TranslatePipelineName("test"),
					Namespace: "test",
					Labels:    map[string]string{},
				},
				Data: map[string]string{
					"variables": "W3siRlJPTUNNQVAiOiAiVkFMVUUxIiwgIkZST01DTUFQMiI6ICJWQUxVRTIifV0=",
				},
			}),
			expectedEnvs: []string{
				"FROMCMAP=VALUE1",
				"FROMCMAP2=VALUE2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMapGetter := configMapGetter{}
			envs, err := configMapGetter.getEnvsFromConfigMap(ctx, "test", "test", tt.client)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expectedEnvs, envs)
		})
	}
}

type fakeUserSecretsGetter struct {
	secrets []types.Secret
	err     error
}

func (fusg fakeUserSecretsGetter) GetUserSecrets(context.Context) ([]types.Secret, error) {
	return fusg.secrets, fusg.err
}

func TestGetEnvsFromSecrets(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		expectedEnvs      []string
		isOkteto          bool
		fakeSecretsGetter fakeUserSecretsGetter
	}{
		{
			name:     "okteto not active",
			isOkteto: false,
		},
		{
			name:              "no user secrets",
			isOkteto:          true,
			fakeSecretsGetter: fakeUserSecretsGetter{},
		},
		{
			name:     "with user secrets",
			isOkteto: true,
			fakeSecretsGetter: fakeUserSecretsGetter{
				secrets: []types.Secret{
					{
						Name:  "FROMSECRETSTORE",
						Value: "AVALUE",
					},
					{
						Name:  "FROMSECRETSTORE2",
						Value: "AVALUE2",
					},
				},
			},
			expectedEnvs: []string{
				"FROMSECRETSTORE=AVALUE",
				"FROMSECRETSTORE2=AVALUE2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Namespace: "test",
						IsOkteto:  tt.isOkteto,
					},
				},
				CurrentContext: "test",
			}
			secretEnvsGetter := secretsEnvsGetter{
				secretsGetter: tt.fakeSecretsGetter,
			}
			envs, err := secretEnvsGetter.getEnvsFromSecrets(ctx)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expectedEnvs, envs)
		})
	}
}

func TestGetEnvsFromSecretsError(t *testing.T) {
	secretEnvsGetter := secretsEnvsGetter{
		secretsGetter: fakeUserSecretsGetter{
			err: assert.AnError,
		},
	}
	envs, err := secretEnvsGetter.getEnvsFromSecrets(context.Background())
	require.Error(t, err)
	require.Nil(t, envs)
}
