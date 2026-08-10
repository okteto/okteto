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
	"strings"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// DownOptions describes a divert to tear down.
type DownOptions struct {
	// Service is the diverted Service in the shared namespace.
	Service string

	// SharedNamespace holds that Service.
	SharedNamespace string

	// RoutingKey is the one route to remove. Other developers' routes are left alone, and
	// the router only comes down when the last of them goes.
	RoutingKey string

	// All tears the whole divert down regardless of who else is using it. It is also what
	// recovers a divert whose route table is missing or unreadable.
	All bool
}

func (o *DownOptions) validate() error {
	if o.Service == "" {
		return fmt.Errorf("the service to stop diverting is required")
	}
	if o.SharedNamespace == "" {
		return fmt.Errorf("the shared namespace is required")
	}
	if o.All && o.RoutingKey != "" {
		return fmt.Errorf("a routing key cannot be given when tearing the whole divert down")
	}

	return nil
}

// Down removes a divert.
//
// With a routing key it removes only that route, so one developer leaving does not take
// everyone else's divert with them; the router and the baseline come down with the last
// route. Without one, or with All, it tears the whole thing down — which is what recovery
// and rollback need.
//
// It is idempotent: running it against a service that was never diverted, or twice in a
// row, succeeds without touching anything.
func (c *Client) Down(ctx context.Context, opts DownOptions) error {
	if err := opts.validate(); err != nil {
		return err
	}

	if opts.All || opts.RoutingKey == "" {
		return c.tearDown(ctx, opts.Service, opts.SharedNamespace)
	}

	routes, err := c.routes(ctx, opts.SharedNamespace, opts.Service)
	if err != nil {
		return err
	}

	// Nothing registered: either not diverted at all, or left over from a run that never
	// got as far as writing a route. Either way, clean up whatever is there.
	if len(routes) == 0 {
		return c.tearDown(ctx, opts.Service, opts.SharedNamespace)
	}

	if _, mine := routes[opts.RoutingKey]; !mine {
		return fmt.Errorf(
			"service %s/%s is not diverted with the routing key %q but with %s: pass --key with one of those, or --all to tear the whole divert down",
			opts.SharedNamespace, opts.Service, opts.RoutingKey, strings.Join(sortedRouteKeys(routes), ", "),
		)
	}

	if len(routes) == 1 {
		return c.tearDown(ctx, opts.Service, opts.SharedNamespace)
	}

	if err := c.removeRoute(ctx, opts.SharedNamespace, opts.Service, opts.RoutingKey); err != nil {
		return err
	}

	delete(routes, opts.RoutingKey)
	c.logger.Infof(
		"stopped diverting %s/%s with the routing key %q; it stays diverted for %s",
		opts.SharedNamespace, opts.Service, opts.RoutingKey, strings.Join(sortedRouteKeys(routes), ", "),
	)

	return nil
}

// tearDown restores the shared namespace completely.
//
// The order is what matters. Restoring the selector comes first, because that is the step
// that puts shared traffic back on the real pods; only once nothing is routed through the
// router is it safe to delete the router, and only then the baseline it was forwarding to.
// Deleting in any other order blackholes the shared service for everyone using it.
func (c *Client) tearDown(ctx context.Context, service, sharedNamespace string) error {
	if err := c.restoreSelector(ctx, service, sharedNamespace); err != nil {
		return err
	}

	return c.deleteSwapResources(ctx, service, sharedNamespace)
}

// restoreSelector puts the original selector back on the Service and drops the annotations
// describing the swap.
func (c *Client) restoreSelector(ctx context.Context, service, sharedNamespace string) error {
	services := c.k8s.CoreV1().Services(sharedNamespace)

	// Retry on conflict rather than failing: the shared namespace is, by definition, a
	// place where something else may be writing to this object at the same time.
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		svc, err := services.Get(ctx, service, metav1.GetOptions{})
		if err != nil {
			return err
		}

		swapState, swapped, err := readState(svc)
		if err != nil {
			return err
		}
		if !swapped {
			c.logger.Infof("service %s/%s is not diverted, leaving its selector untouched", sharedNamespace, service)
			return nil
		}

		updated := svc.DeepCopy()
		updated.Spec.Selector = swapState.OriginalSelector
		delete(updated.Annotations, originalSelectorAnnotation)
		delete(updated.Annotations, baselineServiceAnnotation)
		delete(updated.Annotations, routerDeploymentAnnotation)

		if _, err := services.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
			return err
		}

		c.logger.Infof("restored the original selector of service %s/%s", sharedNamespace, service)
		return nil
	})

	if k8sErrors.IsNotFound(err) {
		// Nothing to restore. The router and baseline may still be around, and the caller
		// goes on to clean them up.
		c.logger.Infof("service %s/%s not found, nothing to restore", sharedNamespace, service)
		return nil
	}
	if err != nil {
		return fmt.Errorf("error restoring the selector of service %s/%s: %w", sharedNamespace, service, err)
	}

	return nil
}

