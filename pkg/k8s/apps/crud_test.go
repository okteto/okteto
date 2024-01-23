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

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
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
		Name:      "test",
		Namespace: "test",
		Image: &build.Info{
			Name: "image",
		},
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
		Name:      "test",
		Namespace: "test",
		Image: &build.Info{
			Name: "image",
		},
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
		Name:      "manifest-name",
		Namespace: "test",
		Dev: model.ManifestDevs{
			"dev": &model.Dev{
				Name:      "dev",
				Namespace: "test",
				Image: &build.Info{
					Name: "image",
				},
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
			},
			"sfs": &model.Dev{
				Name:      "sfs",
				Namespace: "test",
				Image: &build.Info{
					Name: "image",
				},
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
			},
			"autocreate": &model.Dev{
				Name:      "autocreate",
				Namespace: "test",
				Image: &build.Info{
					Name: "image",
				},
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

		result := ListDevModeOn(ctx, manifest, clientset)
		assert.ElementsMatch(t, tt.expectedList, result)

	}
}
