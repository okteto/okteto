// Copyright 2023 The Okteto Authors
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

package forward

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/services"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForwardManager keeps a list of all the active port forwards
type PortForwardManager struct {
	mu                sync.RWMutex
	ctx               context.Context
	client            kubernetes.Interface
	ports             map[int]forward.Forward
	services          map[string]struct{}
	activeDev         *active
	activeDevPF       devPortForwarder
	activeServices    map[string]*active
	restConfig        *rest.Config
	iface             string
	namespace         string
	stopped           bool
	stopTimeout       time.Duration
	buildDevForwarder func(context.Context, string, string) (*active, devPortForwarder, error)
}

type active struct {
	readyChan chan struct{}
	stopChan  chan struct{}
	doneChan  chan struct{}
	out       *bytes.Buffer
	mu        sync.RWMutex
	stopOnce  sync.Once
	doneOnce  sync.Once
	cancel    context.CancelFunc
	err       error
}

type devPortForwarder interface {
	ForwardPorts() error
	GetPorts() ([]portforward.ForwardedPort, error)
}

func (a *active) stop() {
	if a == nil {
		return
	}
	a.stopOnce.Do(func() {
		if a.cancel != nil {
			a.cancel()
		}
		if a.stopChan != nil {
			close(a.stopChan)
		}
	})
}

func (a *active) error() error {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.err
}

func (a *active) finish(err error) {
	if a == nil {
		return
	}
	a.doneOnce.Do(func() {
		a.mu.Lock()
		a.err = err
		a.mu.Unlock()
		if a.doneChan != nil {
			close(a.doneChan)
		}
	})
}

// NewPortForwardManager initializes a new instance
func NewPortForwardManager(ctx context.Context, iface string, restConfig *rest.Config, c kubernetes.Interface, namespace string) *PortForwardManager {
	return &PortForwardManager{
		ctx:         ctx,
		iface:       iface,
		ports:       make(map[int]forward.Forward),
		services:    make(map[string]struct{}),
		restConfig:  restConfig,
		client:      c,
		namespace:   namespace,
		stopTimeout: time.Second,
	}
}

// Add initializes a port forward
func (p *PortForwardManager) Add(f forward.Forward) error {
	if _, ok := p.ports[f.Local]; ok {
		return fmt.Errorf("port %d is listed multiple times, please check your configuration", f.Local)
	}

	// A local port of zero asks client-go to allocate and hold an available
	// port when the forward starts. Probing it here would only introduce a
	// bind/close race and cannot reserve the chosen port.
	if f.Local != 0 && !model.IsPortAvailable(p.iface, f.Local) {
		maxSystemPorts := 1024
		if f.Local <= maxSystemPorts {
			os := runtime.GOOS
			switch os {
			case "darwin":
				return fmt.Errorf("local port %d is privileged. Define 'interface: 0.0.0.0' in your okteto manifest and try again", f.Local)
			case "linux":
				return fmt.Errorf("local port %d is privileged. Try running \"sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/okteto\" and try again", f.Local)
			}
		}
		return fmt.Errorf("local port %d is already in-use in your local machine", f.Local)
	}

	p.ports[f.Local] = f
	if f.Service {
		p.services[f.ServiceName] = struct{}{}
	}

	return nil
}

// StartGlobalForwarding is not implemented
func (*PortForwardManager) StartGlobalForwarding() error {
	return fmt.Errorf("not implemented")
}

// AddReverse is not implemented
func (*PortForwardManager) AddReverse(_ model.Reverse) error {
	return fmt.Errorf("not implemented")
}

// Start starts all the port forwarders to the development container
func (p *PortForwardManager) Start(devPod, namespace string) error {
	return p.StartContext(p.ctx, devPod, namespace)
}

