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

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

// baselineWorkload is the Deployment behind the service being diverted: the pods every
// stale caller connection terminates at.
func baselineWorkload() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testService,
			Namespace: testNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: originalSelector()},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: originalSelector()},
			},
		},
	}
}

func unrelatedWorkload() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "catalog",
			Namespace: testNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "catalog"}},
			},
		},
	}
}

func restartedAt(t *testing.T, d *appsv1.Deployment) string {
	t.Helper()

	return d.Spec.Template.Annotations[restartedAtAnnotation]
}

func TestUp_RestartsTheBaselineSoCallersReconnect(t *testing.T) {
	c, k8s := newUpClient(divertableService(), baselineWorkload())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	stamp := restartedAt(t, getDeployment(t, k8s, testService))
	require.NotEmpty(t, stamp)
	_, err := time.Parse(time.RFC3339, stamp)
	require.NoError(t, err)
}

func TestUp_DoesNotRestartUnrelatedWorkloads(t *testing.T) {
	c, k8s := newUpClient(divertableService(), baselineWorkload(), unrelatedWorkload())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.Empty(t, restartedAt(t, getDeployment(t, k8s, "catalog")))
}

// Rolling the router would drop the traffic already flowing through it, and it is not where
// the stale connections terminate anyway.
func TestUp_NeverRestartsTheRouter(t *testing.T) {
	c, k8s := newUpClient(divertableService(), baselineWorkload())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.Empty(t, restartedAt(t, getDeployment(t, k8s, RouterDeploymentName(testService))))
}

func TestUp_SkipsTheRestartWhenAskedTo(t *testing.T) {
	c, k8s := newUpClient(divertableService(), baselineWorkload())
	opts := testUpOptions()
	opts.SkipBaselineRestart = true

	require.NoError(t, c.Up(context.Background(), opts))

	require.Empty(t, restartedAt(t, getDeployment(t, k8s, testService)))
}

func TestUp_WarnsWhenTheRestartIsSkipped(t *testing.T) {
	logger := &testLogger{}
	c, _ := newUpClient(divertableService(), baselineWorkload())
	c.logger = logger
	opts := testUpOptions()
	opts.SkipBaselineRestart = true

	require.NoError(t, c.Up(context.Background(), opts))

	require.Len(t, logger.warnings, 1)
	require.Contains(t, logger.warnings[0], "reconnect")
}

// The divert itself is live and correct by this point, so a restart that cannot happen is
// reported rather than treated as a failure.
func TestUp_WarnsButSucceedsWhenNoWorkloadMatches(t *testing.T) {
	logger := &testLogger{}
	c, k8s := newUpClient(divertableService())
	c.logger = logger

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.Len(t, logger.warnings, 1)
	require.Contains(t, logger.warnings[0], "no Deployment")
	require.Equal(t, map[string]string{routerPodLabel: testService}, getService(t, k8s, testService).Spec.Selector)
}

func TestUp_WarnsButSucceedsWhenTheRestartFails(t *testing.T) {
	logger := &testLogger{}
	c, k8s := newUpClient(divertableService(), baselineWorkload())
	c.logger = logger
	k8s.PrependReactor("patch", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.Len(t, logger.warnings, 1)
	require.Contains(t, logger.warnings[0], "forbidden")
}

func TestUp_WarnsButSucceedsWhenTheRestartNeverFinishes(t *testing.T) {
	logger := &testLogger{}
	c, _ := newUpClient(divertableService(), baselineWorkload())
	c.logger = logger
	c.waitForRollout = func(context.Context, string, string, time.Duration) error {
		return errors.New("timed out")
	}

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.Len(t, logger.warnings, 1)
	require.Contains(t, logger.warnings[0], "did not finish restarting")
}

func TestSelectorMatchesTemplate(t *testing.T) {
	tests := []struct {
		selector  map[string]string
		podLabels map[string]string
		name      string
		expected  bool
	}{
		{
			name:      "exact match",
			selector:  map[string]string{"app": "api"},
			podLabels: map[string]string{"app": "api"},
			expected:  true,
		},
		{
			name:      "pod carries extra labels",
			selector:  map[string]string{"app": "api"},
			podLabels: map[string]string{"app": "api", "version": "v2"},
			expected:  true,
		},
		{
			name:      "pod is missing a selector label",
			selector:  map[string]string{"app": "api", "tier": "backend"},
			podLabels: map[string]string{"app": "api"},
			expected:  false,
		},
		{
			name:      "value differs",
			selector:  map[string]string{"app": "api"},
			podLabels: map[string]string{"app": "catalog"},
			expected:  false,
		},
		{
			name:      "an empty selector must never match everything",
			selector:  map[string]string{},
			podLabels: map[string]string{"app": "api"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, selectorMatchesTemplate(tt.selector, tt.podLabels))
		})
	}
}

func TestRolloutComplete(t *testing.T) {
	tests := []struct {
		mutate   func(*appsv1.Deployment)
		name     string
		expected bool
	}{
		{
			name:     "fully rolled out",
			mutate:   func(*appsv1.Deployment) {},
			expected: true,
		},
		{
			name:     "status still describes the previous generation",
			mutate:   func(d *appsv1.Deployment) { d.Generation = 4 },
			expected: false,
		},
		{
			name:     "some pods not replaced yet",
			mutate:   func(d *appsv1.Deployment) { d.Status.UpdatedReplicas = 1 },
			expected: false,
		},
		{
			name:     "replaced but not ready",
			mutate:   func(d *appsv1.Deployment) { d.Status.ReadyReplicas = 1 },
			expected: false,
		},
		{
			name:     "old pods still terminating",
			mutate:   func(d *appsv1.Deployment) { d.Status.Replicas = 3 },
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicas := int32(2)
			d := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Generation: 3},
				Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
				Status: appsv1.DeploymentStatus{
					ObservedGeneration: 3,
					UpdatedReplicas:    2,
					ReadyReplicas:      2,
					Replicas:           2,
				},
			}
			tt.mutate(d)

			require.Equal(t, tt.expected, rolloutComplete(d))
		})
	}
}

func TestRolloutComplete_DefaultsToOneReplica(t *testing.T) {
	d := &appsv1.Deployment{
		Status: appsv1.DeploymentStatus{UpdatedReplicas: 1, ReadyReplicas: 1, Replicas: 1},
	}

	require.True(t, rolloutComplete(d))
}

func TestRestartedAtAnnotation_IsNamespaced(t *testing.T) {
	require.True(t, strings.HasPrefix(restartedAtAnnotation, "divert.okteto.com/"))
}
