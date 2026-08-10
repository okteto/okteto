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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

// restartedAtAnnotation is stamped on the baseline workload's pod template to trigger a
// rolling restart, the same mechanism `kubectl rollout restart` uses.
const restartedAtAnnotation = "divert.okteto.com/restarted-at"

// restartBaseline makes every existing caller of the diverted service reconnect through the
// router.
//
// Swapping a Service selector only affects connections opened afterwards: kube-proxy picks
// a backend when a connection is established, and conntrack pins that choice for the life of
// the connection. A caller pooling a connection from before the swap therefore keeps
// reaching the baseline, forever if its traffic is steady.
//
// Those connections all terminate at the pods behind the original selector, so replacing
// those pods tears down every one of them at once — one rolling restart, however many
// callers there are, and without needing to know who they are. This has to run after the
// swap has taken effect: do it earlier and the reconnections simply re-pin to new baseline
// pods.
//
// It never fails the divert. By this point the divert is live and correct; a caller that
// did not reconnect is a convergence problem the developer can finish by hand, and the
// warning says how.
func (c *Client) restartBaseline(ctx context.Context, opts UpOptions, originalSelector map[string]string) {
	workloads, err := c.baselineDeployments(ctx, opts.SharedNamespace, originalSelector)
	if err != nil {
		c.warnRestartIncomplete(opts, fmt.Sprintf("they could not be listed: %s", err))
		return
	}

	if len(workloads) == 0 {
		// The workload may be a StatefulSet, or anything else this does not know how to
		// roll. Saying so is better than silently leaving callers pinned.
		c.warnRestartIncomplete(opts, "no Deployment in the namespace matches the service's original selector")
		return
	}

	for _, name := range workloads {
		if err := c.rollingRestart(ctx, opts.SharedNamespace, name); err != nil {
			c.warnRestartIncomplete(opts, fmt.Sprintf("restarting %s failed: %s", name, err))
			return
		}

		c.logger.Infof("restarted %s/%s so its callers reconnect through the router", opts.SharedNamespace, name)

		if err := c.waitForRollout(ctx, opts.SharedNamespace, name, opts.ReadinessTimeout); err != nil {
			c.warnRestartIncomplete(opts, fmt.Sprintf("%s did not finish restarting: %s", name, err))
			return
		}
	}
}

func (c *Client) warnRestartIncomplete(opts UpOptions, reason string) {
	c.logger.Warning(
		"the divert is live, but callers holding a connection opened before it may keep reaching the baseline: %s. "+
			"Restart the workload behind %s by hand to force them to reconnect: kubectl rollout restart -n %s deployment/<name>",
		reason, opts.Service, opts.SharedNamespace,
	)
}

// baselineDeployments finds the Deployments whose pods the original selector was pointing at.
func (c *Client) baselineDeployments(ctx context.Context, namespace string, selector map[string]string) ([]string, error) {
	if len(selector) == 0 {
		return nil, nil
	}

	list, err := c.k8s.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var names []string
	for i := range list.Items {
		deployment := &list.Items[i]

		// Never roll the router: it is not where the stale connections terminate, and
		// restarting it mid-divert would drop the traffic that is already going through it.
		if _, managed := deployment.Labels[managedServiceLabel]; managed {
			continue
		}

		if selectorMatchesTemplate(selector, deployment.Spec.Template.Labels) {
			names = append(names, deployment.Name)
		}
	}

	return names, nil
}

// selectorMatchesTemplate reports whether a Service selector would select the pods a
// workload creates.
func selectorMatchesTemplate(selector, podLabels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}

	for key, value := range selector {
		if podLabels[key] != value {
			return false
		}
	}

	return true
}

// rollingRestart stamps the pod template so the workload's own update strategy replaces its
// pods gracefully, rather than deleting them all at once.
func (c *Client) rollingRestart(ctx context.Context, namespace, name string) error {
	patch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]string{
						restartedAtAnnotation: time.Now().UTC().Format(time.RFC3339),
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}

	_, err = c.k8s.AppsV1().Deployments(namespace).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	return err
}

// waitForDeploymentRollout blocks until every pod of a workload has been replaced, so that
// `divert up` does not return while callers are still holding connections to old pods.
func (c *Client) waitForDeploymentRollout(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, readinessPollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		deployment, err := c.k8s.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return rolloutComplete(deployment), nil
	})
}

func rolloutComplete(deployment *appsv1.Deployment) bool {
	// A status that predates the patch describes the previous pods, not the new ones.
	if deployment.Generation > deployment.Status.ObservedGeneration {
		return false
	}

	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}

	return deployment.Status.UpdatedReplicas == desired &&
		deployment.Status.ReadyReplicas == desired &&
		deployment.Status.Replicas == desired
}
