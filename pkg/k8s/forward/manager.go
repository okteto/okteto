// Copyright 2020 The Okteto Authors
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
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"

	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const devName = "okteto-development"

// PortForwardManager keeps a list of all the active port forwards
type PortForwardManager struct {
	stopped        bool
	ports          map[int]model.Forward
	services       map[string]struct{}
	activeDev      *active
	activeServices map[string]*active
	ctx            context.Context
	restConfig     *rest.Config
	client         kubernetes.Interface
}

type active struct {
	readyChan chan struct{}
	stopChan  chan struct{}
	out       *bytes.Buffer
	err       error
}

func (a *active) stop() {
	if a != nil && a.stopChan != nil {
		close(a.stopChan)
		a.stopChan = nil
	}
}

func (a *active) closeReady() {
	if a != nil && a.readyChan != nil {
		close(a.readyChan)
		a.readyChan = nil
	}
}

func (a *active) error() error {
	if a != nil {
		return a.err
	}

	return nil
}

// NewPortForwardManager initializes a new instance
func NewPortForwardManager(ctx context.Context, restConfig *rest.Config, c kubernetes.Interface) *PortForwardManager {
	return &PortForwardManager{
		ctx:        ctx,
		ports:      make(map[int]model.Forward),
		services:   make(map[string]struct{}),
		restConfig: restConfig,
		client:     c,
	}
}

// Add initializes a port forward
func (p *PortForwardManager) Add(f model.Forward) error {
	if _, ok := p.ports[f.Local]; ok {
		return fmt.Errorf("port %d is listed multiple times, please check your configuration", f.Local)
	}

	if !model.IsPortAvailable(f.Local) {
		return fmt.Errorf("port %d is already in use in your local machine, please check your configuration", f.Local)
	}

	p.ports[f.Local] = f
	if f.Service {
		p.services[f.ServiceName] = struct{}{}
	}

	return nil
}

// AddReverse is not implemented
func (p *PortForwardManager) AddReverse(_ model.Reverse) error {
	return fmt.Errorf("not implemented")
}

// Start starts all the port forwarders to the development container
func (p *PortForwardManager) Start(devPod, namespace string) error {
	p.stopped = false
	a, devPF, err := p.buildForwarderToDevPod(namespace, devPod)
	if err != nil {
		return fmt.Errorf("failed to forward ports to development container: %w", err)
	}

	p.activeDev = a
	go func() {
		err := devPF.ForwardPorts()
		if err != nil {
			log.Debugf("port forwarding to dev pod finished with errors: %s", err)
			p.activeDev.closeReady()
			p.activeDev.err = err
		}
	}()

	p.activeServices = map[string]*active{}
	for svc := range p.services {
		go p.forwardService(p.ctx, namespace, svc)
	}

	log.Debugf("waiting port forwarding to finish")
	<-p.activeDev.readyChan

	if err := p.activeDev.error(); err != nil {
		return err
	}

	log.Debugf("all port-forwards are connected")
	return nil
}

// Stop stops all the port forwarders
func (p *PortForwardManager) Stop() {
	p.stopped = true
	p.activeDev.stop()

	for _, a := range p.activeServices {
		a.stop()
	}

	p.activeServices = nil
	p.activeDev = nil
	log.Infof("stopped k8s forwarder")
}

func (p *PortForwardManager) buildForwarderToDevPod(namespace, pod string) (*active, *portforward.PortForwarder, error) {
	ports := []string{}
	for _, f := range p.ports {
		if !f.Service {
			ports = append(ports, fmt.Sprintf("%d:%d", f.Local, f.Remote))
		}
	}

	return p.buildForwarder(devName, namespace, pod, ports)
}

func (p *PortForwardManager) buildForwarder(name, namespace, pod string, ports []string) (*active, *portforward.PortForwarder, error) {
	addresses := getListenAddresses()
	dialer, err := p.buildDialer(namespace, pod)
	if err != nil {
		return nil, nil, err
	}

	a := &active{
		readyChan: make(chan struct{}, 1),
		stopChan:  make(chan struct{}, 1),
		out:       new(bytes.Buffer),
	}

	pf, err := portforward.NewOnAddresses(
		dialer,
		addresses,
		ports,
		a.stopChan,
		a.readyChan,
		ioutil.Discard,
		a.out)

	if err != nil {
		return nil, nil, err
	}

	return a, pf, nil
}

func (p *PortForwardManager) buildForwarderToService(ctx context.Context, namespace, service string) (*active, *portforward.PortForwarder, error) {
	svc, err := services.Get(ctx, namespace, service, p.client)
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
	return p.buildForwarder(service, pod.GetNamespace(), pod.GetName(), ports)
}

func getServicePorts(service string, forwards map[int]model.Forward) []string {
	ports := []string{}
	for _, f := range forwards {
		if f.Service && f.ServiceName == service {
			remote := f.Remote
			ports = append(ports, fmt.Sprintf("%d:%d", f.Local, remote))
		}
	}

	return ports
}

func (p *PortForwardManager) buildDialer(namespace, pod string) (httpstream.Dialer, error) {
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

	return spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url), nil
}

func (p *PortForwardManager) forwardService(ctx context.Context, namespace, service string) {
	t := time.NewTicker(3 * time.Second)

	for {
		if p.stopped {
			return
		}

		log.Debugf("forwarding ports for service/%s", service)
		a, pf, err := p.buildForwarderToService(ctx, namespace, service)
		if err != nil {
			log.Debugf("failed to forward ports to service/%s: %s", service, err)
			<-t.C
			continue
		}

		err = pf.ForwardPorts()
		if err != nil {
			log.Debugf("port forwarding to service/%s finished with errors: %s", service, err)
			a.stop()
		} else {
			log.Debugf("port forwarding to service/%s finished", service)
		}

		<-t.C
	}
}

func getListenAddresses() []string {
	addresses := []string{"localhost"}
	extraAddress := os.Getenv("OKTETO_ADDRESS")
	if len(extraAddress) > 0 {
		addresses = append(addresses, extraAddress)
	}

	return addresses
}
