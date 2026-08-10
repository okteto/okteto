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
	"strings"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/divert/router"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const testTargetNamespace = "alice-dev"

// divertableService is a Service that satisfies every bring-up precondition.
func divertableService() *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testService,
			Namespace: testNamespace,
		},
		Spec: apiv1.ServiceSpec{
			Type:      apiv1.ServiceTypeClusterIP,
			ClusterIP: "10.0.0.1",
			Selector:  originalSelector(),
			Ports: []apiv1.ServicePort{{
				Name:       "http",
				Port:       80,
				TargetPort: intstr.FromInt32(8080),
				Protocol:   apiv1.ProtocolTCP,
			}},
			SessionAffinity: apiv1.ServiceAffinityClientIP,
		},
	}
}

func testUpOptions() UpOptions {
	return UpOptions{
		Service:          testService,
		SharedNamespace:  testNamespace,
		TargetNamespace:  testTargetNamespace,
		RoutingKey:       "alice",
		RouterImage:      "ghcr.io/okteto/okteto:3.0.0",
		ReadinessTimeout: time.Second,
	}
}

// newUpClient returns a client whose router always comes up, so that tests exercise the
// choreography rather than a controller that is not running.
func newUpClient(objects ...runtime.Object) (*Client, *fake.Clientset) {
	c, k8s := newTestClient(objects...)
	c.waitForReady = func(context.Context, string, string, int32, time.Duration) error { return nil }
	c.waitForEndpoints = func(context.Context, string, string, time.Duration) error { return nil }
	c.waitForRollout = func(context.Context, string, string, time.Duration) error { return nil }

	return c, k8s
}

func getDeployment(t *testing.T, k8s *fake.Clientset, name string) *appsv1.Deployment {
	t.Helper()

	d, err := k8s.AppsV1().Deployments(testNamespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)

	return d
}

func envOf(container apiv1.Container) map[string]string {
	env := map[string]string{}
	for _, e := range container.Env {
		env[e.Name] = e.Value
	}

	return env
}

