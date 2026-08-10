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
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

// alreadyDiverted is a service Alice has diverted, which Bob is about to join.
func alreadyDiverted(extra ...runtime.Object) (*Client, *fake.Clientset) {
	objects := []runtime.Object{
		swappedService(),
		baselineServiceFor(testService),
		routerDeploymentFor(testService),
		routeTableFor(map[string]string{"alice": "alice-dev"}),
	}

	return newUpClient(append(objects, extra...)...)
}

func bobJoining() UpOptions {
	opts := testUpOptions()
	opts.RoutingKey = "bob"
	opts.TargetNamespace = "bob-dev"

	return opts
}

// The whole point of the route table: a second developer joins instead of being refused.
func TestUp_JoinsAServiceSomeoneElseAlreadyDiverted(t *testing.T) {
	c, k8s := alreadyDiverted()

	require.NoError(t, c.Up(context.Background(), bobJoining()))

	require.Equal(t, map[string]string{"alice": "alice-dev", "bob": "bob-dev"}, getRouteTable(t, k8s))
}

// Joining changes nothing in the shared namespace: the router, the baseline and the
// selector are already serving everyone else.
func TestUp_JoiningLeavesTheSharedNamespaceAlone(t *testing.T) {
	c, k8s := alreadyDiverted()
	before := getDeployment(t, k8s, RouterDeploymentName(testService))

	require.NoError(t, c.Up(context.Background(), bobJoining()))

	require.Equal(t, before, getDeployment(t, k8s, RouterDeploymentName(testService)))
	require.Equal(t, map[string]string{routerPodLabel: testService}, getService(t, k8s, testService).Spec.Selector)
}

// Callers reconnected through the router when the first developer diverted, so rolling the
// baseline again would disrupt the shared namespace for nothing.
func TestUp_JoiningDoesNotRestartTheBaseline(t *testing.T) {
	c, k8s := alreadyDiverted(baselineWorkload())

	require.NoError(t, c.Up(context.Background(), bobJoining()))

	require.Empty(t, restartedAt(t, getDeployment(t, k8s, testService)))
}

func TestUp_JoiningStillMirrorsSharedServices(t *testing.T) {
	c, k8s := alreadyDiverted(plainService("catalog", testNamespace))
	opts := bobJoining()

	require.NoError(t, c.Up(context.Background(), opts))

	mirror, err := k8s.CoreV1().Services(opts.TargetNamespace).Get(context.Background(), "catalog", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "catalog.staging.svc.cluster.local", mirror.Spec.ExternalName)
}

// Two developers cannot share a routing key: the router would have no way to tell their
// traffic apart.
func TestUp_RefusesARoutingKeySomeoneElseIsUsing(t *testing.T) {
	c, k8s := alreadyDiverted()
	opts := testUpOptions()
	opts.RoutingKey = "alice"
	opts.TargetNamespace = "bob-dev"

	err := c.Up(context.Background(), opts)

	require.ErrorContains(t, err, `routing key "alice" is already diverting`)
	require.ErrorContains(t, err, "alice-dev")
	require.Equal(t, map[string]string{"alice": "alice-dev"}, getRouteTable(t, k8s))
}

// Re-running the same divert is a no-op rather than a duplicate-key error.
func TestUp_RejoiningWithTheSameKeyAndNamespaceSucceeds(t *testing.T) {
	c, k8s := alreadyDiverted()
	opts := testUpOptions()
	opts.RoutingKey = "alice"
	opts.TargetNamespace = "alice-dev"

	require.NoError(t, c.Up(context.Background(), opts))

	require.Equal(t, map[string]string{"alice": "alice-dev"}, getRouteTable(t, k8s))
}

// A router that reads its routes from somewhere this version cannot write would accept the
// key and silently never route it.
func TestUp_RefusesToJoinADivertWithNoRouteTable(t *testing.T) {
	c, _ := newUpClient(swappedService(), baselineServiceFor(testService), routerDeploymentFor(testService))

	err := c.Up(context.Background(), bobJoining())

	require.ErrorContains(t, err, "no route table")
	require.ErrorContains(t, err, "--all")
}

func TestUp_JoiningWarnsAboutTheReloadDelay(t *testing.T) {
	logger := &testLogger{}
	c, _ := alreadyDiverted()
	c.logger = logger

	require.NoError(t, c.Up(context.Background(), bobJoining()))

	require.Len(t, logger.warnings, 1)
	require.Contains(t, logger.warnings[0], "reloads its route table")
}
