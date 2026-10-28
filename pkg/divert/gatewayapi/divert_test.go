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

package gatewayapi

import (
	"context"
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/divert/k8s"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type fakeDivertManager struct {
	mock.Mock
}

func (f *fakeDivertManager) CreateOrUpdate(ctx context.Context, d *k8s.Divert) error {
	args := f.Called(ctx, d)
	return args.Error(0)
}

func (f *fakeDivertManager) List(ctx context.Context, namespace string) ([]*k8s.Divert, error) {
	args := f.Called(ctx, namespace)
	return args.Get(0).([]*k8s.Divert), args.Error(1)
}

func (f *fakeDivertManager) Delete(ctx context.Context, name, namespace string) error {
	args := f.Called(ctx, name, namespace)
	return args.Error(0)
}

func Test_updateEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		envVars  []apiv1.EnvVar
		envName  string
		envValue string
		expected []apiv1.EnvVar
	}{
		{
			name:     "empty-env-vars",
			envVars:  []apiv1.EnvVar{},
			envName:  "TEST_VAR",
			envValue: "test-value",
			expected: []apiv1.EnvVar{
				{
					Name:  "TEST_VAR",
					Value: "test-value",
				},
			},
		},
		{
			name: "update-existing-var",
			envVars: []apiv1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
				{
					Name:  "TEST_VAR",
					Value: "old-value",
				},
			},
			envName:  "TEST_VAR",
			envValue: "new-value",
			expected: []apiv1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
				{
					Name:  "TEST_VAR",
					Value: "new-value",
				},
			},
		},
		{
			name: "add-new-var",
			envVars: []apiv1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
			},
			envName:  "NEW_VAR",
			envValue: "new-value",
			expected: []apiv1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
				{
					Name:  "NEW_VAR",
					Value: "new-value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVars := tt.envVars
			updateEnvVar(&envVars, tt.envName, tt.envValue)
			assert.Equal(t, tt.expected, envVars)
		})
	}
}

