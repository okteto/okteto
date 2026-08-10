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

	"github.com/okteto/okteto/pkg/divert/router"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
)

const (
	// maxNameLength is the RFC 1035 label limit that Service names must respect. Derived
	// names are validated against it rather than truncated: truncating would let two long
	// service names collapse onto the same baseline.
	maxNameLength = 63

	// defaultReplicas keeps the router available while it sits in the path of everyone's
	// traffic to the diverted service.
	defaultReplicas = 2

	// defaultReadinessTimeout bounds the wait before the selector swap.
	defaultReadinessTimeout = 2 * time.Minute

	// defaultRouterListenPort is used only when the Service addresses its pods by port
	// name, which leaves the router free to pick the number the name resolves to.
	defaultRouterListenPort = 8080

	// routerHealthPort and its alternate serve readiness. The alternate exists for the one
	// case where the service being diverted already listens on the first choice.
	routerHealthPort          = 9191
	alternateRouterHealthPort = 9192

	// lowestUnprivilegedPort is the first port bindable without NET_BIND_SERVICE.
	lowestUnprivilegedPort = 1024

	// terminationGracePeriod must exceed the router's own shutdown timeout, or the kubelet
	// sends SIGKILL while requests are still draining.
	terminationGracePeriod int64 = 30

	routerContainerName = "divert-router"
	healthProbePath     = "/healthz"
	clusterDomain       = "svc.cluster.local"
)

// UpOptions describes a divert to bring up.
type UpOptions struct {
	// Service is the name of the Service in the shared namespace to put a router in front of.
	Service string

	// SharedNamespace holds that Service.
	SharedNamespace string

	// TargetNamespace is the developer namespace holding their copy of the service.
	TargetNamespace string

	// RoutingKey is the value of the `divert` baggage member that selects the copy.
	RoutingKey string

	// RouterImage is the image running the router. It is the Okteto CLI image: the router
	// is the CLI's own hidden `divert-router` command.
	RouterImage string

	// ReadinessTimeout bounds the wait for the router before the selector is swapped.
	ReadinessTimeout time.Duration

	// Replicas is the router's replica count.
	Replicas int32

	// SkipBaselineRestart leaves the diverted workload alone.
	//
	// By default bring-up rolls it, because that is the only way to make callers holding a
	// connection from before the swap reconnect through the router. That restart briefly
	// disrupts the baseline for everyone in the shared namespace, so it can be turned off
	// when that trade is not worth it — at the cost of the divert not applying to those
	// callers until they reconnect by themselves.
	SkipBaselineRestart bool
}

func (o *UpOptions) setDefaults() {
	if o.Replicas == 0 {
		o.Replicas = defaultReplicas
	}
	if o.ReadinessTimeout == 0 {
		o.ReadinessTimeout = defaultReadinessTimeout
	}
}

func (o *UpOptions) validate() error {
	if o.Service == "" {
		return fmt.Errorf("the service to divert is required")
	}
	if o.SharedNamespace == "" {
		return fmt.Errorf("the shared namespace is required")
	}
	if o.TargetNamespace == "" {
		return fmt.Errorf("the target namespace is required")
	}
	if o.RoutingKey == "" {
		return fmt.Errorf("the routing key is required")
	}
	if o.RouterImage == "" {
		return fmt.Errorf("the router image is required")
	}
	if o.SharedNamespace == o.TargetNamespace {
		return fmt.Errorf("the shared namespace and the target namespace cannot both be %q: a service cannot be diverted to itself", o.SharedNamespace)
	}

	for _, name := range []string{BaselineServiceName(o.Service), RouterDeploymentName(o.Service)} {
		if len(name) > maxNameLength {
			return fmt.Errorf("service %q is too long to divert: it would need a %q object, over the %d character limit", o.Service, name, maxNameLength)
		}
	}

	return nil
}

// plan is everything derived from the Service before anything is created.
type plan struct {
	baselineName string
	routerName   string
	baselineHost string
	ports        []router.PortConfig
	healthPort   int32
}

