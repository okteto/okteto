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

package up

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeGetter struct {
	err  error
	envs []string
}

func (f *fakeGetter) getEnvsFromDevContainer(ctx context.Context, spec *apiv1.PodSpec, name, namespace string, client kubernetes.Interface) ([]string, error) {
	return f.envs, f.err
}

func (f *fakeGetter) getEnvsFromConfigMap(ctx context.Context, name, namespace string, client kubernetes.Interface) ([]string, error) {
	return f.envs, f.err
}

func (f *fakeGetter) getEnvsFromPlatformVariables(context.Context) ([]string, error) {
	return f.envs, f.err
}

func (f *fakeGetter) getEnvsFromImage(string) ([]string, error) {
	return f.envs, f.err
}

func TestGetEnvs(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		fakeDevContainerEnvGetter fakeGetter
		fakeConfigMapEnvsGetter   fakeGetter
		fakeSecretEnvsGetter      fakeGetter
		fakeImageEnvsGetter       fakeGetter
		dev                       *model.Dev
		client                    *fake.Clientset
		name                      string
		expectedEnvs              []string
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
				Environment: env.Environment{
					env.Var{
						Name:  "FROMENVSECTION",
						Value: "VALUE1",
					},
				},
			},
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
			expectedEnvs:            []string{},
			dev: &model.Dev{
				Name:      "test",
				Namespace: "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eg := envsGetter{
				dev:                         tt.dev,
				name:                        "test",
				namespace:                   "test",
				client:                      tt.client,
				devContainerEnvGetter:       &tt.fakeDevContainerEnvGetter,
				configMapEnvsGetter:         &tt.fakeConfigMapEnvsGetter,
				platformVariablesEnvsGetter: &tt.fakeSecretEnvsGetter,
				imageEnvsGetter:             &tt.fakeImageEnvsGetter,
				getDefaultLocalEnvs:         func() []string { return []string{} },
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
		client                    *fake.Clientset
		name                      string
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eg := envsGetter{
				dev: &model.Dev{
					Name:      "test",
					Namespace: "test",
				},
				name:                        "test",
				namespace:                   "test",
				client:                      tt.client,
				configMapEnvsGetter:         &tt.fakeConfigMapEnvsGetter,
				platformVariablesEnvsGetter: &tt.fakeSecretEnvsGetter,
				imageEnvsGetter:             &tt.fakeImageEnvsGetter,
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
		Environment: env.Environment{
			env.Var{
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
		dev:                         dev,
		name:                        "test",
		namespace:                   "test",
		client:                      client,
		devContainerEnvGetter:       &fakeDevContainerEnvGetter,
		configMapEnvsGetter:         &fakeConfigMapEnvsGetter,
		platformVariablesEnvsGetter: &fakeSecretEnvsGetter,
		imageEnvsGetter:             &fakeImageEnvsGetter,
		getDefaultLocalEnvs:         func() []string { return []string{} },
	}
	envs, err := eg.getEnvs(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedEnvsSortedByPriority, envs)
}

type fakeImageGetter struct {
	err           error
	imageMetadata registry.ImageMetadata
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
		client       kubernetes.Interface
		name         string
		expectedEnvs []string
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
		expectedErr  error
		client       kubernetes.Interface
		podspec      *apiv1.PodSpec
		name         string
		expectedEnvs []string
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
		{
			name: "dev container with env var from secret source",
			podspec: &apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						EnvFrom: []apiv1.EnvFromSource{
							{
								SecretRef: &v1.SecretEnvSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "name-of-test-secret",
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
			}),
			expectedEnvs: []string{
				"name-of-test-secret=test",
			},
		},
		{
			name: "dev container with env var from secret not found",
			podspec: &apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						EnvFrom: []apiv1.EnvFromSource{
							{
								SecretRef: &v1.SecretEnvSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "not-found-secret",
									},
								},
							},
						},
					},
				},
			},
			client:       fake.NewSimpleClientset(),
			expectedEnvs: []string{},
			expectedErr:  fmt.Errorf("error getting kubernetes secret: secrets \"not-found-secret\" not found: the development container didn't start successfully because the kubernetes secret 'not-found-secret' was not found"),
		},
		{
			name: "dev container with env var from cm",
			podspec: &apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						EnvFrom: []apiv1.EnvFromSource{
							{
								ConfigMapRef: &v1.ConfigMapEnvSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "name-of-test-env-from-cm",
									},
								},
							},
						},
					},
				},
			},
			client: fake.NewSimpleClientset(&apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name-of-test-env-from-cm",
					Namespace: "ns-test",
				},
				Data: map[string]string{
					"name-of-test-env-from-cm": "test",
				},
			}),
			expectedEnvs: []string{
				"name-of-test-env-from-cm=test",
			},
		},
		{
			name: "dev container with env var from cm not found",
			podspec: &apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						EnvFrom: []apiv1.EnvFromSource{
							{
								ConfigMapRef: &v1.ConfigMapEnvSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "not-found",
									},
								},
							},
						},
					},
				},
			},
			client:       fake.NewSimpleClientset(),
			expectedEnvs: []string{},
			expectedErr:  fmt.Errorf("configmaps \"not-found\" not found: the development container didn't start successfully because the kubernetes configmap 'not-found' was not found"),
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

type fakePlatformVariablesGetter struct {
	err               error
	platformVariables []env.Var
}

func (f fakePlatformVariablesGetter) GetOktetoPlatformVariables(context.Context) ([]env.Var, error) {
	return f.platformVariables, f.err
}

