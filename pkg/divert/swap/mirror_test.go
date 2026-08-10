// Copyright 2026 The Okteto Authors
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

package swap

import (
	"context"
	"errors"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// plainService is an unremarkable service in a namespace, the kind that gets mirrored.
func plainService(name, namespace string) *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{"app": name},
			Ports:    []apiv1.ServicePort{{Port: 80}},
		},
	}
}

func getTargetService(t *testing.T, k8s *fake.Clientset, name string) *apiv1.Service {
	t.Helper()

	svc, err := k8s.CoreV1().Services(testTargetNamespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)

	return svc
}

func requireTargetServiceGone(t *testing.T, k8s *fake.Clientset, name string) {
	t.Helper()

	_, err := k8s.CoreV1().Services(testTargetNamespace).Get(context.Background(), name, metav1.GetOptions{})
	require.True(t, k8sErrors.IsNotFound(err), "expected service %s to be absent, got %v", name, err)
}

func TestUp_MirrorsSharedServicesTheDeveloperDoesNotHave(t *testing.T) {
	c, k8s := newUpClient(divertableService(), plainService("catalog", testNamespace))

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	mirror := getTargetService(t, k8s, "catalog")
	require.Equal(t, apiv1.ServiceTypeExternalName, mirror.Spec.Type)
	require.Equal(t, "catalog.staging.svc.cluster.local", mirror.Spec.ExternalName)
	require.Equal(t, "true", mirror.Annotations[model.OktetoAutoCreateAnnotation])
	require.Equal(t, testNamespace, mirror.Annotations[model.OktetoDivertedNamespaceAnnotation])
}

func TestUp_LeavesTheDevelopersOwnCopyAlone(t *testing.T) {
	own := plainService("catalog", testTargetNamespace)
	c, k8s := newUpClient(divertableService(), plainService("catalog", testNamespace), own)

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.Equal(t, own.Spec.Selector, getTargetService(t, k8s, "catalog").Spec.Selector)
}

// Mirroring the diverted service would send the router's own forward straight back to the
// shared namespace and into the router again.
func TestUp_NeverMirrorsTheDivertedServiceItself(t *testing.T) {
	c, k8s := newUpClient(divertableService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	requireTargetServiceGone(t, k8s, testService)
}

func TestUp_DoesNotMirrorItsOwnPlumbing(t *testing.T) {
	c, k8s := newUpClient(divertableService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	requireTargetServiceGone(t, k8s, BaselineServiceName(testService))
}

func TestUp_ToleratesAServiceAppearingWhileMirroring(t *testing.T) {
	c, k8s := newUpClient(divertableService(), plainService("catalog", testNamespace))
	k8s.PrependReactor("create", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
		create := action.(k8stesting.CreateAction)
		if create.GetNamespace() != testTargetNamespace {
			return false, nil, nil
		}
		return true, nil, k8sErrors.NewAlreadyExists(apiv1.Resource("services"), "catalog")
	})

	require.NoError(t, c.Up(context.Background(), testUpOptions()))
}

// The divert is live by the time mirroring runs, so a failure is reported without undoing it.
func TestUp_ReportsAMirroringFailureWithoutUndoingTheDivert(t *testing.T) {
	c, k8s := newUpClient(divertableService(), plainService("catalog", testNamespace))
	k8s.PrependReactor("create", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
		create := action.(k8stesting.CreateAction)
		if create.GetNamespace() != testTargetNamespace {
			return false, nil, nil
		}
		return true, nil, errors.New("forbidden")
	})

	err := c.Up(context.Background(), testUpOptions())

	require.ErrorContains(t, err, "may fail to resolve them")
	require.Equal(t, map[string]string{routerPodLabel: testService}, getService(t, k8s, testService).Spec.Selector)
	require.NotNil(t, getService(t, k8s, BaselineServiceName(testService)))
}

// Mirrors are shared by every divert into the same namespace, so teardown must not remove
// them: doing so would break a second divert that is still running.
func TestDown_LeavesMirrorsInPlace(t *testing.T) {
	c, k8s := newUpClient(divertableService(), plainService("catalog", testNamespace))
	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	require.Equal(t, apiv1.ServiceTypeExternalName, getTargetService(t, k8s, "catalog").Spec.Type)
}

func TestShouldMirror(t *testing.T) {
	managed := plainService("api-baseline", testNamespace)
	managed.Labels = map[string]string{managedServiceLabel: testService}

	tests := []struct {
		developerHas map[string]bool
		svc          *apiv1.Service
		name         string
		expected     bool
	}{
		{
			name:         "a shared service the developer lacks",
			svc:          plainService("catalog", testNamespace),
			developerHas: map[string]bool{},
			expected:     true,
		},
		{
			name:         "a service the developer already has",
			svc:          plainService("catalog", testNamespace),
			developerHas: map[string]bool{"catalog": true},
			expected:     false,
		},
		{
			name:         "the diverted service itself",
			svc:          plainService(testService, testNamespace),
			developerHas: map[string]bool{},
			expected:     false,
		},
		{
			name:         "plumbing created by a divert",
			svc:          managed,
			developerHas: map[string]bool{},
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, shouldMirror(tt.svc, testService, tt.developerHas))
		})
	}
}
