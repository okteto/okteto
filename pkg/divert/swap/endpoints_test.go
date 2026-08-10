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
	"time"

	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	routerPodIP   = "10.1.0.7"
	baselinePodIP = "10.1.0.3"
)

func routerPod(ip string) *apiv1.Pod {
	return &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-divert-router-abc",
			Namespace: testNamespace,
			Labels:    map[string]string{routerPodLabel: testService},
		},
		Status: apiv1.PodStatus{PodIP: ip},
	}
}

// endpointSliceFor is the Service's EndpointSlice as the controller writes it.
func endpointSliceFor(ip string, ready bool) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-xyz",
			Namespace: testNamespace,
			Labels:    map[string]string{discoveryv1.LabelServiceName: testService},
		},
		Endpoints: []discoveryv1.Endpoint{{
			Addresses:  []string{ip},
			Conditions: discoveryv1.EndpointConditions{Ready: &ready},
		}},
	}
}

func TestWaitForServiceEndpoints_ReturnsOnceTheServiceReachesTheRouter(t *testing.T) {
	c, _ := newTestClient(routerPod(routerPodIP), endpointSliceFor(routerPodIP, true))

	err := c.waitForServiceEndpoints(context.Background(), testNamespace, testService, time.Second)

	require.NoError(t, err)
}

// This is the window that makes a fresh divert look broken: the selector is already
// swapped, but the Service's endpoints still name the pods it used to select.
func TestWaitForServiceEndpoints_WaitsWhileTheEndpointsStillNameTheOldPods(t *testing.T) {
	c, _ := newTestClient(routerPod(routerPodIP), endpointSliceFor(baselinePodIP, true))

	err := c.waitForServiceEndpoints(context.Background(), testNamespace, testService, 100*time.Millisecond)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestWaitForServiceEndpoints_IgnoresEndpointsThatAreNotReady(t *testing.T) {
	c, _ := newTestClient(routerPod(routerPodIP), endpointSliceFor(routerPodIP, false))

	err := c.waitForServiceEndpoints(context.Background(), testNamespace, testService, 100*time.Millisecond)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestWaitForServiceEndpoints_WaitsWhileTheRouterHasNoAddressYet(t *testing.T) {
	c, _ := newTestClient(routerPod(""), endpointSliceFor(routerPodIP, true))

	err := c.waitForServiceEndpoints(context.Background(), testNamespace, testService, 100*time.Millisecond)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// A divert that never takes effect must not be left in place: the shared Service would be
// pointing at a router nothing can reach.
func TestUp_RollsBackWhenTheSwapNeverTakesEffect(t *testing.T) {
	c, k8s := newUpClient(divertableService())
	c.waitForEndpoints = func(context.Context, string, string, time.Duration) error {
		return errors.New("timed out")
	}

	err := c.Up(context.Background(), testUpOptions())

	require.ErrorContains(t, err, "never started resolving to the router")
	require.Equal(t, originalSelector(), getService(t, k8s, testService).Spec.Selector)
	requireServiceGone(t, k8s, BaselineServiceName(testService))
	requireDeploymentGone(t, k8s, RouterDeploymentName(testService))
}

// Established connections do not move when a selector changes, and that is the single most
// likely reason a working divert looks intermittent. Bring-up has to say so.
func TestUp_WarnsThatExistingConnectionsKeepReachingTheBaseline(t *testing.T) {
	logger := &testLogger{}
	c, _ := newUpClient(divertableService())
	c.logger = logger

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.Len(t, logger.warnings, 1)
	require.Contains(t, logger.warnings[0], "reconnect")
}

func TestEndpointsReachAnyOf(t *testing.T) {
	ready := true

	tests := []struct {
		addresses map[string]bool
		name      string
		slices    []discoveryv1.EndpointSlice
		expected  bool
	}{
		{
			name:      "no slices at all",
			slices:    nil,
			addresses: map[string]bool{routerPodIP: true},
			expected:  false,
		},
		{
			name:      "a slice with no endpoints",
			slices:    []discoveryv1.EndpointSlice{{}},
			addresses: map[string]bool{routerPodIP: true},
			expected:  false,
		},
		{
			name: "an endpoint with no readiness recorded counts as ready",
			slices: []discoveryv1.EndpointSlice{{
				Endpoints: []discoveryv1.Endpoint{{Addresses: []string{routerPodIP}}},
			}},
			addresses: map[string]bool{routerPodIP: true},
			expected:  true,
		},
		{
			name: "a matching ready endpoint among others",
			slices: []discoveryv1.EndpointSlice{{
				Endpoints: []discoveryv1.Endpoint{
					{Addresses: []string{baselinePodIP}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
					{Addresses: []string{routerPodIP}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
				},
			}},
			addresses: map[string]bool{routerPodIP: true},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, endpointsReachAnyOf(tt.slices, tt.addresses))
		})
	}
}

func TestRouterPodIPs_SkipsPodsWithoutAnAddress(t *testing.T) {
	c, _ := newTestClient(routerPod(""), runtime.Object(routerPodWithName("api-divert-router-def", routerPodIP)))

	ips, err := c.routerPodIPs(context.Background(), testNamespace, testService)

	require.NoError(t, err)
	require.Equal(t, map[string]bool{routerPodIP: true}, ips)
}

func routerPodWithName(name, ip string) *apiv1.Pod {
	pod := routerPod(ip)
	pod.Name = name

	return pod
}