// lowestListenPort reports whether any proxied port is privileged, which decides whether the
// router needs NET_BIND_SERVICE.
func (p plan) hasPrivilegedPort() bool {
	for _, port := range p.ports {
		if port.Listen < lowestUnprivilegedPort {
			return true
		}
	}

	return false
}

// Up puts a router in front of a service in the shared namespace.
//
// The selector swap is the last thing to happen and only happens once the router is
// serving, because it is the point at which every caller of the shared service starts
// going through the router. Anything that fails before then is rolled back, so an
// interrupted bring-up never leaves the shared Service pointing at a router that is not
// there.
func (c *Client) Up(ctx context.Context, opts UpOptions) error {
	opts.setDefaults()
	if err := opts.validate(); err != nil {
		return err
	}

	svc, err := c.k8s.CoreV1().Services(opts.SharedNamespace).Get(ctx, opts.Service, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error reading service %s/%s: %w", opts.SharedNamespace, opts.Service, err)
	}

	// A service that is already diverted is joined rather than refused. One router serves
	// every developer working on that service; they are told apart by their routing key, so
	// the only thing worth rejecting is a duplicate key (doc §7).
	if _, diverted, err := readState(svc); err != nil {
		return err
	} else if diverted {
		return c.join(ctx, opts)
	}

	p, err := planSwap(svc)
	if err != nil {
		return err
	}

	// The route table is created before the router, so the router's mount is already
	// populated when it starts and the first divert works immediately.
	if err := c.addRoute(ctx, opts.SharedNamespace, opts.Service, opts.RoutingKey, opts.TargetNamespace); err != nil {
		return err
	}

	if err := c.createBaseline(ctx, opts, svc, p); err != nil {
		c.rollback(ctx, opts)
		return err
	}

	if err := c.protectRouter(ctx, opts, p); err != nil {
		c.rollback(ctx, opts)
		return err
	}

	if err := c.startRouter(ctx, opts, p); err != nil {
		c.rollback(ctx, opts)
		return err
	}

	originalSelector, err := c.swapSelector(ctx, opts, p)
	if err != nil {
		c.rollback(ctx, opts)
		return err
	}

	// The selector is only the instruction. Wait for the cluster to have acted on it before
	// telling anyone the divert is live.
	if err := c.waitForEndpoints(ctx, opts.SharedNamespace, opts.Service, opts.ReadinessTimeout); err != nil {
		c.rollback(ctx, opts)
		return fmt.Errorf(
			"service %s/%s never started resolving to the router within %s: %w",
			opts.SharedNamespace, opts.Service, opts.ReadinessTimeout, err,
		)
	}

	c.logger.Infof(
		"service %s/%s is diverted: requests carrying 'baggage: divert=%s' now reach namespace %s",
		opts.SharedNamespace, opts.Service, opts.RoutingKey, opts.TargetNamespace,
	)

	// Existing callers are still pinned to the pods they were already talking to, so the
	// divert is not yet in effect for them. Restarting the baseline is what completes it.
	// It runs here, after the endpoints have converged: any earlier and the reconnections
	// would simply re-pin to new baseline pods.
	if opts.SkipBaselineRestart {
		c.logger.Warning(
			"skipping the baseline restart: callers holding a connection opened before the divert will keep " +
				"reaching the baseline until they reconnect on their own",
		)
	} else {
		c.restartBaseline(ctx, opts, originalSelector)
	}

	// Name resolution comes last, and a failure here does not roll back. The divert is
	// already live and working; undoing it because a convenience Service could not be
	// created would be the worse outcome. It is still reported as an error, because the
	// developer's copy will not be able to call the rest of the shared namespace.
	if err := c.mirrorSharedServices(ctx, opts); err != nil {
		return fmt.Errorf(
			"service %s/%s is diverted, but the shared services could not be mirrored into %s, so your pods may fail to resolve them: %w",
			opts.SharedNamespace, opts.Service, opts.TargetNamespace, err,
		)
	}

	return nil
}

