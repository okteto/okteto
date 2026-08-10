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
)

func TestRoutes_AMissingTableIsAnEmptyTable(t *testing.T) {
	c, _ := newTestClient()

	routes, err := c.routes(context.Background(), testNamespace, testService)

	require.NoError(t, err)
	require.Empty(t, routes)
}

func TestAddRoute_CreatesTheTableForTheFirstDeveloper(t *testing.T) {
	c, k8s := newTestClient()

	require.NoError(t, c.addRoute(context.Background(), testNamespace, testService, "alice", "alice-dev"))

	require.Equal(t, map[string]string{"alice": "alice-dev"}, getRouteTable(t, k8s))
}

func TestAddRoute_LabelsTheTableSoTeardownFindsIt(t *testing.T) {
	c, k8s := newTestClient()
	require.NoError(t, c.addRoute(context.Background(), testNamespace, testService, "alice", "alice-dev"))

	cm, err := k8s.CoreV1().ConfigMaps(testNamespace).Get(context.Background(), RoutesConfigMapName(testService), metav1.GetOptions{})
	require.NoError(t, err)

	require.Equal(t, testService, cm.Labels[managedServiceLabel])
}

// Adding a key patches just that key, so a developer joining cannot overwrite the routes
// already in the table.
func TestAddRoute_KeepsTheRoutesAlreadyThere(t *testing.T) {
	c, k8s := newTestClient(routeTableFor(map[string]string{"alice": "alice-dev"}))

	require.NoError(t, c.addRoute(context.Background(), testNamespace, testService, "bob", "bob-dev"))

	require.Equal(t, map[string]string{"alice": "alice-dev", "bob": "bob-dev"}, getRouteTable(t, k8s))
}

func TestAddRoute_OverwritesItsOwnKey(t *testing.T) {
	c, k8s := newTestClient(routeTableFor(map[string]string{"alice": "old-namespace"}))

	require.NoError(t, c.addRoute(context.Background(), testNamespace, testService, "alice", "alice-dev"))

	require.Equal(t, map[string]string{"alice": "alice-dev"}, getRouteTable(t, k8s))
}

func TestRemoveRoute_LeavesTheOtherRoutesAlone(t *testing.T) {
	c, k8s := newTestClient(routeTableFor(map[string]string{"alice": "alice-dev", "bob": "bob-dev"}))

	require.NoError(t, c.removeRoute(context.Background(), testNamespace, testService, "alice"))

	require.Equal(t, map[string]string{"bob": "bob-dev"}, getRouteTable(t, k8s))
}

func TestRemoveRoute_ToleratesAMissingTable(t *testing.T) {
	c, _ := newTestClient()

	require.NoError(t, c.removeRoute(context.Background(), testNamespace, testService, "alice"))
}

func TestSortedRouteKeys_IsStableAndQuoted(t *testing.T) {
	keys := sortedRouteKeys(map[string]string{"bob": "bob-dev", "alice": "alice-dev"})

	require.Equal(t, []string{`"alice"`, `"bob"`}, keys)
}
