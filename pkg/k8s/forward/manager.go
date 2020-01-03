package forward

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForwardManager keeps a list of all the active port forwards
type PortForwardManager struct {
	portForwards map[int]int
	ctx          context.Context
	restConfig   *rest.Config
	client       *kubernetes.Clientset
	err          error
	stopChan     chan struct{}
	out          *bytes.Buffer
}

// NewPortForwardManager initializes a new instance
func NewPortForwardManager(ctx context.Context, restConfig *rest.Config, c *kubernetes.Clientset) *PortForwardManager {
	return &PortForwardManager{
		ctx:          ctx,
		portForwards: make(map[int]int),
		restConfig:   restConfig,
		client:       c,
		err:          nil,
		out:          new(bytes.Buffer),
	}
}

// Add initializes a port forward
func (p *PortForwardManager) Add(f model.Forward) error {
	if _, ok := p.portForwards[f.Local]; ok {
		return fmt.Errorf("port %d is already taken, please check your configuration", f.Local)
	}

	p.portForwards[f.Local] = f.Remote
	return nil
}

// Start starts all the port forwarders
func (p *PortForwardManager) Start(pod, namespace string) error {
	ready := make(chan struct{}, 1)
	go func(r chan struct{}) {
		if err := p.forward(namespace, pod, ready); err != nil {
			log.Debugf("port forwarding goroutine finished with errors: %s", err)
			p.err = err
			close(ready)
			return
		}
	}(ready)

	log.Debugf("waiting port forwarding to be ready")
	<-ready

	if p.err != nil {
		return p.err
	}

	log.Debugf("port forwarding finished")
	return nil
}

// Stop stops all the port forwarders
func (p *PortForwardManager) Stop() {
	if p.portForwards == nil {
		return
	}

	if p.stopChan == nil {
		log.Debugf("forwarder stop channel was nil")
		return
	}

	log.Debugf("stopping forwarder")
	close(p.stopChan)
	p.portForwards = nil
	log.Debugf("forwarder stopped")
}

func (p *PortForwardManager) forward(namespace, pod string, ready chan struct{}) error {
	url := p.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(p.restConfig)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)
	p.stopChan = make(chan struct{}, 1)
	p.out = new(bytes.Buffer)
	addresses := []string{"localhost"}

	a := os.Getenv("OKTETO_ADDRESS")
	if len(a) > 0 {
		addresses = append(addresses, a)
	}

	ports := []string{}
	for local, remote := range p.portForwards {
		ports = append(ports, fmt.Sprintf("%d:%d", local, remote))
	}

	pf, err := portforward.NewOnAddresses(
		dialer,
		addresses,
		ports,
		p.stopChan,
		ready,
		ioutil.Discard,
		p.out)

	if err != nil {
		return err
	}

	log.Infof("forwarding: %s", ports)

	return pf.ForwardPorts()

}