// StartContext starts all port forwarders while bounding the initial API
// upgrade and readiness wait with ctx. Once ready, the forward remains tied
// to the manager's lifetime context until Stop is called.
func (p *PortForwardManager) StartContext(ctx context.Context, devPod, namespace string) error {
	if ctx == nil {
		return fmt.Errorf("k8s port-forward start context is nil")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("k8s port-forward start cancelled: %w", err)
	}

	p.mu.Lock()
	p.stopped = false
	p.mu.Unlock()

	attemptCtx, cancelAttempt := context.WithCancel(p.ctx)
	cancelled := make(chan struct{})
	stopStartupCancellation := context.AfterFunc(ctx, func() {
		cancelAttempt()
		close(cancelled)
	})
	var disarmOnce sync.Once
	disarmStartupCancellation := func() {
		disarmOnce.Do(func() {
			if !stopStartupCancellation() {
				<-cancelled
			}
		})
	}
	defer disarmStartupCancellation()

	var a *active
	var devPF devPortForwarder
	var err error
	if p.buildDevForwarder != nil {
		a, devPF, err = p.buildDevForwarder(attemptCtx, namespace, devPod)
	} else {
		a, devPF, err = p.buildForwarderToDevPod(attemptCtx, namespace, devPod)
	}
	if err != nil {
		disarmStartupCancellation()
		cancelAttempt()
		return fmt.Errorf("failed to k8s forward to development container: %w", err)
	}
	if a == nil || devPF == nil || a.readyChan == nil || a.doneChan == nil {
		disarmStartupCancellation()
		cancelAttempt()
		return fmt.Errorf("failed to k8s forward to development container: invalid forwarder state")
	}
	if a.cancel == nil {
		a.cancel = cancelAttempt
	}

	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		a.stop()
		disarmStartupCancellation()
		cancelAttempt()
		return fmt.Errorf("k8s port-forward stopped during startup")
	}
	previous := p.activeDev
	p.activeDev = a
	p.activeDevPF = devPF
	p.mu.Unlock()
	if previous != nil && previous != a {
		previous.stop()
	}

	go func() {
		defer cancelAttempt()
		err := devPF.ForwardPorts()
		if err != nil {
			oktetoLog.Infof("k8s forwarding to dev pod finished with errors: %s", err)
		}
		a.finish(err)
		p.clearActive(a)
	}()

	select {
	case <-a.readyChan:
		select {
		case <-a.doneChan:
			if err := a.error(); err != nil {
				return err
			}
			return fmt.Errorf("k8s port-forward stopped before it became usable")
		default:
		}
	case <-a.doneChan:
		disarmStartupCancellation()
		if err := a.error(); err != nil {
			return err
		}
		return fmt.Errorf("k8s port-forward stopped before it became ready")
	case <-attemptCtx.Done():
		a.stop()
		disarmStartupCancellation()
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("k8s port-forward start cancelled: %w", err)
		}
		cause := p.ctx.Err()
		if cause == nil {
			cause = context.Canceled
		}
		return fmt.Errorf("k8s port-forward start cancelled: %w", cause)
	}
	disarmStartupCancellation()

	p.mu.Lock()
	if p.stopped || p.activeDev != a || p.ctx.Err() != nil || ctx.Err() != nil {
		p.mu.Unlock()
		a.stop()
		return fmt.Errorf("k8s port-forward stopped during startup")
	}
	p.activeServices = map[string]*active{}
	p.mu.Unlock()

	for svc := range p.services {
		go p.forwardService(p.ctx, namespace, svc)
	}

	oktetoLog.Infof("all k8s port-forwards are connected")
	return nil
}

func (p *PortForwardManager) clearActive(a *active) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.activeDev == a {
		p.activeDev = nil
		p.activeDevPF = nil
	}
}

