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
	"errors"
	"fmt"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
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

type fakeDiscovery struct {
	errServerGroupsAndResources error
}

func (d *fakeDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, []*metav1.APIResourceList{}, d.errServerGroupsAndResources
}

// not implemented, needed for mocking the discovery client
func (*fakeDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}
func (*fakeDiscovery) OpenAPIV3() openapi.Client {
	return nil
}
func (*fakeDiscovery) RESTClient() rest.Interface {
	return nil
}
func (*fakeDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	return nil, nil
}
func (*fakeDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (*fakeDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (*fakeDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	return nil, nil
}
func (*fakeDiscovery) ServerVersion() (*version.Info, error) {
	return nil, nil
}
func (*fakeDiscovery) WithLegacy() discovery.DiscoveryInterface {
	return nil
}

type fakeK8sClient struct {
	*fake.Clientset
	discoveryClient discovery.DiscoveryInterface
}

func (f *fakeK8sClient) Discovery() discovery.DiscoveryInterface {
	return f.discoveryClient
}

func Test_wander(t *testing.T) {
	testErr := errors.New("test error")

	tests := []struct {
		name            string
		discoveryClient discovery.DiscoveryInterface
		expectedErr     error
	}{
		{
			name: "succeeds with no errors",
			discoveryClient: &fakeDiscovery{
				errServerGroupsAndResources: nil,
			},
			expectedErr: nil,
		},
		{
			name: "tolerates metrics.k8s.io error",
			discoveryClient: &fakeDiscovery{
				errServerGroupsAndResources: &discovery.ErrGroupDiscoveryFailed{
					Groups: map[schema.GroupVersion]error{
						{Group: "metrics.k8s.io", Version: "v1beta1"}: testErr,
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "tolerates custom.metrics.k8s.io error",
			discoveryClient: &fakeDiscovery{
				errServerGroupsAndResources: &discovery.ErrGroupDiscoveryFailed{
					Groups: map[schema.GroupVersion]error{
						{Group: "custom.metrics.k8s.io", Version: "v1beta2"}: testErr,
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "tolerates both metrics APIs errors",
			discoveryClient: &fakeDiscovery{
				errServerGroupsAndResources: &discovery.ErrGroupDiscoveryFailed{
					Groups: map[schema.GroupVersion]error{
						{Group: "metrics.k8s.io", Version: "v1beta1"}:        testErr,
						{Group: "custom.metrics.k8s.io", Version: "v1beta2"}: testErr,
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "returns error for non-metrics API",
			discoveryClient: &fakeDiscovery{
				errServerGroupsAndResources: &discovery.ErrGroupDiscoveryFailed{
					Groups: map[schema.GroupVersion]error{
						{Group: "other.k8s.io", Version: "v1"}: testErr,
					},
				},
			},
			expectedErr: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "other.k8s.io", Version: "v1"}: testErr,
				},
			},
		},
		{
			name: "returns error for non-metrics API when metrics.k8s.io also fails",
			discoveryClient: &fakeDiscovery{
				errServerGroupsAndResources: &discovery.ErrGroupDiscoveryFailed{
					Groups: map[schema.GroupVersion]error{
						{Group: "metrics.k8s.io", Version: "v1beta1"}: testErr,
						{Group: "other.k8s.io", Version: "v1"}:        testErr,
					},
				},
			},
			expectedErr: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "other.k8s.io", Version: "v1"}: testErr,
				},
			},
		},
		{
			name: "returns error for non-metrics API when custom.metrics.k8s.io also fails",
			discoveryClient: &fakeDiscovery{
				errServerGroupsAndResources: &discovery.ErrGroupDiscoveryFailed{
					Groups: map[schema.GroupVersion]error{
						{Group: "custom.metrics.k8s.io", Version: "v1beta2"}: testErr,
						{Group: "other.k8s.io", Version: "v1"}:               testErr,
					},
				},
			},
			expectedErr: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "other.k8s.io", Version: "v1"}: testErr,
				},
			},
		},
		{
			name: "returns error for non-metrics API when both metrics APIs also fail",
			discoveryClient: &fakeDiscovery{
				errServerGroupsAndResources: &discovery.ErrGroupDiscoveryFailed{
					Groups: map[schema.GroupVersion]error{
						{Group: "metrics.k8s.io", Version: "v1beta1"}:        testErr,
						{Group: "custom.metrics.k8s.io", Version: "v1beta2"}: testErr,
						{Group: "other.k8s.io", Version: "v1"}:               testErr,
					},
				},
			},
			expectedErr: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "other.k8s.io", Version: "v1"}: testErr,
				},
			},
		},
		{
			name: "returns error for multiple non-metrics APIs",
			discoveryClient: &fakeDiscovery{
				errServerGroupsAndResources: &discovery.ErrGroupDiscoveryFailed{
					Groups: map[schema.GroupVersion]error{
						{Group: "other.k8s.io", Version: "v1"}:   testErr,
						{Group: "another.k8s.io", Version: "v1"}: testErr,
					},
				},
			},
			expectedErr: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "other.k8s.io", Version: "v1"}:   testErr,
					{Group: "another.k8s.io", Version: "v1"}: testErr,
				},
			},
		},
		{
			name: "returns unknown errors",
			discoveryClient: &fakeDiscovery{
				errServerGroupsAndResources: testErr,
			},
			expectedErr: testErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

			trip := &Trip{
				k8s:       &fakeK8sClient{discoveryClient: tt.discoveryClient},
				dynamic:   dynamicClient,
				namespace: "test",
				list:      metav1.ListOptions{},
				sem:       semaphore.NewWeighted(10),
			}

			err := trip.wander(ctx, TravelerFunc(func(obj runtime.Object) error {
				return nil
			}))

			require.Equal(t, tt.expectedErr, err)
		})
	}
}