// planSwap validates that a Service can be diverted and derives everything the swap needs.
func planSwap(svc *apiv1.Service) (plan, error) {
	if _, _, err := readState(svc); err != nil {
		return plan{}, err
	}

	if svc.Spec.Type == apiv1.ServiceTypeExternalName {
		return plan{}, fmt.Errorf("service %s/%s is an ExternalName service: it has no pods to put a router in front of", svc.Namespace, svc.Name)
	}
	if svc.Spec.ClusterIP == apiv1.ClusterIPNone {
		return plan{}, fmt.Errorf("service %s/%s is headless: callers resolve its pods directly, so a router in front of it would never be reached", svc.Namespace, svc.Name)
	}
	if len(svc.Spec.Selector) == 0 {
		return plan{}, fmt.Errorf("service %s/%s has no selector: its endpoints are managed by hand, so there is no selector to swap", svc.Namespace, svc.Name)
	}
	if len(svc.Spec.Ports) == 0 {
		return plan{}, fmt.Errorf("service %s/%s exposes no ports", svc.Namespace, svc.Name)
	}

	ports, err := planPorts(svc)
	if err != nil {
		return plan{}, err
	}

	listenPorts := make([]int32, 0, len(ports))
	for _, port := range ports {
		listenPorts = append(listenPorts, int32(port.Listen))
	}

	baselineName := BaselineServiceName(svc.Name)

	return plan{
		baselineName: baselineName,
		routerName:   RouterDeploymentName(svc.Name),
		baselineHost: fmt.Sprintf("%s.%s.%s", baselineName, svc.Namespace, clusterDomain),
		ports:        ports,
		healthPort:   healthPortFor(listenPorts),
	}, nil
}

// planPorts decides which port the router binds for each port of the Service, so that every
// one of them keeps reaching it unchanged after the swap.
//
// The two kinds of targetPort behave differently and the difference drives the whole
// function. A numeric one pins the router to that exact number: the Service will forward
// there and nowhere else. A named one does not — the name is resolved against the router
// pod's own container ports, so the router may listen wherever it likes and declare the name
// on that port.
//
// The pinned ports are therefore assigned first and the named ones fill in around them.
// Doing it in one pass would let a named port be handed a number a later numeric port needs,
// and the router would fail to bind its second listener.
func planPorts(svc *apiv1.Service) ([]router.PortConfig, error) {
	ports := make([]router.PortConfig, len(svc.Spec.Ports))
	taken := make(map[int32]bool, len(svc.Spec.Ports))

	for i, port := range svc.Spec.Ports {
		if port.Protocol != "" && port.Protocol != apiv1.ProtocolTCP {
			return nil, fmt.Errorf(
				"service %s/%s exposes the %s port %q: only TCP can be header-routed",
				svc.Namespace, svc.Name, port.Protocol, port.Name,
			)
		}

		if namedTargetPort(port) != "" {
			continue
		}

		listen := numericTargetPort(port)
		if taken[listen] {
			return nil, fmt.Errorf(
				"service %s/%s targets port %d from more than one of its ports, so a router cannot tell them apart",
				svc.Namespace, svc.Name, listen,
			)
		}
		taken[listen] = true
		ports[i] = router.PortConfig{Listen: int(listen), Service: int(port.Port)}
	}

	next := int32(defaultRouterListenPort)
	for i, port := range svc.Spec.Ports {
		name := namedTargetPort(port)
		if name == "" {
			continue
		}

		for taken[next] {
			next++
		}
		taken[next] = true
		ports[i] = router.PortConfig{Name: name, Listen: int(next), Service: int(port.Port)}
	}

	return ports, nil
}

// namedTargetPort is the container port name a Service port addresses, empty when it
// addresses a number instead.
func namedTargetPort(port apiv1.ServicePort) string {
	if port.TargetPort.Type == intstr.String {
		return port.TargetPort.StrVal
	}

	return ""
}