// deleteSwapResources removes the router Deployment, the baseline Service and the route table.
//
// They are found by label rather than by the names recorded in the annotations, which is
// what makes teardown able to clean up after a bring-up that was interrupted before it ever
// got as far as writing those annotations.
func (c *Client) deleteSwapResources(ctx context.Context, service, sharedNamespace string) error {
	listOptions := metav1.ListOptions{LabelSelector: managedByServiceSelector(service)}

	// The router goes first: it forwards to the baseline, so removing the baseline while
	// the router is still up would turn any in-flight request into a 502.
	if err := c.deleteRouterDeployments(ctx, sharedNamespace, listOptions); err != nil {
		return err
	}

	if err := c.deleteDisruptionBudgets(ctx, sharedNamespace, listOptions); err != nil {
		return err
	}

	if err := c.deleteBaselineServices(ctx, sharedNamespace, listOptions); err != nil {
		return err
	}

	// The route table goes last: the router mounts it.
	return c.deleteRouteTables(ctx, sharedNamespace, listOptions)
}

// deleteDisruptionBudgets removes the router's budget. A cluster without the policy API
// never had one, so a failure to list is reported rather than blocking teardown of
// everything else.
func (c *Client) deleteDisruptionBudgets(ctx context.Context, namespace string, listOptions metav1.ListOptions) error {
	budgets := c.k8s.PolicyV1().PodDisruptionBudgets(namespace)

	list, err := budgets.List(ctx, listOptions)
	if err != nil {
		c.logger.Warning("could not list divert disruption budgets in namespace %s: %s", namespace, err)
		return nil
	}

	for i := range list.Items {
		name := list.Items[i].Name
		if err := budgets.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !k8sErrors.IsNotFound(err) {
			return fmt.Errorf("error deleting divert disruption budget %s/%s: %w", namespace, name, err)
		}
		c.logger.Infof("deleted divert disruption budget %s/%s", namespace, name)
	}

	return nil
}

func (c *Client) deleteRouteTables(ctx context.Context, namespace string, listOptions metav1.ListOptions) error {
	configMaps := c.k8s.CoreV1().ConfigMaps(namespace)

	list, err := configMaps.List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("error listing divert route tables in namespace %s: %w", namespace, err)
	}

	for i := range list.Items {
		name := list.Items[i].Name
		if err := configMaps.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !k8sErrors.IsNotFound(err) {
			return fmt.Errorf("error deleting divert route table %s/%s: %w", namespace, name, err)
		}
		c.logger.Infof("deleted divert route table %s/%s", namespace, name)
	}

	return nil
}

func (c *Client) deleteRouterDeployments(ctx context.Context, namespace string, listOptions metav1.ListOptions) error {
	deployments := c.k8s.AppsV1().Deployments(namespace)

	list, err := deployments.List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("error listing divert router deployments in namespace %s: %w", namespace, err)
	}

	for i := range list.Items {
		name := list.Items[i].Name
		if err := deployments.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !k8sErrors.IsNotFound(err) {
			return fmt.Errorf("error deleting divert router deployment %s/%s: %w", namespace, name, err)
		}
		c.logger.Infof("deleted divert router deployment %s/%s", namespace, name)
	}

	return nil
}

func (c *Client) deleteBaselineServices(ctx context.Context, namespace string, listOptions metav1.ListOptions) error {
	services := c.k8s.CoreV1().Services(namespace)

	list, err := services.List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("error listing divert baseline services in namespace %s: %w", namespace, err)
	}

	for i := range list.Items {
		name := list.Items[i].Name
		if err := services.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !k8sErrors.IsNotFound(err) {
			return fmt.Errorf("error deleting divert baseline service %s/%s: %w", namespace, name, err)
		}
		c.logger.Infof("deleted divert baseline service %s/%s", namespace, name)
	}

	return nil
}