// ForwardedPorts returns the concrete local ports held by the active forward.
// It is primarily useful when a caller requested local port zero.
func (p *PortForwardManager) ForwardedPorts() ([]portforward.ForwardedPort, error) {
	p.mu.RLock()
	a := p.activeDev
	pf := p.activeDevPF
	p.mu.RUnlock()
	if a == nil || pf == nil {
		return nil, fmt.Errorf("k8s port-forward is not started")
	}
	select {
	case <-a.readyChan:
	default:
		return nil, fmt.Errorf("k8s port-forward listeners are not ready")
	}
	select {
	case <-a.doneChan:
		if err := a.error(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("k8s port-forward is stopped")
	default:
	}

	return pf.GetPorts()
}

// Stop stops all the port forwarders
func (p *PortForwardManager) Stop() {
	p.mu.Lock()
	p.stopped = true
	a := p.activeDev
	services := p.activeServices
	p.activeServices = nil
	p.activeDev = nil
	p.activeDevPF = nil
	p.mu.Unlock()

	a.stop()
	if a != nil && a.doneChan != nil && p.stopTimeout > 0 {
		select {
		case <-a.doneChan:
		case <-time.After(p.stopTimeout):
			oktetoLog.Infof("timed out waiting for k8s forwarder to stop")
		}
	}
	for _, a := range services {
		a.stop()
	}
	oktetoLog.Infof("stopped k8s forwarder")
}

func (fm *PortForwardManager) TransformLabelsToServiceName(f forward.Forward) (forward.Forward, error) {
	serviceName, err := fm.GetServiceNameByLabel(fm.namespace, f.Labels)
	if err != nil {
		return f, err
	}
	f.ServiceName = serviceName
	return f, nil
}

func (p *PortForwardManager) buildForwarderToDevPod(ctx context.Context, namespace, pod string) (*active, *portforward.PortForwarder, error) {
	ports := []string{}
	for _, f := range p.ports {
		if !f.Service {
			ports = append(ports, fmt.Sprintf("%d:%d", f.Local, f.Remote))
		}
	}

	return p.buildForwarder(ctx, namespace, pod, ports)
}

func (p *PortForwardManager) buildForwarder(ctx context.Context, namespace, pod string, ports []string) (*active, *portforward.PortForwarder, error) {
	forwardCtx, cancel := context.WithCancel(ctx)
	dialer, err := p.buildDialer(forwardCtx, namespace, pod)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	a := &active{
		readyChan: make(chan struct{}, 1),
		stopChan:  make(chan struct{}, 1),
		doneChan:  make(chan struct{}),
		out:       new(bytes.Buffer),
		cancel:    cancel,
	}

	pf, err := portforward.NewOnAddresses(
		dialer,
		[]string{p.iface},
		ports,
		a.stopChan,
		a.readyChan,
		io.Discard,
		a.out)

	if err != nil {
		cancel()
		return nil, nil, err
	}

	return a, pf, nil
}

func (p *PortForwardManager) buildForwarderToService(ctx context.Context, namespace, service string) (*active, *portforward.PortForwarder, error) {
	svc, err := services.Get(ctx, service, namespace, p.client)
	if err != nil {
		return nil, nil, err
	}

	if len(svc.Spec.Ports) == 0 {
		return nil, nil, fmt.Errorf("service/%s doesn't have ports", svc.GetName())
	}

	pod, err := pods.GetBySelector(ctx, namespace, svc.Spec.Selector, p.client)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get pod mapped to service/%s: %w", svc.GetName(), err)
	}

	ports := getServicePorts(svc.GetName(), p.ports)
	return p.buildForwarder(ctx, pod.GetNamespace(), pod.GetName(), ports)
}

func getServicePorts(service string, forwards map[int]forward.Forward) []string {
	ports := []string{}
	for _, f := range forwards {
		if f.Service && f.ServiceName == service {
			remote := f.Remote
			ports = append(ports, fmt.Sprintf("%d:%d", f.Local, remote))
		}
	}

	return ports
}

func (p *PortForwardManager) buildDialer(ctx context.Context, namespace, pod string) (httpstream.Dialer, error) {
	url := p.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("portforward").URL()

	if p.restConfig == nil {
		return nil, fmt.Errorf("restConfig is nil")
	}

	transport, upgrader, err := spdy.RoundTripperFor(p.restConfig)
	if err != nil {
		return nil, err
	}

	return &contextDialer{
		ctx:      ctx,
		client:   &http.Client{Transport: transport},
		upgrader: upgrader,
		method:   http.MethodPost,
		url:      url,
	}, nil
}

type contextDialer struct {
	ctx      context.Context
	client   *http.Client
	upgrader spdy.Upgrader
	method   string
	url      *url.URL
}

func (d *contextDialer) Dial(protocols ...string) (httpstream.Connection, string, error) {
	request, err := http.NewRequestWithContext(d.ctx, d.method, d.url.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("create port-forward request: %w", err)
	}
	return spdy.Negotiate(d.upgrader, d.client, request, protocols...)
}

func (p *PortForwardManager) forwardService(ctx context.Context, namespace, service string) {
	t := time.NewTicker(3 * time.Second)

	for {
		p.mu.RLock()
		stopped := p.stopped
		p.mu.RUnlock()
		if stopped {
			return
		}

		oktetoLog.Infof("k8s forwarding ports for service/%s", service)
		a, pf, err := p.buildForwarderToService(ctx, namespace, service)
		if err != nil {
			oktetoLog.Infof("failed to k8s forward ports to service/%s: %s", service, err)
			<-t.C
			continue
		}

		if err := pf.ForwardPorts(); err != nil {
			oktetoLog.Infof("k8s forwarding to service/%s finished with errors: %s", service, err)
			a.stop()
		} else {
			oktetoLog.Infof("k8s forwarding to service/%s finished", service)
		}

		<-t.C
	}
}

func (p *PortForwardManager) GetServiceNameByLabel(namespace string, labelsMap map[string]string) (string, error) {
	labelsString := labels.TransformLabelsToSelector(labelsMap)
	serviceName, err := services.GetServiceNameByLabel(p.ctx, namespace, p.client, labelsString)
	if err != nil {
		return "", err
	}
	return serviceName, nil
}