func Test_UpdatePod(t *testing.T) {
	tests := []struct {
		name     string
		podSpec  apiv1.PodSpec
		expected apiv1.PodSpec
		driver   *Driver
	}{
		{
			name: "empty-pod",
			podSpec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app",
					},
				},
			},
			expected: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app",
						Env: []apiv1.EnvVar{
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
				DNSConfig: &apiv1.PodDNSConfig{
					Searches: []string{"staging.svc.cluster.local"},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
		{
			name: "pod-with-existing-env-vars",
			podSpec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "old-value",
							},
						},
					},
				},
				DNSConfig: &apiv1.PodDNSConfig{
					Searches: []string{"existing-search"},
				},
			},
			expected: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
				DNSConfig: &apiv1.PodDNSConfig{
					Searches: []string{"staging.svc.cluster.local", "existing-search"},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
		{
			name: "pod-with-multiple-containers",
			podSpec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app1",
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
						},
					},
				},
			},
			expected: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app1",
						Env: []apiv1.EnvVar{
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
				DNSConfig: &apiv1.PodDNSConfig{
					Searches: []string{"staging.svc.cluster.local"},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
		{
			name: "pod-with-init-containers",
			podSpec: apiv1.PodSpec{
				InitContainers: []apiv1.Container{
					{
						Name: "app1",
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
						},
					},
				},
			},
			expected: apiv1.PodSpec{
				InitContainers: []apiv1.Container{
					{
						Name: "app1",
						Env: []apiv1.EnvVar{
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
				DNSConfig: &apiv1.PodDNSConfig{
					Searches: []string{"staging.svc.cluster.local"},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
		{
			name: "pod-with-init-containers-and-main-containers",
			podSpec: apiv1.PodSpec{
				InitContainers: []apiv1.Container{
					{
						Name: "app1",
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name: "app3",
					},
				},
			},
			expected: apiv1.PodSpec{
				InitContainers: []apiv1.Container{
					{
						Name: "app1",
						Env: []apiv1.EnvVar{
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name: "app3",
						Env: []apiv1.EnvVar{
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
				DNSConfig: &apiv1.PodDNSConfig{
					Searches: []string{"staging.svc.cluster.local"},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.driver.UpdatePod(tt.podSpec)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// requireDeveloperNamespace checks the annotations of the ingresses in the developer namespace, by name,
// and that no service was created there
func requireDeveloperNamespace(t *testing.T, c *fake.Clientset, expectedIngresses map[string]map[string]string) {
	t.Helper()
	ctx := context.Background()
	ingresses, err := c.NetworkingV1().Ingresses("cindy").List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	annotations := map[string]map[string]string{}
	for _, in := range ingresses.Items {
		annotations[in.Name] = in.Annotations
	}
	require.Equal(t, expectedIngresses, annotations)

	services, err := c.CoreV1().Services("cindy").List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, services.Items)
}

func newDeployDriver(c *fake.Clientset) (*Driver, *fakeDivertManager) {
	dm := &fakeDivertManager{}
	dm.On("List", mock.Anything, "cindy").Return([]*k8s.Divert{}, nil)
	d := &Driver{client: c, name: "test", namespace: "cindy", divert: model.DivertDeploy{Namespace: "staging"}, divertManager: dm}
	return d, dm
}

func Test_Deploy_Ingresses(t *testing.T) {
	leftoverCopyAnnotations := map[string]string{
		"a1":                                    "v1",
		model.OktetoAutoCreateAnnotation:        "true",
		model.OktetoDivertedNamespaceAnnotation: "staging",
		model.OktetoDivertHeaderAnnotation:      "cindy",
	}
	tests := []struct {
		expected map[string]map[string]string
		name     string
		objects  []runtime.Object
	}{
		{
			name: "ingress only in the shared namespace is not created",
			objects: []runtime.Object{
				newIngress("i1", "staging", "s1", map[string]string{"a1": "v1"}),
				&apiv1.Service{ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "staging"}},
			},
			expected: map[string]map[string]string{},
		},
		{
			name: "developer ingress matching a shared ingress gets the divert header",
			objects: []runtime.Object{
				newIngress("i1", "staging", "s1", map[string]string{"a1": "v2"}),
				newIngress("i1", "cindy", "s1", map[string]string{"a1": "v1"}),
			},
			expected: map[string]map[string]string{
				"i1": {"a1": "v1", model.OktetoDivertHeaderAnnotation: "cindy"},
			},
		},
		{
			name: "developer ingress without a shared ingress is not modified",
			objects: []runtime.Object{
				newIngress("i1", "staging", "s1", map[string]string{"a1": "v2"}),
				newIngress("i2", "cindy", "s2", map[string]string{"a1": "v1"}),
			},
			expected: map[string]map[string]string{
				"i2": {"a1": "v1"},
			},
		},
		{
			name: "backend service only in the shared namespace is not created",
			objects: []runtime.Object{
				newIngress("i1", "staging", "s2", map[string]string{"a1": "v2"}),
				&apiv1.Service{ObjectMeta: metav1.ObjectMeta{Name: "s2", Namespace: "staging"}},
				newIngress("i1", "cindy", "s2", map[string]string{"a1": "v1"}),
			},
			expected: map[string]map[string]string{
				"i1": {"a1": "v1", model.OktetoDivertHeaderAnnotation: "cindy"},
			},
		},
		{
			name: "leftover copy of a shared ingress is not translated again",
			objects: []runtime.Object{
				newIngress("i1", "staging", "s1", map[string]string{"a1": "v2"}),
				newIngress("i1", "cindy", "s1", leftoverCopyAnnotations),
			},
			expected: map[string]map[string]string{
				"i1": leftoverCopyAnnotations,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientset(tt.objects...)
			d, dm := newDeployDriver(c)

			err := d.Deploy(context.Background())
			require.NoError(t, err)
			requireDeveloperNamespace(t, c, tt.expected)
			dm.AssertExpectations(t)
		})
	}
}

func Test_Deploy_IngressPatchError(t *testing.T) {
	c := fake.NewClientset(
		newIngress("i1", "staging", "s1", nil),
		newIngress("i1", "cindy", "s1", nil),
	)
	c.PrependReactor("patch", "ingresses", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("patch error")
	})
	d, _ := newDeployDriver(c)

	err := d.Deploy(context.Background())
	require.EqualError(t, err, "patch error")
}

func Test_deployDivertResources_Success(t *testing.T) {
	ctx := context.Background()

	developerServices := map[string]*apiv1.Service{
		"svc1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc1",
				Namespace: "cindy",
				Annotations: map[string]string{
					model.OktetoDivertedNamespaceAnnotation: "staging",
				},
			},
		},
		"svc2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:        "svc2",
				Namespace:   "cindy",
				Annotations: map[string]string{},
			},
		},
		"svc3": {
			ObjectMeta: metav1.ObjectMeta{
				Name:        "svc3",
				Namespace:   "cindy",
				Annotations: map[string]string{},
			},
		},
	}

	dm := &fakeDivertManager{}
	dm.On("CreateOrUpdate", mock.Anything, mock.MatchedBy(func(d *k8s.Divert) bool {
		return d.Name == "test-svc2" &&
			d.Namespace == "cindy" &&
			d.Spec.Service == "svc2" &&
			d.Spec.SharedNamespace == "staging" &&
			d.Spec.DivertKey == "cindy"
	})).Return(nil).Once()

	dm.On("CreateOrUpdate", mock.Anything, mock.MatchedBy(func(d *k8s.Divert) bool {
		return d.Name == "test-svc3" &&
			d.Namespace == "cindy" &&
			d.Spec.Service == "svc3" &&
			d.Spec.SharedNamespace == "staging" &&
			d.Spec.DivertKey == "cindy"
	})).Return(nil).Once()

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			Namespace: "staging",
		},
		divertManager: dm,
		cache: &cache{
			developerServices: developerServices,
		},
	}

	err := d.deployDivertResources(ctx)

	assert.NoError(t, err)
	dm.AssertExpectations(t)
}

func Test_deployDivertResources_NoServices(t *testing.T) {
	ctx := context.Background()

	developerServices := map[string]*apiv1.Service{}

	dm := &fakeDivertManager{}

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			Namespace: "staging",
		},
		divertManager: dm,
		cache: &cache{
			developerServices: developerServices,
		},
	}

	err := d.deployDivertResources(ctx)

	assert.NoError(t, err)
	dm.AssertNumberOfCalls(t, "CreateOrUpdate", 0)
}

func Test_deployDivertResources_OnlyAnnotatedServices(t *testing.T) {
	ctx := context.Background()

	developerServices := map[string]*apiv1.Service{
		"svc1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc1",
				Namespace: "cindy",
				Annotations: map[string]string{
					model.OktetoDivertedNamespaceAnnotation: "staging",
				},
			},
		},
		"svc2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc2",
				Namespace: "cindy",
				Annotations: map[string]string{
					model.OktetoDivertedNamespaceAnnotation: "staging",
				},
			},
		},
	}

	dm := &fakeDivertManager{}

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			Namespace: "staging",
		},
		divertManager: dm,
		cache: &cache{
			developerServices: developerServices,
		},
	}

	err := d.deployDivertResources(ctx)

	assert.NoError(t, err)
	dm.AssertNumberOfCalls(t, "CreateOrUpdate", 0)
}

