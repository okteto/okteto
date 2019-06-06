package forward

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// PortForwardManager keeps a list of all the active port forwards
type PortForwardManager struct {
	portForwards map[int]*PortForward
	ctx          context.Context
	restConfig   *rest.Config
	client       *kubernetes.Clientset
	ErrChan      chan error
}

type portForwardHealthcheck struct {
	lastConnectionTime time.Time
	isDisconnected     bool
}

// NewPortForwardManager initializes a new instance
func NewPortForwardManager(ctx context.Context, restConfig *rest.Config, c *kubernetes.Clientset, errchan chan error) *PortForwardManager {
	return &PortForwardManager{
		ctx:          ctx,
		portForwards: make(map[int]*PortForward),
		restConfig:   restConfig,
		client:       c,
		ErrChan:      errchan,
	}
}

// Add initializes a port forward
func (p *PortForwardManager) Add(localPort, remotePort int) error {
	if _, ok := p.portForwards[localPort]; ok {
		return fmt.Errorf("port %d is already taken, please check your configuration", localPort)
	}

	p.portForwards[localPort] = &PortForward{
		localPort:  localPort,
		remotePort: remotePort,
		out:        new(bytes.Buffer),
	}

	return nil
}

// Start starts all the port forwarders
func (p *PortForwardManager) Start(pod, namespace string) {
	req := p.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("portforward")

	var wg sync.WaitGroup

	for _, pf := range p.portForwards {
		wg.Add(1)
		ready := make(chan struct{}, 1)
		go func(f *PortForward, r chan struct{}) {
			defer wg.Done()
			<-r
			log.Debugf("[port-forward-%d:%d] ready", f.localPort, f.remotePort)
			return
		}(pf, ready)

		go func(f *PortForward, r chan struct{}) {
			log.Debugf("[port-forward-%d:%d] connecting to %s/pod/%s", f.localPort, f.remotePort, namespace, pod)
			if err := f.start(p.restConfig, req.URL(), pod, r); err != nil {
				if err == nil {
					log.Debugf("[port-forward-%d:%d] goroutine forwarding finished", f.localPort, f.remotePort)
				} else {
					log.Debugf("[port-forward-%d:%d] goroutine forwarding finished with errors: %s", f.localPort, f.remotePort, err)
				}

				close(ready)
				p.ErrChan <- fmt.Errorf("Unable to listen on %d:%d", f.localPort, f.remotePort)
			}
		}(pf, ready)
	}

	log.Debugf("waiting for all ports to be ready")
	wg.Wait()
	log.Debugf("all ports are ready")
}

// Stop stops all the port forwarders
func (p *PortForwardManager) Stop() {
	var wg sync.WaitGroup

	if p.portForwards == nil {
		return
	}

	for _, pf := range p.portForwards {
		wg.Add(1)
		go func(f *PortForward) {
			defer wg.Done()
			f.stop()
		}(pf)

	}

	wg.Wait()
	p.portForwards = nil
}
