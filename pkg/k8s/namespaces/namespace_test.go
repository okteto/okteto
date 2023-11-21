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

package namespaces

import (
	"context"
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDestroySFSVolumesIfNeeded(t *testing.T) {
	ns := "test"
	appName := "test-app"
	tests := []struct {
		name           string
		k8Resources    []runtime.Object
		expectedPVCs   []apiv1.PersistentVolumeClaim
		includeVolumes bool
	}{
		{
			name:           "WithoutVolumeClaimTemplates",
			includeVolumes: true,
			k8Resources: []runtime.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sfs-1",
						Namespace: ns,
						Labels: map[string]string{
							model.DeployedByLabel: appName,
						},
					},
				},
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-1",
						Namespace: ns,
					},
				},
			},
			expectedPVCs: []apiv1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-1",
						Namespace: ns,
					},
				},
			},
		},
		{
			name:           "WithOneVolumeClaimTemplate",
			includeVolumes: true,
			k8Resources: []runtime.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sfs-1",
						Namespace: ns,
						Labels: map[string]string{
							model.DeployedByLabel: appName,
						},
					},
					Spec: appsv1.StatefulSetSpec{
						VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "pvc",
								},
							},
						},
					},
				},
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-sfs-1-aadfwt312ad",
						Namespace: ns,
					},
				},
			},
			expectedPVCs: []apiv1.PersistentVolumeClaim{},
		},
		{
			name:           "WithVolumeClaimTemplatesWithoutHavingToIncludeVolumes",
			includeVolumes: false,
			k8Resources: []runtime.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sfs-1",
						Namespace: ns,
						Labels: map[string]string{
							model.DeployedByLabel: appName,
						},
					},
					Spec: appsv1.StatefulSetSpec{
						VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "pvc",
								},
							},
						},
					},
				},
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-sfs-1-aadfwt312ad",
						Namespace: ns,
					},
				},
			},
			expectedPVCs: []apiv1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-sfs-1-aadfwt312ad",
						Namespace: ns,
					},
				},
			},
		},
		{
			name:           "WithVolumeClaimTemplatesButVolumeWithDeployedByLabel",
			includeVolumes: true,
			k8Resources: []runtime.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sfs-1",
						Namespace: ns,
						Labels: map[string]string{
							model.DeployedByLabel: appName,
						},
					},
					Spec: appsv1.StatefulSetSpec{
						VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "pvc",
								},
							},
						},
					},
				},
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-sfs-1-aadfwt312ad",
						Namespace: ns,
						Labels: map[string]string{
							model.DeployedByLabel: appName,
						},
					},
				},
			},
			expectedPVCs: []apiv1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-sfs-1-aadfwt312ad",
						Namespace: ns,
						Labels: map[string]string{
							model.DeployedByLabel: appName,
						},
					},
				},
			},
		},
		{
			name:           "WithSeveralVolumeClaimTemplate",
			includeVolumes: true,
			k8Resources: []runtime.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sfs-1",
						Namespace: ns,
						Labels: map[string]string{
							model.DeployedByLabel: appName,
						},
					},
					Spec: appsv1.StatefulSetSpec{
						VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "pvc",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "mysql",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "redis",
								},
							},
						},
					},
				},
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-sfs-1-aadfwt312ad",
						Namespace: ns,
					},
				},
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysql-sfs-1-afahrret34",
						Namespace: ns,
					},
				},
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-sfs-1-jdtrtqwetq",
						Namespace: ns,
					},
				},
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mongodb",
						Namespace: ns,
					},
				},
			},
			expectedPVCs: []apiv1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mongodb",
						Namespace: ns,
					},
				},
			},
		},
		{
			name:           "WithVolumeClaimTemplateOnlyDeleteTheOnesWithoutKeepPolicy",
			includeVolumes: true,
			k8Resources: []runtime.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sfs-1",
						Namespace: ns,
						Labels: map[string]string{
							model.DeployedByLabel: appName,
						},
					},
					Spec: appsv1.StatefulSetSpec{
						VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "pvc",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "mysql",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "redis",
								},
							},
						},
					},
				},
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-sfs-1-aadfwt312ad",
						Namespace: ns,
						Annotations: map[string]string{
							resourcePolicyAnnotation: keepPolicy,
						},
					},
				},
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysql-sfs-1-afahrret34",
						Namespace: ns,
					},
				},
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-sfs-1-jdtrtqwetq",
						Namespace: ns,
					},
				},
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mongodb",
						Namespace: ns,
					},
				},
			},
			expectedPVCs: []apiv1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-sfs-1-aadfwt312ad",
						Namespace: ns,
						Annotations: map[string]string{
							resourcePolicyAnnotation: keepPolicy,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mongodb",
						Namespace: ns,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			c := fake.NewSimpleClientset(tt.k8Resources...)

			opts := DeleteAllOptions{
				IncludeVolumes: tt.includeVolumes,
				LabelSelector:  fmt.Sprintf("%s=%s", model.DeployedByLabel, appName),
			}

			n := &Namespaces{
				k8sClient: c,
			}

			err := n.DestroySFSVolumes(ctx, ns, opts)
			assert.NoError(t, err)

			pvcList, err := c.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})

			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.expectedPVCs, pvcList.Items)
		})
	}
}
