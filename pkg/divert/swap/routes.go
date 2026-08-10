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
	"encoding/json"
	"fmt"
	"sort"

	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// RoutesConfigMapName is the ConfigMap holding a service's route table: one data key per
// routing key, whose value is the namespace that key diverts to.
//
// It is mounted into the router rather than read through the API, which is what lets the
// router run with no ServiceAccount and no RBAC of its own.
func RoutesConfigMapName(service string) string {
	return service + routesConfigMapSuffix
}

// routes returns the current route table. A missing ConfigMap is an empty table, not an
// error: it is the normal state of a service that is not diverted.
func (c *Client) routes(ctx context.Context, namespace, service string) (map[string]string, error) {
	cm, err := c.k8s.CoreV1().ConfigMaps(namespace).Get(ctx, RoutesConfigMapName(service), metav1.GetOptions{})
	if k8sErrors.IsNotFound(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error reading the route table of %s/%s: %w", namespace, service, err)
	}

	if cm.Data == nil {
		return map[string]string{}, nil
	}

	return cm.Data, nil
}

// addRoute registers one routing key, creating the table if this is the first developer.
//
// The update is a patch of a single key rather than a write of the whole map, so two
// developers joining the same divert at the same time cannot overwrite each other's route.
func (c *Client) addRoute(ctx context.Context, namespace, service, key, targetNamespace string) error {
	configMaps := c.k8s.CoreV1().ConfigMaps(namespace)

	patch, err := json.Marshal(map[string]interface{}{
		"data": map[string]string{key: targetNamespace},
	})
	if err != nil {
		return err
	}

	_, err = configMaps.Patch(ctx, RoutesConfigMapName(service), types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err == nil {
		c.logger.Infof("registered routing key %q for service %s/%s", key, namespace, service)
		return nil
	}
	if !k8sErrors.IsNotFound(err) {
		return fmt.Errorf("error registering routing key %q for %s/%s: %w", key, namespace, service, err)
	}

	created := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RoutesConfigMapName(service),
			Namespace: namespace,
			Labels:    map[string]string{managedServiceLabel: service},
		},
		Data: map[string]string{key: targetNamespace},
	}

	if _, err := configMaps.Create(ctx, created, metav1.CreateOptions{}); err != nil {
		// Someone else created the table between the patch and the create. Their table is
		// as good as ours, so add the key to it instead of failing.
		if k8sErrors.IsAlreadyExists(err) {
			_, err = configMaps.Patch(ctx, RoutesConfigMapName(service), types.StrategicMergePatchType, patch, metav1.PatchOptions{})
		}
		if err != nil {
			return fmt.Errorf("error creating the route table for %s/%s: %w", namespace, service, err)
		}
	}

	c.logger.Infof("registered routing key %q for service %s/%s", key, namespace, service)
	return nil
}

// removeRoute deregisters one routing key, leaving every other developer's alone.
func (c *Client) removeRoute(ctx context.Context, namespace, service, key string) error {
	// A JSON merge patch deletes a key when its value is null. Patching one key rather than
	// writing the whole map means a concurrent join is not lost.
	patch, err := json.Marshal(map[string]interface{}{
		"data": map[string]interface{}{key: nil},
	})
	if err != nil {
		return err
	}

	_, err = c.k8s.CoreV1().ConfigMaps(namespace).Patch(ctx, RoutesConfigMapName(service), types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return fmt.Errorf("error deregistering routing key %q for %s/%s: %w", key, namespace, service, err)
	}

	c.logger.Infof("deregistered routing key %q for service %s/%s", key, namespace, service)
	return nil
}

// sortedRouteKeys lists the routing keys currently registered, for error messages that tell
// the developer what is actually there.
func sortedRouteKeys(routes map[string]string) []string {
	keys := make([]string, 0, len(routes))
	for key := range routes {
		keys = append(keys, fmt.Sprintf("%q", key))
	}
	sort.Strings(keys)

	return keys
}
