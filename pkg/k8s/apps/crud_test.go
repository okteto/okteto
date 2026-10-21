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

package apps

import (
	"context"
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

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

func TestGetStatefulset(t *testing.T) {
	ctx := context.Background()
	sfs := &appsv1.StatefulSet{
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
	}

	clientset := fake.NewSimpleClientset(sfs)

	dev := &model.Dev{
		Name:  "test",
		Image: "image",
		PersistentVolumeInfo: &model.PersistentVolumeInfo{
			Enabled: true,
		},
	}
	app, err := Get(ctx, dev, "test", clientset)
	if err != nil {
		t.Fatal(err)
	}
	if app.ObjectMeta().Name != "test" {
		t.Fatal("not retrieved correctly")
	}
}

func TestGetDeployment(t *testing.T) {
	ctx := context.Background()
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
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
	}

	clientset := fake.NewSimpleClientset(d)

	dev := &model.Dev{
		Name:  "test",
		Image: "image",
		PersistentVolumeInfo: &model.PersistentVolumeInfo{
			Enabled: true,
		},
	}
	app, err := Get(ctx, dev, "test", clientset)
	if err != nil {
		t.Fatal(err)
	}
	if app.ObjectMeta().Name != "test" {
		t.Fatal("not retrieved correctly")
	}
}

func TestValidateMountPaths(t *testing.T) {
	tests := []struct {
		spec          *v1.PodSpec
		dev           *model.Dev
		name          string
		expectedError bool
	}{
		{
			name: "Correct validation sfs",
			spec: &v1.PodSpec{
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
			dev: &model.Dev{
				Name: "test",
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							RemotePath: "/data2",
						},
					},
				},
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
			},
			expectedError: false,
		},
		{
			name: "Wrong validation",
			spec: &v1.PodSpec{
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
			dev: &model.Dev{
				Name: "test",
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							RemotePath: "/data",
						},
					},
				},
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
			},
			expectedError: true,
		},
		{
			name: "Wrong validation pv disabled",
			spec: &v1.PodSpec{
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
			dev: &model.Dev{
				Name: "test",
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							RemotePath: "/data",
						},
					},
				},
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: false,
				},
			},
			expectedError: false,
		},
		{
			name: "Wrong validation second up",
			spec: &v1.PodSpec{
				Containers: []v1.Container{
					{
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "test-okteto",
								MountPath: "/data",
							},
						},
					},
				},
			},
			dev: &model.Dev{
				Name: "test",
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							RemotePath: "/data",
						},
					},
				},
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := ValidateMountPaths(tt.spec, tt.dev)

			if err == nil && tt.expectedError {
				t.Fatalf("Didn't receive any error and it was expected")
			} else if err != nil && !tt.expectedError {
				t.Fatalf("Receive error and it was not expected")
			}
		})
	}

}

func TestListDevModeOn(t *testing.T) {
	manifest := &model.Manifest{
		Name: "manifest-name",
		Dev: model.ManifestDevs{
			"dev": &model.Dev{
				Name:  "dev",
				Image: "image",
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
			},
			"sfs": &model.Dev{
				Name:  "sfs",
				Image: "image",
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
			},
			"autocreate": &model.Dev{
				Name:  "autocreate",
				Image: "image",
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
				Autocreate: true,
			},
		},
	}
	tests := []struct {
		expectedError error
		sfs           *appsv1.StatefulSet
		ds            *appsv1.Deployment
		name          string
		expectedList  []string
	}{
		{
			name: "none-dev-mode",
			sfs: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sfs",
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
			},
			ds: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dev",
					Namespace: "test",
				},
				Spec: appsv1.DeploymentSpec{
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
			},
			expectedList: []string{},
		},
		{
			name: "dev-is-dev-mode",
			sfs: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sfs",
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
			},
			ds: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dev",
					Namespace: "test",
					Labels: map[string]string{
						constants.DevLabel: "true",
					},
				},
				Spec: appsv1.DeploymentSpec{
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
			},
			expectedList: []string{"dev"},
		},
		{
			name: "both-are-dev-mode",
			sfs: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sfs",
					Namespace: "test",
					Labels: map[string]string{
						constants.DevLabel: "true",
					},
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
			},
			ds: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dev",
					Namespace: "test",
					Labels: map[string]string{
						constants.DevLabel: "true",
					},
				},
				Spec: appsv1.DeploymentSpec{
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
			},
			expectedList: []string{"dev", "sfs"},
		},
		{
			name: "err-dev-not-found",
			sfs: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sfs-other",
					Namespace: "test",
					Labels: map[string]string{
						constants.DevLabel: "true",
					},
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
			},
			ds:           &appsv1.Deployment{},
			expectedList: []string{},
		},
		{
			name: "autocreate-dev",
			sfs:  &appsv1.StatefulSet{},
			ds: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "autocreate-okteto",
					Namespace: "test",
					Labels: map[string]string{
						constants.DevLabel: "true",
					},
				},
				Spec: appsv1.DeploymentSpec{
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
			},
			expectedList: []string{"autocreate"},
		},
		{
			name: "dev-is-not-at-manifest",
			sfs:  &appsv1.StatefulSet{},
			ds: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "not-manifest",
					Namespace: "test",
					Labels: map[string]string{
						constants.DevLabel: "true",
					},
				},
				Spec: appsv1.DeploymentSpec{
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
			},
			expectedList: []string{},
		},
	}

	for _, tt := range tests {
		ctx := context.Background()
		clientset := fake.NewSimpleClientset(tt.sfs, tt.ds)

		result := ListDevModeOn(ctx, manifest.Dev, "test", clientset)
		assert.ElementsMatch(t, tt.expectedList, result)

	}
}

