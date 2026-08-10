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

	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func getDisruptionBudget(t *testing.T, k8s *fake.Clientset, name string) *policyv1.PodDisruptionBudget {
	t.Helper()

	budget, err := k8s.PolicyV1().PodDisruptionBudgets(testNamespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)

	return budget
}

// From the swap onwards the router carries everyone's traffic to the shared service, so a
// node drain must not be allowed to take every replica at once.
func TestUp_GivesTheRouterADisruptionBudget(t *testing.T) {
	c, k8s := newUpClient(divertableService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	budget := getDisruptionBudget(t, k8s, RouterDeploymentName(testService))
	require.Equal(t, int32(1), budget.Spec.MinAvailable.IntVal)
	require.Equal(t, map[string]string{routerPodLabel: testService}, budget.Spec.Selector.MatchLabels)
	require.Equal(t, testService, budget.Labels[managedServiceLabel])
}

func TestDown_RemovesTheDisruptionBudget(t *testing.T) {
	c, k8s := newUpClient(divertableService())
	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.NoError(t, c.Down(context.Background(), testDownOptions()))

	_, err := k8s.PolicyV1().PodDisruptionBudgets(testNamespace).Get(context.Background(), RouterDeploymentName(testService), metav1.GetOptions{})
	require.True(t, k8sErrors.IsNotFound(err))
}

// A cluster without the policy API, or a developer without permission to create budgets,
// should still get a working divert.
func TestUp_SurvivesNotBeingAbleToCreateADisruptionBudget(t *testing.T) {
	logger := &testLogger{}
	c, k8s := newUpClient(divertableService())
	c.logger = logger
	k8s.PrependReactor("create", "poddisruptionbudgets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	require.Equal(t, map[string]string{routerPodLabel: testService}, getService(t, k8s, testService).Spec.Selector)
	require.Contains(t, logger.warnings[0], "disruption budget")
}

// Spreading the replicas is a preference, not a requirement: a single-node cluster is where
// this gets evaluated first, and DoNotSchedule would leave the router permanently pending.
func TestUp_SpreadsTheRouterAcrossNodesWithoutRequiringIt(t *testing.T) {
	c, k8s := newUpClient(divertableService())

	require.NoError(t, c.Up(context.Background(), testUpOptions()))

	constraints := getDeployment(t, k8s, RouterDeploymentName(testService)).Spec.Template.Spec.TopologySpreadConstraints
	require.Len(t, constraints, 1)
	require.Equal(t, apiv1.LabelHostname, constraints[0].TopologyKey)
	require.Equal(t, apiv1.ScheduleAnyway, constraints[0].WhenUnsatisfiable)
	require.Equal(t, int32(1), constraints[0].MaxSkew)
}
