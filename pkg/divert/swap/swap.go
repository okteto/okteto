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

// Package swap performs the Kubernetes choreography behind a mesh-free divert: it puts a
// router in front of a Service in the shared namespace by repointing that Service's
// selector, and takes it back out again.
//
// The package deliberately knows nothing about the Okteto manifest. Both the
// `okteto divert up`/`down` commands and any future manifest-driven driver drive it
// through the same API.
//
// # Recovering a divert by hand
//
// Everything teardown needs is stored on the Service itself, so a divert can always be
// undone without the CLI, from any machine with cluster access:
//
//	NS=staging SVC=api
//
//	# What the selector was before the swap.
//	SELECTOR=$(kubectl get svc "$SVC" -n "$NS" \
//	  -o jsonpath='{.metadata.annotations.divert\.okteto\.com/original-selector}')
//
//	# Put it back. This has to be a JSON patch: a merge patch would union the two
//	# selectors, and a Service selecting on both label sets matches no pods at all.
//	kubectl patch svc "$SVC" -n "$NS" --type=json -p "[
//	  {\"op\":\"replace\",\"path\":\"/spec/selector\",\"value\":$SELECTOR},
//	  {\"op\":\"remove\",\"path\":\"/metadata/annotations/divert.okteto.com~1original-selector\"},
//	  {\"op\":\"remove\",\"path\":\"/metadata/annotations/divert.okteto.com~1baseline-service\"},
//	  {\"op\":\"remove\",\"path\":\"/metadata/annotations/divert.okteto.com~1router-deployment\"}]"
//
//	# Then the router and the baseline, in that order.
//	kubectl delete deploy,svc -n "$NS" -l divert.okteto.com/service="$SVC"
//
// The `~1` sequences are JSON Pointer escaping for the `/` in the annotation keys.
package swap

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apiv1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	// originalSelectorAnnotation holds the JSON-encoded selector the Service had before it
	// was pointed at the router.
	originalSelectorAnnotation = "divert.okteto.com/original-selector"

	// baselineServiceAnnotation holds the name of the Service that inherited that selector.
	baselineServiceAnnotation = "divert.okteto.com/baseline-service"

	// routerDeploymentAnnotation holds the name of the router Deployment serving the swap.
	routerDeploymentAnnotation = "divert.okteto.com/router-deployment"

	// managedServiceLabel marks every object this package creates with the service it
	// belongs to. Teardown uses it to find leftovers from an interrupted bring-up, whose
	// describing annotations were never written.
	managedServiceLabel = "divert.okteto.com/service"

	// routerPodLabel is what the swapped Service selects on to reach the router pods.
	routerPodLabel = "divert-router"

	// BaselineServiceSuffix and RouterDeploymentSuffix derive the names of the objects a
	// swap creates. Names are also persisted in annotations, so teardown never has to
	// recompute them and stays correct across a change to this scheme.
	baselineServiceSuffix  = "-baseline"
	routerDeploymentSuffix = "-divert-router"
	routesConfigMapSuffix  = "-divert-routes"

	// routesVolumeName and routesMountPath are where the router finds its route table.
	routesVolumeName = "divert-routes"
	routesMountPath  = "/etc/okteto/divert-routes"

	// readinessPollInterval is how often bring-up checks whether the router is serving.
	readinessPollInterval = 2 * time.Second

	// endpointsPollInterval is how often bring-up checks whether the swap has taken effect.
	endpointsPollInterval = 500 * time.Millisecond
)

// Logger is the minimal logging surface this package needs.
type Logger interface {
	Infof(format string, args ...interface{})
	Warning(format string, args ...interface{})
}

// readinessWaiter blocks until a Deployment has the requested number of ready replicas.
// It is a field on Client so that tests can drive bring-up without a controller running.
type readinessWaiter func(ctx context.Context, namespace, name string, replicas int32, timeout time.Duration) error

// endpointsWaiter blocks until a Service's endpoints actually point at the router.
type endpointsWaiter func(ctx context.Context, namespace, service string, timeout time.Duration) error

// rolloutWaiter blocks until every pod of a workload has been replaced.
type rolloutWaiter func(ctx context.Context, namespace, name string, timeout time.Duration) error

// Client performs swap operations against a cluster.
type Client struct {
	k8s              kubernetes.Interface
	logger           Logger
	waitForReady     readinessWaiter
	waitForEndpoints endpointsWaiter
	waitForRollout   rolloutWaiter
}

