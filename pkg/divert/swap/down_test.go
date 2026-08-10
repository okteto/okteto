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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	testService   = "api"
	testNamespace = "staging"
	otherService  = "catalog"
)

type testLogger struct {
	messages []string
	warnings []string
}

func (l *testLogger) Infof(format string, args ...interface{}) {
	l.messages = append(l.messages, fmt.Sprintf(format, args...))
}

func (l *testLogger) Warning(format string, args ...interface{}) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}

func originalSelector() map[string]string {
	return map[string]string{"app": "api"}
}

// swappedService is a Service as it looks while diverted: selecting the router pods, with
// the annotations describing how to put it back.
func swappedService() *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testService,
			Namespace: testNamespace,
			Annotations: map[string]string{
				originalSelectorAnnotation: `{"app":"api"}`,
				baselineServiceAnnotation:  BaselineServiceName(testService),
				routerDeploymentAnnotation: RouterDeploymentName(testService),
				"unrelated.example.com/a":  "keep me",
			},
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{routerPodLabel: testService},
		},
	}
}

// untouchedService is a Service that was never diverted.
func untouchedService() *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testService,
			Namespace: testNamespace,
		},
		Spec: apiv1.ServiceSpec{Selector: originalSelector()},
	}
}

func baselineServiceFor(service string) *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BaselineServiceName(service),
			Namespace: testNamespace,
			Labels:    map[string]string{managedServiceLabel: service},
		},
		Spec: apiv1.ServiceSpec{Selector: originalSelector()},
	}
}

func routerDeploymentFor(service string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RouterDeploymentName(service),
			Namespace: testNamespace,
			Labels:    map[string]string{managedServiceLabel: service},
		},
	}
}

// routeTableFor is the ConfigMap holding the service's routing keys, one data entry each.
func routeTableFor(routes map[string]string) *apiv1.ConfigMap {
	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RoutesConfigMapName(testService),
			Namespace: testNamespace,
			Labels:    map[string]string{managedServiceLabel: testService},
		},
		Data: routes,
	}
}

// divertedByTwo is a service two developers are diverting at the same time, which is the
// case the route table exists for.
func divertedByTwo() (*Client, *fake.Clientset) {
	return newTestClient(
		swappedService(),
		baselineServiceFor(testService),
		routerDeploymentFor(testService),
		routeTableFor(map[string]string{"alice": "alice-dev", "bob": "bob-dev"}),
	)
}

func getRouteTable(t *testing.T, k8s *fake.Clientset) map[string]string {
	t.Helper()

	cm, err := k8s.CoreV1().ConfigMaps(testNamespace).Get(context.Background(), RoutesConfigMapName(testService), metav1.GetOptions{})
	require.NoError(t, err)

	return cm.Data
}

func requireConfigMapGone(t *testing.T, k8s *fake.Clientset, name string) {
	t.Helper()

	_, err := k8s.CoreV1().ConfigMaps(testNamespace).Get(context.Background(), name, metav1.GetOptions{})
	require.True(t, k8sErrors.IsNotFound(err), "expected config map %s to be gone, got %v", name, err)
}

func testDownOptions() DownOptions {
	return DownOptions{
		Service:         testService,
		SharedNamespace: testNamespace,
	}
}

func newTestClient(objects ...runtime.Object) (*Client, *fake.Clientset) {
	k8s := fake.NewSimpleClientset(objects...)
	return NewClient(k8s, &testLogger{}), k8s
}

func getService(t *testing.T, k8s *fake.Clientset, name string) *apiv1.Service {
	t.Helper()

	svc, err := k8s.CoreV1().Services(testNamespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)

	return svc
}

func requireServiceGone(t *testing.T, k8s *fake.Clientset, name string) {
	t.Helper()

	_, err := k8s.CoreV1().Services(testNamespace).Get(context.Background(), name, metav1.GetOptions{})
	require.True(t, k8sErrors.IsNotFound(err), "expected service %s to be gone, got %v", name, err)
}

func requireDeploymentGone(t *testing.T, k8s *fake.Clientset, name string) {
	t.Helper()

	_, err := k8s.AppsV1().Deployments(testNamespace).Get(context.Background(), name, metav1.GetOptions{})
	require.True(t, k8sErrors.IsNotFound(err), "expected deployment %s to be gone, got %v", name, err)
}

func fullySwappedCluster() (*Client, *fake.Clientset) {
	return newTestClient(
		swappedService(),
		baselineServiceFor(testService),
		routerDeploymentFor(testService),
	)
}

func TestDown_RestoresTheOriginalSelector(t *testing.T) {
	c, k8s := fullySwappedCluster()

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	require.Equal(t, originalSelector(), getService(t, k8s, testService).Spec.Selector)
}