// numericTargetPort is the port number a Service port addresses. An unset targetPort
// defaults to the service port.
func numericTargetPort(port apiv1.ServicePort) int32 {
	if port.TargetPort.IntVal != 0 {
		return port.TargetPort.IntVal
	}

	return port.Port
}

// healthPortFor picks a readiness port that does not collide with anything being proxied.
// Readiness has to live on its own port so that every proxied port stays transparent for
// every path, including the application's own health endpoints.
func healthPortFor(listenPorts []int32) int32 {
	for _, candidate := range []int32{routerHealthPort, alternateRouterHealthPort} {
		if !containsPort(listenPorts, candidate) {
			return candidate
		}
	}

	// Both candidates are in use by the service itself. Walk up until something is free;
	// the router pod runs nothing else, so the only conflicts possible are these.
	port := int32(alternateRouterHealthPort)
	for containsPort(listenPorts, port) {
		port++
	}

	return port
}

func containsPort(ports []int32, port int32) bool {
	for _, p := range ports {
		if p == port {
			return true
		}
	}

	return false
}

// createBaseline stands up the Service that keeps the real pods reachable once the original
// Service stops selecting them.
func (c *Client) createBaseline(ctx context.Context, opts UpOptions, svc *apiv1.Service, p plan) error {
	baseline := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.baselineName,
			Namespace: opts.SharedNamespace,
			Labels:    map[string]string{managedServiceLabel: opts.Service},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeClusterIP,
			// The original selector moves here verbatim, along with the ports exactly as
			// they were: a renamed port would break any caller addressing it by name.
			Selector:              svc.Spec.Selector,
			Ports:                 svc.Spec.Ports,
			SessionAffinity:       svc.Spec.SessionAffinity,
			SessionAffinityConfig: svc.Spec.SessionAffinityConfig,
		},
	}

	if _, err := c.k8s.CoreV1().Services(opts.SharedNamespace).Create(ctx, baseline, metav1.CreateOptions{}); err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			return fmt.Errorf(
				"service %s/%s already exists but %s is not diverted: run 'okteto divert down' to clean up a previous attempt",
				opts.SharedNamespace, p.baselineName, opts.Service,
			)
		}
		return fmt.Errorf("error creating baseline service %s/%s: %w", opts.SharedNamespace, p.baselineName, err)
	}

	c.logger.Infof("created baseline service %s/%s", opts.SharedNamespace, p.baselineName)
	return nil
}

// protectRouter stops a node drain from taking every router replica at once.
//
// From the moment the selector is swapped, this Deployment is in the path of everyone's
// traffic to the shared service. A cluster autoscaler or an upgrade evicting both replicas
// together would black-hole it, and nothing about the divert would explain why.
//
// A failure here is not fatal: a cluster without the policy API, or a developer without
// permission to create budgets, should still get a working divert. It is reported instead.
func (c *Client) protectRouter(ctx context.Context, opts UpOptions, p plan) error {
	minAvailable := intstr.FromInt32(1)
	budget := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.routerName,
			Namespace: opts.SharedNamespace,
			Labels:    map[string]string{managedServiceLabel: opts.Service},
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{routerPodLabel: opts.Service},
			},
		},
	}

	_, err := c.k8s.PolicyV1().PodDisruptionBudgets(opts.SharedNamespace).Create(ctx, budget, metav1.CreateOptions{})
	if err != nil && !k8sErrors.IsAlreadyExists(err) {
		c.logger.Warning(
			"could not create a disruption budget for the router of %s/%s, so a node drain could take every replica at once: %s",
			opts.SharedNamespace, opts.Service, err,
		)
	}

	return nil
}