func TestUp_CreatesTheBaselineWithTheOriginalSelectorAndPorts(t *testing.T) {
	c, k8s := newUpClient(divertableService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	baseline := getService(t, k8s, BaselineServiceName(testService))
	require.Equal(t, originalSelector(), baseline.Spec.Selector)
	require.Equal(t, divertableService().Spec.Ports, baseline.Spec.Ports)
	require.Equal(t, apiv1.ServiceAffinityClientIP, baseline.Spec.SessionAffinity)
	require.Equal(t, testService, baseline.Labels[managedServiceLabel])
}

func TestUp_PointsTheServiceAtTheRouter(t *testing.T) {
	c, k8s := newUpClient(divertableService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	svc := getService(t, k8s, testService)
	require.Equal(t, map[string]string{routerPodLabel: testService}, svc.Spec.Selector)
}

func TestUp_RecordsHowToUndoItselfOnTheService(t *testing.T) {
	c, k8s := newUpClient(divertableService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	annotations := getService(t, k8s, testService).Annotations
	require.JSONEq(t, `{"app":"api"}`, annotations[originalSelectorAnnotation])
	require.Equal(t, BaselineServiceName(testService), annotations[baselineServiceAnnotation])
	require.Equal(t, RouterDeploymentName(testService), annotations[routerDeploymentAnnotation])
}

// Bring-up followed by teardown must leave the Service exactly as it was found.
func TestUp_IsUndoneByDown(t *testing.T) {
	c, k8s := newUpClient(divertableService())
	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	svc := getService(t, k8s, testService)
	require.Equal(t, originalSelector(), svc.Spec.Selector)
	require.Empty(t, svc.Annotations)
	requireServiceGone(t, k8s, BaselineServiceName(testService))
	requireDeploymentGone(t, k8s, RouterDeploymentName(testService))
}

func TestUp_ConfiguresTheRouterFromTheService(t *testing.T) {
	c, k8s := newUpClient(divertableService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	env := envOf(getDeployment(t, k8s, RouterDeploymentName(testService)).Spec.Template.Spec.Containers[0])
	require.Equal(t, testService, env["SERVICE_NAME"])
	require.Equal(t, testNamespace, env["SHARED_NAMESPACE"])
	require.Equal(t, "api-baseline.staging.svc.cluster.local", env["BASELINE_HOST"])
	// No name: the Service targets port 8080 by number, so the router's container port does
	// not need one. A named targetPort is the case that carries a name across.
	require.JSONEq(t, `[{"listen":8080,"service":80}]`, env["PORTS"])
	require.Equal(t, "9191", env["HEALTH_PORT"])
	require.Equal(t, routesMountPath, env["ROUTES_DIR"])
	require.Empty(t, env["ROUTES"], "routes come from the mounted table, not the environment")
}

// multiPortService exposes an HTTP port addressed by number and a gRPC port addressed by
// name, which are the two cases the router has to handle differently.
func multiPortService() *apiv1.Service {
	svc := divertableService()
	svc.Spec.Ports = append(svc.Spec.Ports, apiv1.ServicePort{
		Name:       "grpc",
		Port:       9090,
		TargetPort: intstr.FromString("grpc-port"),
		Protocol:   apiv1.ProtocolTCP,
	})

	return svc
}

func TestUp_ServesEveryPortOfAMultiPortService(t *testing.T) {
	c, k8s := newUpClient(multiPortService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	env := envOf(getDeployment(t, k8s, RouterDeploymentName(testService)).Spec.Template.Spec.Containers[0])
	require.JSONEq(t,
		`[{"listen":8080,"service":80},{"name":"grpc-port","listen":8081,"service":9090}]`,
		env["PORTS"],
	)
}

// The named port must not be handed a number a numeric port already pins, or the router
// fails to bind its second listener and the whole divert never comes up.
func TestUp_GivesEveryPortItsOwnListenPort(t *testing.T) {
	c, k8s := newUpClient(multiPortService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	ports := getDeployment(t, k8s, RouterDeploymentName(testService)).Spec.Template.Spec.Containers[0].Ports
	seen := map[int32]bool{}
	for _, port := range ports {
		require.False(t, seen[port.ContainerPort], "container port %d declared twice", port.ContainerPort)
		seen[port.ContainerPort] = true
	}
}

// Two Service ports targeting the same pod port cannot be told apart by a router.
func TestPlanPorts_RefusesTwoPortsTargetingTheSamePodPort(t *testing.T) {
	svc := divertableService()
	svc.Spec.Ports = append(svc.Spec.Ports, apiv1.ServicePort{
		Name: "metrics", Port: 9090, TargetPort: intstr.FromInt32(8080), Protocol: apiv1.ProtocolTCP,
	})

	_, err := planPorts(svc)

	require.ErrorContains(t, err, "targets port 8080 from more than one of its ports")
}

// A Service targeting its pods by port name resolves that name against the router's own
// container ports, so the name has to survive onto them.
func TestUp_PreservesNamedTargetPortsOnTheRouterContainer(t *testing.T) {
	c, k8s := newUpClient(multiPortService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	ports := getDeployment(t, k8s, RouterDeploymentName(testService)).Spec.Template.Spec.Containers[0].Ports
	require.Equal(t, "", ports[0].Name)
	require.Equal(t, "grpc-port", ports[1].Name)
}

func TestUp_TheBaselineKeepsEveryPortVerbatim(t *testing.T) {
	c, k8s := newUpClient(multiPortService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.Equal(t, multiPortService().Spec.Ports, getService(t, k8s, BaselineServiceName(testService)).Spec.Ports)
}

func TestUp_MountsTheRouteTableIntoTheRouter(t *testing.T) {
	c, k8s := newUpClient(divertableService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	pod := getDeployment(t, k8s, RouterDeploymentName(testService)).Spec.Template.Spec
	require.Len(t, pod.Volumes, 1)
	require.Equal(t, RoutesConfigMapName(testService), pod.Volumes[0].ConfigMap.Name)
	require.Len(t, pod.Containers[0].VolumeMounts, 1)
	require.Equal(t, routesMountPath, pod.Containers[0].VolumeMounts[0].MountPath)
}

// A missing table must never stop the router starting: an empty table sends everything to
// the baseline, whereas a pod stuck in ContainerCreating takes the shared service down.
func TestUp_TheRouteTableMountIsOptional(t *testing.T) {
	c, k8s := newUpClient(divertableService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	volume := getDeployment(t, k8s, RouterDeploymentName(testService)).Spec.Template.Spec.Volumes[0]
	require.True(t, *volume.ConfigMap.Optional)
}

// The table is written before the router so its mount is already populated at start-up,
// which is what makes the first divert work immediately.
func TestUp_RegistersTheRoutingKeyBeforeStartingTheRouter(t *testing.T) {
	c, k8s := newUpClient(divertableService())
	var routesWhenRouterStarted map[string]string
	c.waitForReady = func(context.Context, string, string, int32, time.Duration) error {
		routesWhenRouterStarted = getRouteTable(t, k8s)
		return nil
	}

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.Equal(t, map[string]string{"alice": testTargetNamespace}, routesWhenRouterStarted)
}

func TestUp_LabelsTheRouterPodsForTheSwappedSelector(t *testing.T) {
	c, k8s := newUpClient(divertableService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	deployment := getDeployment(t, k8s, RouterDeploymentName(testService))
	require.Equal(t, testService, deployment.Spec.Template.Labels[routerPodLabel])
	require.Equal(t, testService, deployment.Spec.Template.Labels[managedServiceLabel])
	require.Equal(t, int32(defaultReplicas), *deployment.Spec.Replicas)
}

// The router drains for 15 seconds; a shorter grace period would have the kubelet kill it
// mid-request.
func TestUp_GivesTheRouterTimeToDrain(t *testing.T) {
	c, k8s := newUpClient(divertableService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	deployment := getDeployment(t, k8s, RouterDeploymentName(testService))
	require.Greater(t, *deployment.Spec.Template.Spec.TerminationGracePeriodSeconds, int64(15))
}

func TestUp_WaitsForTheRouterBeforeSwapping(t *testing.T) {
	c, k8s := newUpClient(divertableService())
	var selectorWhenWaited map[string]string
	c.waitForReady = func(context.Context, string, string, int32, time.Duration) error {
		selectorWhenWaited = getService(t, k8s, testService).Spec.Selector
		return nil
	}

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.Equal(t, originalSelector(), selectorWhenWaited)
}

func TestUp_RollsBackWhenTheRouterNeverBecomesReady(t *testing.T) {
	c, k8s := newUpClient(divertableService())
	c.waitForReady = func(context.Context, string, string, int32, time.Duration) error {
		return errors.New("timed out")
	}

	err := c.Up(context.Background(), testUpOptions())

	require.ErrorContains(t, err, "did not become ready")
	require.Equal(t, originalSelector(), getService(t, k8s, testService).Spec.Selector)
	requireServiceGone(t, k8s, BaselineServiceName(testService))
	requireDeploymentGone(t, k8s, RouterDeploymentName(testService))
}

func TestUp_RollsBackWhenTheSwapFails(t *testing.T) {
	c, k8s := newUpClient(divertableService())
	k8s.PrependReactor("update", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("denied by admission webhook")
	})

	err := c.Up(context.Background(), testUpOptions())

	require.ErrorContains(t, err, "error pointing service")
	requireServiceGone(t, k8s, BaselineServiceName(testService))
	requireDeploymentGone(t, k8s, RouterDeploymentName(testService))
}

func TestUp_RefusesWhenAPreviousAttemptLeftABaselineBehind(t *testing.T) {
	c, _ := newUpClient(divertableService(), baselineServiceFor(testService))

	err := c.Up(context.Background(), testUpOptions())

	require.ErrorContains(t, err, "okteto divert down")
}

func TestUp_RefusesWhenAPreviousAttemptLeftARouterBehind(t *testing.T) {
	c, _ := newUpClient(divertableService(), routerDeploymentFor(testService))

	err := c.Up(context.Background(), testUpOptions())

	require.ErrorContains(t, err, "okteto divert down")
}

func TestUp_RefusesWhenTheServiceDoesNotExist(t *testing.T) {
	c, _ := newUpClient()

	err := c.Up(context.Background(), testUpOptions())

	require.ErrorContains(t, err, "error reading service")
}

func TestPlanSwap_Refusals(t *testing.T) {
	headless := divertableService()
	headless.Spec.ClusterIP = apiv1.ClusterIPNone

	externalName := divertableService()
	externalName.Spec.Type = apiv1.ServiceTypeExternalName

	noSelector := divertableService()
	noSelector.Spec.Selector = nil

	noPorts := divertableService()
	noPorts.Spec.Ports = nil

	udp := divertableService()
	udp.Spec.Ports[0].Protocol = apiv1.ProtocolUDP

	// A single non-TCP port among otherwise fine ones still cannot be header-routed.
	mixedProtocols := divertableService()
	mixedProtocols.Spec.Ports = append(mixedProtocols.Spec.Ports, apiv1.ServicePort{
		Name: "dns", Port: 53, Protocol: apiv1.ProtocolUDP,
	})

	unreadable := divertableService()
	unreadable.Annotations = map[string]string{originalSelectorAnnotation: "{not json"}

	tests := []struct {
		svc             *apiv1.Service
		name            string
		expectedMessage string
	}{
		{name: "external name", svc: externalName, expectedMessage: "no pods to put a router in front of"},
		{name: "headless", svc: headless, expectedMessage: "headless"},
		{name: "no selector", svc: noSelector, expectedMessage: "has no selector"},
		{name: "no ports", svc: noPorts, expectedMessage: "exposes no ports"},
		{name: "udp port", svc: udp, expectedMessage: "only TCP can be header-routed"},
		{name: "a udp port among tcp ones", svc: mixedProtocols, expectedMessage: `the UDP port "dns"`},
		{name: "unreadable annotation", svc: unreadable, expectedMessage: "unreadable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := planSwap(tt.svc)

			require.ErrorContains(t, err, tt.expectedMessage)
		})
	}
}

// A refused Service must be left exactly as it was, with nothing created alongside it.
func TestUp_CreatesNothingWhenTheServiceIsRefused(t *testing.T) {
	headless := divertableService()
	headless.Spec.ClusterIP = apiv1.ClusterIPNone
	c, k8s := newUpClient(headless)

	require.Error(t, c.Up(context.Background(), testUpOptions()))

	requireServiceGone(t, k8s, BaselineServiceName(testService))
	requireDeploymentGone(t, k8s, RouterDeploymentName(testService))
}

func TestUpOptions_Validate(t *testing.T) {
	tests := []struct {
		mutate          func(*UpOptions)
		name            string
		expectedMessage string
	}{
		{
			name:            "missing service",
			mutate:          func(o *UpOptions) { o.Service = "" },
			expectedMessage: "service to divert is required",
		},
		{
			name:            "missing shared namespace",
			mutate:          func(o *UpOptions) { o.SharedNamespace = "" },
			expectedMessage: "shared namespace is required",
		},
		{
			name:            "missing target namespace",
			mutate:          func(o *UpOptions) { o.TargetNamespace = "" },
			expectedMessage: "target namespace is required",
		},
		{
			name:            "missing routing key",
			mutate:          func(o *UpOptions) { o.RoutingKey = "" },
			expectedMessage: "routing key is required",
		},
		{
			name:            "missing router image",
			mutate:          func(o *UpOptions) { o.RouterImage = "" },
			expectedMessage: "router image is required",
		},
		{
			name:            "diverting a namespace to itself",
			mutate:          func(o *UpOptions) { o.TargetNamespace = o.SharedNamespace },
			expectedMessage: "cannot be diverted to itself",
		},
		{
			name:            "service name too long for the derived objects",
			mutate:          func(o *UpOptions) { o.Service = strings.Repeat("a", maxNameLength) },
			expectedMessage: "too long to divert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := testUpOptions()
			tt.mutate(&opts)

			require.ErrorContains(t, opts.validate(), tt.expectedMessage)
		})
	}
}

func TestUpOptions_Defaults(t *testing.T) {
	opts := UpOptions{}

	opts.setDefaults()

	require.Equal(t, int32(defaultReplicas), opts.Replicas)
	require.Equal(t, defaultReadinessTimeout, opts.ReadinessTimeout)
}

func TestPlanPorts(t *testing.T) {
	tests := []struct {
		name     string
		ports    []apiv1.ServicePort
		expected []router.PortConfig
	}{
		{
			name:     "numeric target port pins the router to it",
			ports:    []apiv1.ServicePort{{Port: 80, TargetPort: intstr.FromInt32(8080)}},
			expected: []router.PortConfig{{Listen: 8080, Service: 80}},
		},
		{
			name:     "named target port leaves the router free to choose",
			ports:    []apiv1.ServicePort{{Port: 80, TargetPort: intstr.FromString("http")}},
			expected: []router.PortConfig{{Name: "http", Listen: defaultRouterListenPort, Service: 80}},
		},
		{
			name:     "unset target port defaults to the service port",
			ports:    []apiv1.ServicePort{{Port: 80}},
			expected: []router.PortConfig{{Listen: 80, Service: 80}},
		},
		{
			name: "a named port fills in around the numeric ones",
			ports: []apiv1.ServicePort{
				{Port: 80, TargetPort: intstr.FromString("http")},
				{Port: 90, TargetPort: intstr.FromInt32(defaultRouterListenPort)},
			},
			expected: []router.PortConfig{
				{Name: "http", Listen: defaultRouterListenPort + 1, Service: 80},
				{Listen: defaultRouterListenPort, Service: 90},
			},
		},
		{
			name: "two named ports get two different listen ports",
			ports: []apiv1.ServicePort{
				{Port: 80, TargetPort: intstr.FromString("http")},
				{Port: 90, TargetPort: intstr.FromString("grpc")},
			},
			expected: []router.PortConfig{
				{Name: "http", Listen: defaultRouterListenPort, Service: 80},
				{Name: "grpc", Listen: defaultRouterListenPort + 1, Service: 90},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := divertableService()
			svc.Spec.Ports = tt.ports

			ports, err := planPorts(svc)

			require.NoError(t, err)
			require.Equal(t, tt.expected, ports)
		})
	}
}

func TestHealthPortFor_AvoidsTheProxyPort(t *testing.T) {
	require.Equal(t, int32(routerHealthPort), healthPortFor([]int32{8080}))
	require.Equal(t, int32(alternateRouterHealthPort), healthPortFor([]int32{routerHealthPort}))
}

// A service exposing both candidates leaves neither free, and readiness still needs a port
// of its own.
func TestHealthPortFor_WalksPastBothCandidates(t *testing.T) {
	require.Equal(t, int32(alternateRouterHealthPort+1), healthPortFor([]int32{routerHealthPort, alternateRouterHealthPort}))
}

func TestRouterSecurityContext_AddsNetBindServiceOnlyForPrivilegedPorts(t *testing.T) {
	unprivileged := plan{ports: []router.PortConfig{{Listen: 8080, Service: 80}}}
	privileged := plan{ports: []router.PortConfig{{Listen: 80, Service: 80}}}

	require.Empty(t, routerSecurityContext(unprivileged).Capabilities.Add)
	require.Equal(t, []apiv1.Capability{"NET_BIND_SERVICE"}, routerSecurityContext(privileged).Capabilities.Add)
}

// One privileged port among several is enough to need the capability.
func TestRouterSecurityContext_AddsNetBindServiceForAnyPrivilegedPort(t *testing.T) {
	mixed := plan{ports: []router.PortConfig{{Listen: 8080, Service: 80}, {Listen: 443, Service: 443}}}

	require.Equal(t, []apiv1.Capability{"NET_BIND_SERVICE"}, routerSecurityContext(mixed).Capabilities.Add)
}

func TestRouterSecurityContext_DropsEverythingElse(t *testing.T) {
	sc := routerSecurityContext(plan{ports: []router.PortConfig{{Listen: 8080, Service: 80}}})

	require.Equal(t, []apiv1.Capability{"ALL"}, sc.Capabilities.Drop)
	require.False(t, *sc.AllowPrivilegeEscalation)
}