func TestDown_RemovesTheSwapAnnotations(t *testing.T) {
	c, k8s := fullySwappedCluster()

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	annotations := getService(t, k8s, testService).Annotations
	require.NotContains(t, annotations, originalSelectorAnnotation)
	require.NotContains(t, annotations, baselineServiceAnnotation)
	require.NotContains(t, annotations, routerDeploymentAnnotation)
}

func TestDown_LeavesUnrelatedAnnotationsAlone(t *testing.T) {
	c, k8s := fullySwappedCluster()

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	require.Equal(t, "keep me", getService(t, k8s, testService).Annotations["unrelated.example.com/a"])
}

func TestDown_DeletesTheRouterAndTheBaseline(t *testing.T) {
	c, k8s := fullySwappedCluster()

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	requireDeploymentGone(t, k8s, RouterDeploymentName(testService))
	requireServiceGone(t, k8s, BaselineServiceName(testService))
}

func TestDown_IsANoOpOnAServiceThatWasNeverDiverted(t *testing.T) {
	c, k8s := newTestClient(untouchedService())

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	require.Equal(t, originalSelector(), getService(t, k8s, testService).Spec.Selector)
}

func TestDown_IsIdempotent(t *testing.T) {
	c, k8s := fullySwappedCluster()
	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	require.Equal(t, originalSelector(), getService(t, k8s, testService).Spec.Selector)
}

// An interrupted bring-up creates the router and the baseline but never gets as far as
// annotating the Service. Teardown still has to find them.
func TestDown_CleansUpAfterAnInterruptedBringUp(t *testing.T) {
	c, k8s := newTestClient(
		untouchedService(),
		baselineServiceFor(testService),
		routerDeploymentFor(testService),
	)

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	requireDeploymentGone(t, k8s, RouterDeploymentName(testService))
	requireServiceGone(t, k8s, BaselineServiceName(testService))
}

func TestDown_SucceedsWhenTheBaselineIsAlreadyGone(t *testing.T) {
	c, k8s := newTestClient(swappedService(), routerDeploymentFor(testService))

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	require.Equal(t, originalSelector(), getService(t, k8s, testService).Spec.Selector)
	requireDeploymentGone(t, k8s, RouterDeploymentName(testService))
}

func TestDown_SucceedsWhenTheRouterIsAlreadyGone(t *testing.T) {
	c, k8s := newTestClient(swappedService(), baselineServiceFor(testService))

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	require.Equal(t, originalSelector(), getService(t, k8s, testService).Spec.Selector)
	requireServiceGone(t, k8s, BaselineServiceName(testService))
}

// The Service being gone is not a reason to leave the router and baseline behind.
func TestDown_CleansUpWhenTheServiceIsGone(t *testing.T) {
	c, k8s := newTestClient(baselineServiceFor(testService), routerDeploymentFor(testService))

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	requireDeploymentGone(t, k8s, RouterDeploymentName(testService))
	requireServiceGone(t, k8s, BaselineServiceName(testService))
}

func TestDown_LeavesAnotherServicesDivertAlone(t *testing.T) {
	c, k8s := newTestClient(
		swappedService(),
		baselineServiceFor(testService),
		routerDeploymentFor(testService),
		baselineServiceFor(otherService),
		routerDeploymentFor(otherService),
	)

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	require.NotNil(t, getService(t, k8s, BaselineServiceName(otherService)))
	_, err := k8s.AppsV1().Deployments(testNamespace).Get(context.Background(), RouterDeploymentName(otherService), metav1.GetOptions{})
	require.NoError(t, err)
}

// A selector that cannot be read back is the one case teardown must refuse. Deleting the
// router anyway would leave the Service selecting pods that no longer exist.
func TestDown_RefusesToActOnAnUnreadableSelectorAnnotation(t *testing.T) {
	svc := swappedService()
	svc.Annotations[originalSelectorAnnotation] = "{not json"
	c, k8s := newTestClient(svc, baselineServiceFor(testService), routerDeploymentFor(testService))

	err := c.Down(context.Background(), testDownOptions())

	require.ErrorContains(t, err, "unreadable")
	require.NotNil(t, getService(t, k8s, BaselineServiceName(testService)))
}

func TestDown_RefusesToActOnAnEmptySelectorAnnotation(t *testing.T) {
	svc := swappedService()
	svc.Annotations[originalSelectorAnnotation] = "{}"
	c, _ := newTestClient(svc)

	err := c.Down(context.Background(), testDownOptions())

	require.ErrorContains(t, err, "empty")
}

func TestReadState_Swapped(t *testing.T) {
	expected := state{
		OriginalSelector: originalSelector(),
		BaselineService:  BaselineServiceName(testService),
		RouterDeployment: RouterDeploymentName(testService),
	}

	result, swapped, err := readState(swappedService())

	require.NoError(t, err)
	require.True(t, swapped)
	require.Equal(t, expected, result)
}