// startRouter creates the router Deployment and waits until it is serving.
func (c *Client) startRouter(ctx context.Context, opts UpOptions, p plan) error {
	deployment := routerDeployment(opts, p)

	if _, err := c.k8s.AppsV1().Deployments(opts.SharedNamespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			return fmt.Errorf(
				"deployment %s/%s already exists but %s is not diverted: run 'okteto divert down' to clean up a previous attempt",
				opts.SharedNamespace, p.routerName, opts.Service,
			)
		}
		return fmt.Errorf("error creating router deployment %s/%s: %w", opts.SharedNamespace, p.routerName, err)
	}

	c.logger.Infof("created router deployment %s/%s, waiting for it to be ready", opts.SharedNamespace, p.routerName)

	if err := c.waitForReady(ctx, opts.SharedNamespace, p.routerName, opts.Replicas, opts.ReadinessTimeout); err != nil {
		return fmt.Errorf(
			"router %s/%s did not become ready within %s, so %s was left untouched: %w",
			opts.SharedNamespace, p.routerName, opts.ReadinessTimeout, opts.Service, err,
		)
	}

	c.logger.Infof("router %s/%s is ready", opts.SharedNamespace, p.routerName)
	return nil
}

// swapSelector is the cutover: from here on, callers of the shared service go through the
// router. It runs last and only after the router is serving.
// It returns the selector it recorded, which is what the baseline restart needs to find the
// workload the stale connections terminate at.
func (c *Client) swapSelector(ctx context.Context, opts UpOptions, p plan) (map[string]string, error) {
	services := c.k8s.CoreV1().Services(opts.SharedNamespace)

	var recorded map[string]string

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		svc, err := services.Get(ctx, opts.Service, metav1.GetOptions{})
		if err != nil {
			return err
		}

		recorded = svc.Spec.Selector

		// The selector is read inside the retry, not before it: a conflict means something
		// else wrote to this object, and what we saw earlier may no longer be current.
		// Recording a stale selector would make teardown restore the wrong thing.
		originalSelector, err := json.Marshal(svc.Spec.Selector)
		if err != nil {
			return err
		}

		updated := svc.DeepCopy()
		if updated.Annotations == nil {
			updated.Annotations = map[string]string{}
		}
		updated.Annotations[originalSelectorAnnotation] = string(originalSelector)
		updated.Annotations[baselineServiceAnnotation] = p.baselineName
		updated.Annotations[routerDeploymentAnnotation] = p.routerName
		updated.Spec.Selector = map[string]string{routerPodLabel: opts.Service}

		_, err = services.Update(ctx, updated, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("error pointing service %s/%s at the router: %w", opts.SharedNamespace, opts.Service, err)
	}

	c.logger.Infof("service %s/%s now points at the router", opts.SharedNamespace, opts.Service)
	return recorded, nil
}

// rollback removes whatever the bring-up managed to create. It never reports failure: the
// error that triggered the rollback is the one the caller needs to see.
func (c *Client) rollback(ctx context.Context, opts UpOptions) {
	c.logger.Infof("rolling back the divert of %s/%s", opts.SharedNamespace, opts.Service)

	// Full teardown, not just a delete of what was created. Bring-up can fail after the
	// selector has already been swapped, and deleting the router while the Service still
	// selects it would leave that Service with no endpoints at all — an outage for
	// everyone, caused by the cleanup rather than by the original failure. Down restores
	// the selector first when there is one to restore, and is a no-op when there is not.
	if err := c.Down(ctx, DownOptions{Service: opts.Service, SharedNamespace: opts.SharedNamespace}); err != nil {
		c.logger.Warning(
			"could not fully roll back the divert of %s/%s, run 'okteto divert down --service %s --from %s' to finish cleaning up: %s",
			opts.SharedNamespace, opts.Service, opts.Service, opts.SharedNamespace, err,
		)
	}
}

