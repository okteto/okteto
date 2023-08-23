package up

import (
	"context"
	"fmt"
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

func (f *fakeGetter) getEnvsFromDevContainer(ctx context.Context, spec *apiv1.PodSpec, name string, namespace string, client kubernetes.Interface) ([]string, error) {
	return f.envs, f.err
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
		name                      string
		expectedEnvs              []string
		dev                       *model.Dev
		client                    *fake.Clientset
		fakeDevContainerEnvGetter fakeGetter
		fakeConfigMapEnvsGetter   fakeGetter
		fakeSecretEnvsGetter      fakeGetter
		fakeImageEnvsGetter       fakeGetter
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
			dev: &model.Dev{
				Name:      "test",
				Namespace: "test",
			},
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
			dev: &model.Dev{
				Name:      "test",
				Namespace: "test",
			},
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
			dev: &model.Dev{
				Name:      "test",
				Namespace: "test",
			},
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
			fakeDevContainerEnvGetter: fakeGetter{envs: []string{"FROMPOD=VALUE1"}},
			fakeConfigMapEnvsGetter:   fakeGetter{},
			fakeSecretEnvsGetter:      fakeGetter{},
			fakeImageEnvsGetter:       fakeGetter{},
			expectedEnvs:              []string{"FROMPOD=VALUE1"},
			dev: &model.Dev{
				Name:      "test",
				Namespace: "test",
			},
		},
		{
			name: "only envs from environment section in manifest",
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
			fakeConfigMapEnvsGetter: fakeGetter{},
			fakeSecretEnvsGetter:    fakeGetter{},
			fakeImageEnvsGetter:     fakeGetter{},
			expectedEnvs:            []string{"FROMENVSECTION=VALUE1"},
			dev: &model.Dev{
				Name:      "test",
				Namespace: "test",
				Environment: model.Environment{
					model.EnvVar{
						Name:  "FROMENVSECTION",
						Value: "VALUE1",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eg := envsGetter{
				dev:                   tt.dev,
				name:                  "test",
				namespace:             "test",
				client:                tt.client,
				devContainerEnvGetter: &tt.fakeDevContainerEnvGetter,
				configMapEnvsGetter:   &tt.fakeConfigMapEnvsGetter,
				secretsEnvsGetter:     &tt.fakeSecretEnvsGetter,
				imageEnvsGetter:       &tt.fakeImageEnvsGetter,
				getDefaultLocalEnvs:   func() []string { return []string{} },
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
		name                      string
		client                    *fake.Clientset
		fakeDevContainerEnvGetter fakeGetter
		fakeConfigMapEnvsGetter   fakeGetter
		fakeSecretEnvsGetter      fakeGetter
		fakeImageEnvsGetter       fakeGetter
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

func TestGetEnvForHybridModeWithProperPriority(t *testing.T) {
	ctx := context.Background()

	fakeDevContainerEnvGetter := fakeGetter{envs: []string{"ENVFROMPOD=FROMPODVALUE"}}
	fakeConfigMapEnvsGetter := fakeGetter{envs: []string{"ENVFROMCONFIGMAP=FROMCONFIGMAPVALUE"}}
	fakeSecretEnvsGetter := fakeGetter{envs: []string{"ENVFROMSECRET=FROMSECRETVALUE"}}
	fakeImageEnvsGetter := fakeGetter{envs: []string{"ENVFROMIMAGE=FROMIMAGEVALUE"}}
	client := fake.NewSimpleClientset(&appsv1.StatefulSet{
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
									Name:  "ENVFROMPOD",
									Value: "FROMPODVALUE",
								},
								{
									Name: "SECRET_FROM_POD",
									ValueFrom: &v1.EnvVarSource{
										SecretKeyRef: &v1.SecretKeySelector{
											LocalObjectReference: v1.LocalObjectReference{
												Name: "",
											},
											Key:      "",
											Optional: nil,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	dev := &model.Dev{
		Name:      "test",
		Namespace: "test",
		Environment: model.Environment{
			model.EnvVar{
				Name:  "ENVFROMMANIFEST",
				Value: "FROMMANIFESTVALUE",
			},
		},
	}

	// according to exec.Cmd.Env docs, if cmd.Env contains duplicate environment keys, only the last
	// value in the slice for each duplicate key is used so most priority values needs to be add
	// at the end of the list
	expectedEnvsSortedByPriority := []string{
		"ENVFROMIMAGE=FROMIMAGEVALUE",
		"ENVFROMSECRET=FROMSECRETVALUE",
		"ENVFROMCONFIGMAP=FROMCONFIGMAPVALUE",
		"ENVFROMPOD=FROMPODVALUE",
		"ENVFROMMANIFEST=FROMMANIFESTVALUE",
	}

	eg := envsGetter{
		dev:                   dev,
		name:                  "test",
		namespace:             "test",
		client:                client,
		devContainerEnvGetter: &fakeDevContainerEnvGetter,
		configMapEnvsGetter:   &fakeConfigMapEnvsGetter,
		secretsEnvsGetter:     &fakeSecretEnvsGetter,
		imageEnvsGetter:       &fakeImageEnvsGetter,
		getDefaultLocalEnvs:   func() []string { return []string{} },
	}
	envs, err := eg.getEnvs(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedEnvsSortedByPriority, envs)
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

func TestGetEnvsFromDevContainer(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name         string
		expectedEnvs []string
		expectedErr  error
		podspec      *apiv1.PodSpec
		client       kubernetes.Interface
	}{
		{
			name: "dev container without env vars",
			podspec: &apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Env: []apiv1.EnvVar{},
					},
				},
			},
		},
		{
			name: "dev container with regular env var",
			podspec: &apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Env: []apiv1.EnvVar{
							{
								Name:  "FROMPOD",
								Value: "VALUE1",
							},
						},
					},
				},
			},
			expectedEnvs: []string{
				"FROMPOD=VALUE1",
			},
		},
		{
			name: "dev container with env var from secret",
			podspec: &apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Env: []apiv1.EnvVar{
							{
								Name: "SECRET_FROM_POD",
								ValueFrom: &apiv1.EnvVarSource{
									SecretKeyRef: &apiv1.SecretKeySelector{
										Key: "name-of-test-secret",
										LocalObjectReference: apiv1.LocalObjectReference{
											Name: "name-of-test-secret",
										},
									},
								},
							},
							{
								Name:  "FROMPOD",
								Value: "VALUE1",
							},
						},
					},
				},
			},
			client: fake.NewSimpleClientset(&apiv1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name-of-test-secret",
					Namespace: "ns-test",
				},
				Data: map[string][]byte{
					"name-of-test-secret": []byte("test"),
				},
			}),
			expectedEnvs: []string{
				"SECRET_FROM_POD=test",
				"FROMPOD=VALUE1",
			},
		},
		{
			name: "dev container with env var from secret (not found)",
			podspec: &apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Env: []apiv1.EnvVar{
							{
								Name:  "FROMPOD",
								Value: "VALUE1",
							},
							{
								Name: "SECRET_FROM_POD",
								ValueFrom: &apiv1.EnvVarSource{
									SecretKeyRef: &apiv1.SecretKeySelector{
										Key: "name-of-test-secret",
										LocalObjectReference: apiv1.LocalObjectReference{
											Name: "name-of-test-secret",
										},
									},
								},
							},
							{
								Name:  "FROMPOD2",
								Value: "VALUE2",
							},
						},
					},
				},
			},
			client: fake.NewSimpleClientset(),
			expectedEnvs: []string{
				"FROMPOD=VALUE1",
			},
			expectedErr: fmt.Errorf("error getting kubernetes secret: secrets \"name-of-test-secret\" not found: the development container didn't start successfully because the kubernetes secret 'name-of-test-secret' was not found"),
		},
		{
			name: "dev container with env var from ENV, secret, and configmap",
			podspec: &apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Env: []apiv1.EnvVar{
							{
								Name:  "FROMPOD",
								Value: "VALUE1",
							},
							{
								Name: "SECRET_FROM_POD",
								ValueFrom: &apiv1.EnvVarSource{
									SecretKeyRef: &apiv1.SecretKeySelector{
										Key: "name-of-test-secret",
										LocalObjectReference: apiv1.LocalObjectReference{
											Name: "name-of-test-secret",
										},
									},
								},
							},
							{
								Name: "FROM_CM",
								ValueFrom: &apiv1.EnvVarSource{
									ConfigMapKeyRef: &apiv1.ConfigMapKeySelector{
										Key: "name-of-test-env-from-cm",
										LocalObjectReference: apiv1.LocalObjectReference{
											Name: "name-of-test-env-from-cm",
										},
									},
								},
							},
						},
					},
				},
			},
			client: fake.NewSimpleClientset(&apiv1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name-of-test-secret",
					Namespace: "ns-test",
				},
				Data: map[string][]byte{
					"name-of-test-secret": []byte("test"),
				},
			}, &apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name-of-test-env-from-cm",
					Namespace: "ns-test",
				},
				Data: map[string]string{
					"name-of-test-env-from-cm": "test",
				},
			}),
			expectedEnvs: []string{
				"FROMPOD=VALUE1",
				"SECRET_FROM_POD=test",
				"FROM_CM=test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devContainerEnvGetter := devContainerEnvGetter{}
			envs, err := devContainerEnvGetter.getEnvsFromDevContainer(ctx, tt.podspec, "", "ns-test", tt.client)
			if err != nil {
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			}
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
