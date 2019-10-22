package forward

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"

	"github.com/okteto/okteto/pkg/log"

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
func NewPortForwardManager(ctx context.Context, restConfig *rest.Config, c *kubernetes.Clientset, errchan chan error) *PortForwardManager {
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
func (p *PortForwardManager) Add(localPort, remotePort int) error {
	if _, ok := p.portForwards[localPort]; ok {
		return fmt.Errorf("port %d is already taken, please check your configuration", localPort)
	}

	p.portForwards[localPort] = remotePort
	return nil
}

// Start starts all the port forwarders
func (p *PortForwardManager) Start(pod, namespace string) error {
	var wg sync.WaitGroup

	ready := make(chan struct{}, 1)
	wg.Add(1)
	go func(r chan struct{}) {
		defer wg.Done()
		<-r
		log.Debugf("port forwarding finished")
	}(ready)

	go func(r chan struct{}) {
		if err := p.forward(namespace, pod, ready); err != nil {
			log.Debugf("port forwarding goroutine finished with errors: %s", err)
			p.err = err
			close(ready)
			return
		}
	}(ready)

	log.Debugf("waiting port forwarding to be ready")
	wg.Wait()
	if p.err != nil {
		return p.err
	}

	log.Debugf("port forwarding set up")
	return nil
}

// Stop stops all the port forwarders
func (p *PortForwardManager) Stop() {
	if p.portForwards == nil {
		return
	}

	if p.stopChan == nil {
		log.Debugf("stop channel was nil")
		return
	}

	close(p.stopChan)
	<-p.stopChan
	p.portForwards = nil
	log.Debugf("forwarder stopped")
}

func (p *PortForwardManager) forward(namespace, pod string, ready chan struct{}) error {
	req := p.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(p.restConfig)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	p.stopChan = make(chan struct{}, 1)
	p.out = new(bytes.Buffer)
	addresses := []string{"localhost"}

	a := os.Getenv("OKTETO_ADDRESS")
	if len(a) > 0 {
		addresses = append(addresses, a)
	}

	ports := []string{}
	for local,remote := range p.portForwards {
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