func routerDeployment(opts UpOptions, p plan) *appsv1.Deployment {
	replicas := opts.Replicas
	grace := terminationGracePeriod

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.routerName,
			Namespace: opts.SharedNamespace,
			Labels:    map[string]string{managedServiceLabel: opts.Service},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{routerPodLabel: opts.Service},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						routerPodLabel:      opts.Service,
						managedServiceLabel: opts.Service,
					},
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: &grace,
					Containers:                    []apiv1.Container{routerContainer(opts, p)},
					Volumes:                       []apiv1.Volume{routesVolume(opts.Service)},
					TopologySpreadConstraints:     routerTopologySpread(opts.Service),
				},
			},
		},
	}
}

func routerContainer(opts UpOptions, p plan) apiv1.Container {
	return apiv1.Container{
		Name:    routerContainerName,
		Image:   opts.RouterImage,
		Command: []string{"okteto", "divert-router"},
		Ports:   routerContainerPorts(p),
		Env:     routerEnv(opts, p),
		VolumeMounts: []apiv1.VolumeMount{{
			Name:      routesVolumeName,
			MountPath: routesMountPath,
			ReadOnly:  true,
		}},
		ReadinessProbe:  healthProbe(p.healthPort),
		LivenessProbe:   healthProbe(p.healthPort),
		SecurityContext: routerSecurityContext(p),
		Resources:       routerResources(),
	}
}

// routerContainerPorts declares every port the router serves.
//
// The names matter as much as the numbers: a Service that addresses its pods by port name
// resolves that name against the pod's container ports, so a named port has to keep its
// name here or the swapped Service will not reach the router at all.
func routerContainerPorts(p plan) []apiv1.ContainerPort {
	ports := make([]apiv1.ContainerPort, 0, len(p.ports)+1)
	for _, port := range p.ports {
		ports = append(ports, apiv1.ContainerPort{
			Name:          port.Name,
			ContainerPort: int32(port.Listen),
		})
	}

	return append(ports, apiv1.ContainerPort{ContainerPort: p.healthPort})
}

// routerTopologySpread spreads the router's replicas across nodes, so that losing one node
// does not take the shared service down with it.
//
// ScheduleAnyway rather than DoNotSchedule: a single-node cluster is exactly where this gets
// evaluated first, and refusing to schedule there would turn an availability preference into
// a hard failure.
func routerTopologySpread(service string) []apiv1.TopologySpreadConstraint {
	return []apiv1.TopologySpreadConstraint{{
		MaxSkew:           1,
		TopologyKey:       apiv1.LabelHostname,
		WhenUnsatisfiable: apiv1.ScheduleAnyway,
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{routerPodLabel: service},
		},
	}}
}

// routerEnv is the router's whole configuration. The names come from the router package
// itself rather than being spelled out here, so the two sides cannot drift.
func routerEnv(opts UpOptions, p plan) []apiv1.EnvVar {
	return []apiv1.EnvVar{
		{Name: router.EnvServiceName, Value: opts.Service},
		{Name: router.EnvSharedNamespace, Value: opts.SharedNamespace},
		{Name: router.EnvBaselineHost, Value: p.baselineHost},
		{Name: router.EnvPorts, Value: encodePorts(p.ports)},
		{Name: router.EnvHealthPort, Value: fmt.Sprint(p.healthPort)},
		{Name: router.EnvRoutesDir, Value: routesMountPath},
	}
}

// encodePorts serialises the port list for the router. A marshalling failure is impossible
// for this shape, so an empty list is the safe fallback: the router refuses to start on it
// rather than serving something half-configured.
func encodePorts(ports []router.PortConfig) string {
	encoded, err := json.Marshal(ports)
	if err != nil {
		return ""
	}

	return string(encoded)
}

func healthProbe(healthPort int32) *apiv1.Probe {
	return &apiv1.Probe{
		ProbeHandler: apiv1.ProbeHandler{
			HTTPGet: &apiv1.HTTPGetAction{
				Path: healthProbePath,
				Port: intstr.FromInt32(healthPort),
			},
		},
		PeriodSeconds:    2,
		FailureThreshold: 3,
	}
}