func Test_deployDivertResources_CreateError(t *testing.T) {
	ctx := context.Background()

	developerServices := map[string]*apiv1.Service{
		"svc1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:        "svc1",
				Namespace:   "cindy",
				Annotations: map[string]string{},
			},
		},
	}

	dm := &fakeDivertManager{}
	dm.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(fmt.Errorf("failed to create divert")).Once()

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			Namespace: "staging",
		},
		divertManager: dm,
		cache: &cache{
			developerServices: developerServices,
		},
	}

	err := d.deployDivertResources(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error diverting service")
	dm.AssertExpectations(t)
}

func Test_deployDivertResources_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	developerServices := map[string]*apiv1.Service{
		"svc1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:        "svc1",
				Namespace:   "cindy",
				Annotations: map[string]string{},
			},
		},
	}

	dm := &fakeDivertManager{}

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			Namespace: "staging",
		},
		divertManager: dm,
		cache: &cache{
			developerServices: developerServices,
		},
	}

	err := d.deployDivertResources(ctx)

	assert.Error(t, err)
	assert.Equal(t, ctx.Err(), err)
	dm.AssertNumberOfCalls(t, "CreateOrUpdate", 0)
}

func Test_deleteOrphanDiverts_Success(t *testing.T) {
	ctx := context.Background()

	divertResources := map[string]*k8s.Divert{
		"divert1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "divert1",
				Namespace: "cindy",
			},
			Spec: k8s.DivertSpec{
				Service: "service1",
			},
		},
		"divert2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "divert2",
				Namespace: "cindy",
			},
			Spec: k8s.DivertSpec{
				Service: "service2",
			},
		},
		"divert3": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "divert3",
				Namespace: "cindy",
			},
			Spec: k8s.DivertSpec{
				Service: "service3",
			},
		},
	}

	developerServices := map[string]*apiv1.Service{
		"service1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service1",
				Namespace: "cindy",
			},
		},
	}

	dm := &fakeDivertManager{}
	dm.On("Delete", mock.Anything, "divert2", "cindy").Return(nil).Once()
	dm.On("Delete", mock.Anything, "divert3", "cindy").Return(nil).Once()

	d := &Driver{
		namespace:     "cindy",
		divertManager: dm,
		cache: &cache{
			divertResources:   divertResources,
			developerServices: developerServices,
		},
	}

	err := d.deleteOrphanDiverts(ctx)

	require.NoError(t, err)
	dm.AssertExpectations(t)
	dm.AssertNotCalled(t, "Delete", mock.Anything, "divert1", "cindy")
}