func TestReadState_NotSwapped(t *testing.T) {
	result, swapped, err := readState(untouchedService())

	require.NoError(t, err)
	require.False(t, swapped)
	require.Equal(t, state{}, result)
}

// A service diverted by two developers: one leaving must not take the other's divert down.
func TestDown_RemovesOnlyTheDepartingDevelopersRoute(t *testing.T) {
	c, k8s := divertedByTwo()
	opts := testDownOptions()
	opts.RoutingKey = "alice"

	require.NoError(t, c.Down(context.Background(), opts))

	require.Equal(t, map[string]string{"bob": "bob-dev"}, getRouteTable(t, k8s))
	require.Equal(t, map[string]string{routerPodLabel: testService}, getService(t, k8s, testService).Spec.Selector)
	require.NotNil(t, getService(t, k8s, BaselineServiceName(testService)))
}

// The last developer out turns the lights off.
func TestDown_TearsEverythingDownWithTheLastRoute(t *testing.T) {
	c, k8s := newTestClient(
		swappedService(),
		baselineServiceFor(testService),
		routerDeploymentFor(testService),
		routeTableFor(map[string]string{"alice": "alice-dev"}),
	)
	opts := testDownOptions()
	opts.RoutingKey = "alice"

	require.NoError(t, c.Down(context.Background(), opts))

	require.Equal(t, originalSelector(), getService(t, k8s, testService).Spec.Selector)
	requireDeploymentGone(t, k8s, RouterDeploymentName(testService))
	requireServiceGone(t, k8s, BaselineServiceName(testService))
	requireConfigMapGone(t, k8s, RoutesConfigMapName(testService))
}

func TestDown_RefusesWhenTheRoutingKeyBelongsToSomeoneElse(t *testing.T) {
	c, k8s := divertedByTwo()
	opts := testDownOptions()
	opts.RoutingKey = "carol"

	err := c.Down(context.Background(), opts)

	require.ErrorContains(t, err, `not diverted with the routing key "carol"`)
	require.ErrorContains(t, err, `"alice", "bob"`)
	require.Len(t, getRouteTable(t, k8s), 2)
	require.Equal(t, map[string]string{routerPodLabel: testService}, getService(t, k8s, testService).Spec.Selector)
}

func TestDown_AllRemovesEveryonesRoutes(t *testing.T) {
	c, k8s := divertedByTwo()
	opts := testDownOptions()
	opts.All = true

	require.NoError(t, c.Down(context.Background(), opts))

	require.Equal(t, originalSelector(), getService(t, k8s, testService).Spec.Selector)
	requireDeploymentGone(t, k8s, RouterDeploymentName(testService))
	requireConfigMapGone(t, k8s, RoutesConfigMapName(testService))
}

// A divert with no route table predates this mechanism, or never got one written. Teardown
// still has to clean it up rather than refusing over a missing key.
func TestDown_WithNoRouteTableFallsBackToFullTeardown(t *testing.T) {
	c, k8s := newTestClient(swappedService(), baselineServiceFor(testService), routerDeploymentFor(testService))
	opts := testDownOptions()
	opts.RoutingKey = "alice"

	require.NoError(t, c.Down(context.Background(), opts))

	require.Equal(t, originalSelector(), getService(t, k8s, testService).Spec.Selector)
	requireServiceGone(t, k8s, BaselineServiceName(testService))
}

func TestDownOptions_Validate(t *testing.T) {
	tests := []struct {
		mutate          func(*DownOptions)
		name            string
		expectedMessage string
	}{
		{
			name:            "missing service",
			mutate:          func(o *DownOptions) { o.Service = "" },
			expectedMessage: "service to stop diverting is required",
		},
		{
			name:            "missing shared namespace",
			mutate:          func(o *DownOptions) { o.SharedNamespace = "" },
			expectedMessage: "shared namespace is required",
		},
		{
			name: "a routing key together with --all",
			mutate: func(o *DownOptions) {
				o.All = true
				o.RoutingKey = "alice"
			},
			expectedMessage: "cannot be given when tearing the whole divert down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := testDownOptions()
			tt.mutate(&opts)

			require.ErrorContains(t, opts.validate(), tt.expectedMessage)
		})
	}
}

func TestNames(t *testing.T) {
	require.Equal(t, "api-baseline", BaselineServiceName(testService))
	require.Equal(t, "api-divert-router", RouterDeploymentName(testService))
	require.Equal(t, "api-divert-routes", RoutesConfigMapName(testService))
}

func TestManagedByServiceSelector(t *testing.T) {
	require.Equal(t, "divert.okteto.com/service=api", managedByServiceSelector(testService))
}