func TestGetEnvsFromPlatformVariables(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		fakeSecretsGetter fakePlatformVariablesGetter
		name              string
		expectedEnvs      []string
		isOkteto          bool
	}{
		{
			name:     "okteto not active",
			isOkteto: false,
		},
		{
			name:              "no platform variables",
			isOkteto:          true,
			fakeSecretsGetter: fakePlatformVariablesGetter{},
		},
		{
			name:     "with platform variables",
			isOkteto: true,
			fakeSecretsGetter: fakePlatformVariablesGetter{
				platformVariables: []env.Var{
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
			okteto.CurrentStore = &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"test": {
						Namespace: "test",
						IsOkteto:  tt.isOkteto,
					},
				},
				CurrentContext: "test",
			}
			secretEnvsGetter := platformVariablesEnvsGetter{
				variablesGetter: tt.fakeSecretsGetter,
			}
			envs, err := secretEnvsGetter.getEnvsFromPlatformVariables(ctx)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expectedEnvs, envs)
		})
	}
}

func TestGetEnvsFromSecretsError(t *testing.T) {
	secretEnvsGetter := platformVariablesEnvsGetter{
		variablesGetter: fakePlatformVariablesGetter{
			err: assert.AnError,
		},
	}
	envs, err := secretEnvsGetter.getEnvsFromPlatformVariables(context.Background())
	require.Error(t, err)
	require.Nil(t, envs)
}

func TestCheckOktetoStartError(t *testing.T) {
	msg := "test"
	tt := []struct {
		expected    error
		dev         *model.Dev
		K8sProvider *test.FakeK8sProvider
		name        string
	}{
		{
			name: "error providing k8s client",
			K8sProvider: &test.FakeK8sProvider{
				ErrProvide: assert.AnError,
			},
			expected: assert.AnError,
		},
		{
			name:        "error getting app",
			K8sProvider: test.NewFakeK8sProvider(),
			dev: &model.Dev{
				Name:      "test",
				Namespace: "test",
			},
			expected: apps.ErrApplicationNotFound{
				Name: "test",
			},
		},
		{
			name: "error refreshing",
			K8sProvider: test.NewFakeK8sProvider(
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				}),
			dev: &model.Dev{
				Name:      "test",
				Namespace: "test",
			},
			expected: &k8sErrors.StatusError{
				ErrStatus: metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusNotFound,
					Message: "deployments.apps \"test-okteto\" not found",
					Reason:  metav1.StatusReasonNotFound,
					Details: &metav1.StatusDetails{
						Name:  "test-okteto",
						Kind:  "deployments",
						Group: "apps",
					},
				},
			},
		},
		{
			name: "error getRunningPod",
			K8sProvider: test.NewFakeK8sProvider(
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto",
						Namespace: "test",
					},
				}),
			dev: &model.Dev{
				Name:      "test",
				Namespace: "test",
			},
			expected: fmt.Errorf("not found"),
		},
		{
			name: "error pv enabled",
			K8sProvider: test.NewFakeK8sProvider(
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto",
						Namespace: "test",
						UID:       "1234",
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "1",
						},
					},
				},
				&appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto-1234",
						Namespace: "test",
						UID:       "1234",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "Deployment",
								UID:  "1234",
							},
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "1",
						},
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto-1234",
						Namespace: "test",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "ReplicaSet",
								UID:  "1234",
							},
						},
					},
				}),
			dev: &model.Dev{
				Name:      "test",
				Namespace: "test",
			},
			expected: fmt.Errorf(msg),
		},
		{
			name: "error pv enabled",
			K8sProvider: test.NewFakeK8sProvider(
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto",
						Namespace: "test",
						UID:       "1234",
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "1",
						},
					},
				},
				&appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto-1234",
						Namespace: "test",
						UID:       "1234",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "Deployment",
								UID:  "1234",
							},
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "1",
						},
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto-1234",
						Namespace: "test",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "ReplicaSet",
								UID:  "1234",
							},
						},
					},
				}),
			dev: &model.Dev{
				Name:      "test",
				Namespace: "test",
				Secrets: []model.Secret{
					{
						LocalPath:  "test",
						RemotePath: "test",
					},
				},
			},
			expected: fmt.Errorf(msg),
		},
	}

	for _, tt := range tt {
		t.Run(tt.name, func(t *testing.T) {
			upCtx := &upContext{
				Dev:               tt.dev,
				K8sClientProvider: tt.K8sProvider,
				Options: &Options{
					ManifestPathFlag: "test",
				},
				Pod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto-1234",
						Namespace: "test",
					},
				},
			}
			err := upCtx.checkOktetoStartError(context.Background(), msg)
			assert.ErrorContains(t, err, tt.expected.Error())
		})
	}
}

func TestCleanCommand(t *testing.T) {
	tt := []struct {
		name              string
		k8sClientProvider *test.FakeK8sProvider
		expected          string
	}{
		{
			name: "error providing k8s client",
			k8sClientProvider: &test.FakeK8sProvider{
				ErrProvide: assert.AnError,
			},
			expected: "",
		},
	}
	for _, tt := range tt {
		t.Run(tt.name, func(t *testing.T) {
			upCtx := &upContext{
				K8sClientProvider: tt.k8sClientProvider,
			}
			upCtx.cleanCommand(context.Background())

			var output string
			select {
			case out := <-upCtx.cleaned:
				output = out
			default:
				output = ""
			}
			assert.Equal(t, tt.expected, output)
		})
	}
}