// NewClient returns a Client backed by the given Kubernetes client.
func NewClient(k8s kubernetes.Interface, logger Logger) *Client {
	c := &Client{
		k8s:    k8s,
		logger: logger,
	}
	c.waitForReady = c.waitForDeployment
	c.waitForEndpoints = c.waitForServiceEndpoints
	c.waitForRollout = c.waitForDeploymentRollout

	return c
}

// waitForDeployment polls until every replica of the router is ready. Bring-up waits for
// all of them rather than the first: the swap points everyone's traffic at this Deployment,
// so it should be at full capacity before that happens, not one pod into a rollout.
func (c *Client) waitForDeployment(ctx context.Context, namespace, name string, replicas int32, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, readinessPollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		d, err := c.k8s.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return d.Status.ReadyReplicas >= replicas, nil
	})
}

// waitForServiceEndpoints blocks until the Service's own endpoints name a router pod.
//
// Changing a selector does not move any traffic by itself. The EndpointSlice controller has
// to recompute the Service's endpoints first, and until it does, callers keep reaching the
// pods the Service used to select. Returning from `divert up` before that happens reports a
// divert that is not yet in effect, which looks exactly like a divert that does not work.
func (c *Client) waitForServiceEndpoints(ctx context.Context, namespace, service string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, endpointsPollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		routerIPs, err := c.routerPodIPs(ctx, namespace, service)
		if err != nil {
			return false, err
		}
		if len(routerIPs) == 0 {
			return false, nil
		}

		slices, err := c.k8s.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", discoveryv1.LabelServiceName, service),
		})
		if err != nil {
			return false, err
		}

		return endpointsReachAnyOf(slices.Items, routerIPs), nil
	})
}

func endpointsReachAnyOf(slices []discoveryv1.EndpointSlice, addresses map[string]bool) bool {
	for i := range slices {
		for _, endpoint := range slices[i].Endpoints {
			if endpoint.Conditions.Ready != nil && !*endpoint.Conditions.Ready {
				continue
			}

			for _, address := range endpoint.Addresses {
				if addresses[address] {
					return true
				}
			}
		}
	}

	return false
}

func (c *Client) routerPodIPs(ctx context.Context, namespace, service string) (map[string]bool, error) {
	pods, err := c.k8s.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", routerPodLabel, service),
	})
	if err != nil {
		return nil, err
	}

	ips := make(map[string]bool, len(pods.Items))
	for i := range pods.Items {
		if ip := pods.Items[i].Status.PodIP; ip != "" {
			ips[ip] = true
		}
	}

	return ips, nil
}

// BaselineServiceName is the name of the Service that keeps serving the real workload
// while the original Service points at the router.
func BaselineServiceName(service string) string {
	return service + baselineServiceSuffix
}

// RouterDeploymentName is the name of the Deployment running the router for a service.
func RouterDeploymentName(service string) string {
	return service + routerDeploymentSuffix
}

// state is everything teardown needs, read back from the Service's own annotations. It is
// stored on the Service rather than held by the CLI so that a divert can always be undone
// from the object itself, with no CLI, no laptop and no route table involved.
type state struct {
	OriginalSelector map[string]string
	BaselineService  string
	RouterDeployment string
}

// readState reports whether a Service is currently swapped, and if so what it was before.
//
// A Service carrying the selector annotation but nothing decodable is an error rather than
// "not swapped": silently treating it as untouched would leave shared traffic pointed at a
// router forever, which is the one outcome teardown exists to prevent.
func readState(svc *apiv1.Service) (state, bool, error) {
	raw, ok := svc.Annotations[originalSelectorAnnotation]
	if !ok {
		return state{}, false, nil
	}

	var selector map[string]string
	if err := json.Unmarshal([]byte(raw), &selector); err != nil {
		return state{}, false, fmt.Errorf(
			"service %s/%s has an unreadable %s annotation (%q): restore its selector by hand and remove the annotation: %w",
			svc.Namespace, svc.Name, originalSelectorAnnotation, raw, err,
		)
	}

	if len(selector) == 0 {
		return state{}, false, fmt.Errorf(
			"service %s/%s has an empty %s annotation: restore its selector by hand and remove the annotation",
			svc.Namespace, svc.Name, originalSelectorAnnotation,
		)
	}

	return state{
		OriginalSelector: selector,
		BaselineService:  svc.Annotations[baselineServiceAnnotation],
		RouterDeployment: svc.Annotations[routerDeploymentAnnotation],
	}, true, nil
}

// managedByServiceSelector is the label selector matching everything a swap created for a
// given service.
func managedByServiceSelector(service string) string {
	return fmt.Sprintf("%s=%s", managedServiceLabel, service)
}
