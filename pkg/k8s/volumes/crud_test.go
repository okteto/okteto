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

package volumes

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
)

func Test_checkPVCValues(t *testing.T) {
	className := "class"
	var tests = []struct {
		name      string
		pvc       *apiv1.PersistentVolumeClaim
		dev       *model.Dev
		wantError bool
	}{
		{
			name: "pvc-with-more-storage-size",
			pvc: &apiv1.PersistentVolumeClaim{
				Spec: apiv1.PersistentVolumeClaimSpec{
					StorageClassName: &className,
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": resource.MustParse("20Gi"),
						},
					},
				},
			},
			dev: &model.Dev{
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Size:         "30Gi",
					StorageClass: "class",
				},
			},
			wantError: false,
		},

		{
			name: "ok-without-storage-class",
			pvc: &apiv1.PersistentVolumeClaim{
				Spec: apiv1.PersistentVolumeClaimSpec{
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": resource.MustParse("20Gi"),
						},
					},
				},
			},
			dev: &model.Dev{
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Size: "20Gi",
				},
			},
			wantError: false,
		},
		{
			name: "ok-with-storage-class",
			pvc: &apiv1.PersistentVolumeClaim{
				Spec: apiv1.PersistentVolumeClaimSpec{
					StorageClassName: &className,
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": resource.MustParse("20Gi"),
						},
					},
				},
			},
			dev: &model.Dev{
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Size:         "20Gi",
					StorageClass: "class",
				},
			},
			wantError: false,
		},
		{
			name: "pvc-without-storage",
			pvc: &apiv1.PersistentVolumeClaim{
				Spec: apiv1.PersistentVolumeClaimSpec{
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"cpu": resource.MustParse("1"),
						},
					},
				},
			},
			dev: &model.Dev{
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Size:         "20Gi",
					StorageClass: "class",
				},
			},
			wantError: true,
		},

		{
			name: "pvc-with-less-storage-size",
			pvc: &apiv1.PersistentVolumeClaim{
				Spec: apiv1.PersistentVolumeClaimSpec{
					StorageClassName: &className,
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": resource.MustParse("20Gi"),
						},
					},
				},
			},
			dev: &model.Dev{
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Size:         "10Gi",
					StorageClass: "class",
				},
			},
			wantError: true,
		},
		{
			name: "pvc-with-wrong-storage-class",
			pvc: &apiv1.PersistentVolumeClaim{
				Spec: apiv1.PersistentVolumeClaimSpec{
					StorageClassName: &className,
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": resource.MustParse("20Gi"),
						},
					},
				},
			},
			dev: &model.Dev{
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Size:         "20Gi",
					StorageClass: "wrong-class",
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkPVCValues(tt.pvc, tt.dev, "")
			if err == nil && tt.wantError {
				t.Errorf("checkPVCValues in test '%s' did not fail", tt.name)
			}
			if err != nil && !tt.wantError {
				t.Errorf("checkPVCValues in test '%s' failed: %s", tt.name, err)
			}
		})
	}
}

func TestDestroyWithoutTimeout(t *testing.T) {
	ctx := context.Background()
	pvcName := "pvc-1"
	ns := "test"
	pvc := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvcName,
		},
	}
	c := fake.NewSimpleClientset(pvc)

	err := DestroyWithoutTimeout(ctx, pvcName, ns, c)
	assert.NoError(t, err)

	pvcList, err := c.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(pvcList.Items))
}

func TestDestroyWithoutTimeoutNoExistentVolumen(t *testing.T) {
	ctx := context.Background()
	pvcName := "pvc-1"
	ns := "test"
	pvc := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvc-2",
		},
	}
	c := fake.NewSimpleClientset(pvc)

	err := DestroyWithoutTimeout(ctx, pvcName, ns, c)

	assert.NoError(t, err)
}

func TestDestroyWithoutTimeoutWithGenericError(t *testing.T) {
	ctx := context.Background()
	pvcName := "pvc-1"
	ns := "test"
	pvc := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvcName,
		},
	}
	c := fake.NewSimpleClientset(pvc)

	c.Fake.PrependReactor("delete", "persistentvolumeclaims", func(action k8sTesting.Action) (bool, runtime.Object, error) {
		return true, nil, assert.AnError
	})

	err := DestroyWithoutTimeout(ctx, pvcName, ns, c)

	assert.Error(t, err)
}