// routerSecurityContext drops everything the router does not need. NET_BIND_SERVICE is
// added only when the Service addresses its pods on a privileged port, which the router
// then has to bind. It is the one capability the restricted Pod Security Standard allows
// to be added back.
func routerSecurityContext(p plan) *apiv1.SecurityContext {
	allowPrivilegeEscalation := false
	capabilities := &apiv1.Capabilities{Drop: []apiv1.Capability{"ALL"}}
	if p.hasPrivilegedPort() {
		capabilities.Add = []apiv1.Capability{"NET_BIND_SERVICE"}
	}

	return &apiv1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		Capabilities:             capabilities,
		SeccompProfile:           &apiv1.SeccompProfile{Type: apiv1.SeccompProfileTypeRuntimeDefault},
	}
}

func routerResources() apiv1.ResourceRequirements {
	return apiv1.ResourceRequirements{
		Requests: apiv1.ResourceList{
			apiv1.ResourceCPU:    resource.MustParse("10m"),
			apiv1.ResourceMemory: resource.MustParse("32Mi"),
		},
		Limits: apiv1.ResourceList{
			apiv1.ResourceMemory: resource.MustParse("128Mi"),
		},
	}
}

// routesVolume mounts the route table into the router.
//
// It is Optional so that a missing table never stops the router from starting: an empty
// table simply sends everything to the baseline, which is the correct behaviour, whereas a
// pod stuck in ContainerCreating would take the shared service down.
func routesVolume(service string) apiv1.Volume {
	optional := true

	return apiv1.Volume{
		Name: routesVolumeName,
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{Name: RoutesConfigMapName(service)},
				Optional:             &optional,
			},
		},
	}
}

// join adds a routing key to a service someone else has already diverted.
//
// Nothing about the shared namespace changes: the router, the baseline and the selector are
// already in place and serving every other developer. Only the route table grows, and only
// the joining developer's own namespace gets its service mirrors. In particular the baseline
// is not restarted — callers reconnected through the router when the first divert happened.
func (c *Client) join(ctx context.Context, opts UpOptions) error {
	routes, err := c.routes(ctx, opts.SharedNamespace, opts.Service)
	if err != nil {
		return err
	}

	// A divert with no route table is one this version did not create. Its router reads
	// routes from somewhere this cannot reach, so adding a key would look like it worked
	// and silently do nothing.
	if len(routes) == 0 {
		return fmt.Errorf(
			"service %s/%s is diverted but has no route table: tear it down with 'okteto divert down --service %s --from %s --all' and divert it again",
			opts.SharedNamespace, opts.Service, opts.Service, opts.SharedNamespace,
		)
	}

	if existing, taken := routes[opts.RoutingKey]; taken && existing != opts.TargetNamespace {
		return fmt.Errorf(
			"routing key %q is already diverting %s/%s to namespace %q: pick another key with --key",
			opts.RoutingKey, opts.SharedNamespace, opts.Service, existing,
		)
	}

	if err := c.addRoute(ctx, opts.SharedNamespace, opts.Service, opts.RoutingKey, opts.TargetNamespace); err != nil {
		return err
	}

	c.logger.Infof(
		"joined the divert of %s/%s: requests carrying 'baggage: divert=%s' now reach namespace %s",
		opts.SharedNamespace, opts.Service, opts.RoutingKey, opts.TargetNamespace,
	)

	// The router already running picks the new key up from its mounted table rather than
	// through the API, so it is not instant.
	c.logger.Warning(
		"the router reloads its route table from disk, so %q can take up to a minute to start routing",
		opts.RoutingKey,
	)

	if err := c.mirrorSharedServices(ctx, opts); err != nil {
		return fmt.Errorf(
			"joined the divert of %s/%s, but the shared services could not be mirrored into %s, so your pods may fail to resolve them: %w",
			opts.SharedNamespace, opts.Service, opts.TargetNamespace, err,
		)
	}

	return nil
}