func Test_deleteOrphanDiverts_NoOrphans(t *testing.T) {
	ctx := context.Background()

	divertResources := map[string]*k8s.Divert{
		"divert1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "divert1",
				Namespace: "cindy",
			},
			Spec: k8s.DivertSpec{
				Service: "service1",
			},
		},
		"divert2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "divert2",
				Namespace: "cindy",
			},
			Spec: k8s.DivertSpec{
				Service: "service2",
			},
		},
	}

	developerServices := map[string]*apiv1.Service{
		"service1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service1",
				Namespace: "cindy",
			},
		},
		"service2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service2",
				Namespace: "cindy",
			},
		},
	}

	dm := &fakeDivertManager{}

	d := &Driver{
		namespace:     "cindy",
		divertManager: dm,
		cache: &cache{
			divertResources:   divertResources,
			developerServices: developerServices,
		},
	}

	err := d.deleteOrphanDiverts(ctx)

	require.NoError(t, err)
	dm.AssertNumberOfCalls(t, "Delete", 0)
}

func Test_deleteOrphanDiverts_DeleteError(t *testing.T) {
	ctx := context.Background()

	divertResources := map[string]*k8s.Divert{
		"divert1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "divert1",
				Namespace: "cindy",
			},
			Spec: k8s.DivertSpec{
				Service: "service1",
			},
		},
		"divert2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "divert2",
				Namespace: "cindy",
			},
			Spec: k8s.DivertSpec{
				Service: "service2",
			},
		},
	}

	developerServices := map[string]*apiv1.Service{}

	dm := &fakeDivertManager{}
	isExpectedDivert := func(arg interface{}) bool {
		divertName := arg.(string)
		return divertName == "divert1" || divertName == "divert2"
	}
	isExpectedNamespace := func(arg interface{}) bool {
		namespace := arg.(string)
		return namespace == "cindy"
	}
	dm.On("Delete", mock.Anything, mock.MatchedBy(isExpectedDivert), mock.MatchedBy(isExpectedNamespace)).Return(nil).Once()
	dm.On("Delete", mock.Anything, mock.MatchedBy(isExpectedDivert), mock.MatchedBy(isExpectedNamespace)).Return(fmt.Errorf("delete error")).Once()

	d := &Driver{
		namespace:     "cindy",
		divertManager: dm,
		cache: &cache{
			divertResources:   divertResources,
			developerServices: developerServices,
		},
	}

	err := d.deleteOrphanDiverts(ctx)

	require.Error(t, err)
	require.Contains(t, err.Error(), "delete error")
	dm.AssertExpectations(t)
}