func TestGetTranslations_InheritKubernetesResources(t *testing.T) {
	tests := []struct {
		name              string
		envVarValue       string
		dev               *model.Dev
		deployment        *appsv1.Deployment
		expectedResources model.ResourceRequirements
		shouldInherit     bool
	}{
		{
			name:        "inherit resources when env var is true and dev resources are empty",
			envVarValue: "true",
			dev: &model.Dev{
				Name:      "test",
				Container: "test",
				Resources: model.ResourceRequirements{},
			},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "test",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceMemory: resource.MustParse("128Mi"),
											v1.ResourceCPU:    resource.MustParse("100m"),
										},
										Limits: v1.ResourceList{
											v1.ResourceMemory: resource.MustParse("256Mi"),
											v1.ResourceCPU:    resource.MustParse("200m"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResources: model.ResourceRequirements{
				Requests: model.ResourceList{
					v1.ResourceMemory: resource.MustParse("128Mi"),
					v1.ResourceCPU:    resource.MustParse("100m"),
				},
				Limits: model.ResourceList{
					v1.ResourceMemory: resource.MustParse("256Mi"),
					v1.ResourceCPU:    resource.MustParse("200m"),
				},
			},
			shouldInherit: true,
		},
		{
			name:        "do not inherit when env var is false",
			envVarValue: "false",
			dev: &model.Dev{
				Name:      "test",
				Container: "test",
				Resources: model.ResourceRequirements{},
			},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "test",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceMemory: resource.MustParse("128Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResources: model.ResourceRequirements{},
			shouldInherit:     false,
		},
		{
			name:        "do not inherit when dev resources are not empty",
			envVarValue: "true",
			dev: &model.Dev{
				Name:      "test",
				Container: "test",
				Resources: model.ResourceRequirements{
					Requests: model.ResourceList{
						v1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "test",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceMemory: resource.MustParse("128Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResources: model.ResourceRequirements{
				Requests: model.ResourceList{
					v1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
			shouldInherit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(model.OktetoInheritKubernetesResourcesEnvVar, tt.envVarValue)

			ctx := context.Background()
			clientset := fake.NewSimpleClientset(tt.deployment)

			app := NewDeploymentApp(tt.deployment)
			translations, err := GetTranslations(ctx, "test", "test-manifest", tt.dev, app, false, clientset)

			assert.NoError(t, err)
			assert.NotNil(t, translations)

			// Check that the main translation exists
			mainTranslation, exists := translations[tt.deployment.Name]
			assert.True(t, exists)
			assert.NotNil(t, mainTranslation)

			assert.Equal(t, tt.expectedResources, mainTranslation.Rules[0].Resources)
		})
	}
}

func TestGetTranslations_InheritKubernetesNodeSelector(t *testing.T) {
	tests := []struct {
		name                 string
		envVarValue          string
		dev                  *model.Dev
		deployment           *appsv1.Deployment
		expectedNodeSelector map[string]string
		shouldInherit        bool
	}{
		{
			name:        "inherit nodeSelector when env var is true and dev nodeSelector is empty",
			envVarValue: "true",
			dev: &model.Dev{
				Name:         "test",
				Container:    "app",
				NodeSelector: nil,
			},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							NodeSelector: map[string]string{
								"kubernetes.io/os":   "linux",
								"kubernetes.io/arch": "amd64",
							},
							Containers: []v1.Container{
								{
									Name:  "app",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			expectedNodeSelector: map[string]string{
				"kubernetes.io/os":   "linux",
				"kubernetes.io/arch": "amd64",
			},
			shouldInherit: true,
		},
		{
			name:        "do not inherit when env var is false",
			envVarValue: "false",
			dev: &model.Dev{
				Name:         "test",
				Container:    "app",
				NodeSelector: nil,
			},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							NodeSelector: map[string]string{
								"node-type": "gpu",
							},
							Containers: []v1.Container{
								{
									Name:  "app",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			expectedNodeSelector: nil,
			shouldInherit:        false,
		},
		{
			name:        "do not inherit when dev nodeSelector is not empty",
			envVarValue: "true",
			dev: &model.Dev{
				Name:      "test",
				Container: "app",
				NodeSelector: map[string]string{
					"custom-label": "custom-value",
				},
			},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							NodeSelector: map[string]string{
								"kubernetes.io/os": "linux",
							},
							Containers: []v1.Container{
								{
									Name:  "app",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			expectedNodeSelector: map[string]string{
				"custom-label": "custom-value",
			},
			shouldInherit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(model.OktetoInheritKubernetesNodeSelectorEnvVar, tt.envVarValue)

			ctx := context.Background()
			clientset := fake.NewSimpleClientset(tt.deployment)

			app := NewDeploymentApp(tt.deployment)
			translations, err := GetTranslations(ctx, "test", "test-manifest", tt.dev, app, false, clientset)

			assert.NoError(t, err)
			assert.NotNil(t, translations)

			// Check that the main translation exists
			mainTranslation, exists := translations["test"]
			assert.True(t, exists)
			assert.NotNil(t, mainTranslation)

			// Check that there's at least one rule
			assert.Greater(t, len(mainTranslation.Rules), 0)

			assert.Equal(t, tt.expectedNodeSelector, mainTranslation.Rules[0].NodeSelector)
		})
	}
}
