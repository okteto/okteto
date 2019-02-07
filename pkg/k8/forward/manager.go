package forward

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudnativedevelopment/cnd/pkg/log"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type healthcheck func() bool

// CNDPortForwardManager keeps a list of all the active port forwards
type CNDPortForwardManager struct {
	portForwards map[int]*CNDPortForward
	ctx          context.Context
	restConfig   *rest.Config
	client       *kubernetes.Clientset
	ErrChan      chan error
}

type portForwardHealthcheck struct {
	lastConnectionTime time.Time
	isDisconnected     bool
}

// NewCNDPortForwardManager initializes a new instance
func NewCNDPortForwardManager(ctx context.Context, restConfig *rest.Config, c *kubernetes.Clientset) *CNDPortForwardManager {
	return &CNDPortForwardManager{
		ctx:          ctx,
		portForwards: make(map[int]*CNDPortForward),
		restConfig:   restConfig,
		client:       c,
		ErrChan:      make(chan error, 1),
	}
}

// Add initializes a port forward
func (p *CNDPortForwardManager) Add(localPort, remotePort int, h healthcheck) error {
	if _, ok := p.portForwards[localPort]; ok {
		return fmt.Errorf("port %d is already taken, please check your configuration", localPort)
	}

	p.portForwards[localPort] = &CNDPortForward{
		localPort:  localPort,
		remotePort: remotePort,
		out:        new(bytes.Buffer),
		h:          h,
	}

	return nil
}

// Start starts all the port forwarders
func (p *CNDPortForwardManager) Start(pod *apiv1.Pod) {
	req := p.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward")

	var wg sync.WaitGroup

	for _, pf := range p.portForwards {
		wg.Add(1)
		ready := make(chan struct{}, 1)
		go func(f *CNDPortForward, r chan struct{}) {
			defer wg.Done()
			<-r
			log.Debugf("[port-forward-%d:%d] ready", f.localPort, f.remotePort)
			return
		}(pf, ready)

		go func(f *CNDPortForward, r chan struct{}) {
			log.Debugf("[port-forward-%d:%d] connecting to %s/pod/%s", f.localPort, f.remotePort, pod.Namespace, pod.Name)
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
func (p *CNDPortForwardManager) Stop() {
	var wg sync.WaitGroup

	if p.portForwards == nil {
		return
	}

	for _, pf := range p.portForwards {
		wg.Add(1)
		go func(f *CNDPortForward) {
			defer wg.Done()
			f.stop()
		}(pf)

	}

	wg.Wait()
}

// Monitor will send a message to disconnected if healthcheck of a port shows as disconnected for more than 30 seconds.
func (p *CNDPortForwardManager) Monitor(ctx context.Context, disconnect, reconnect chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	maxWait := 30 * time.Second
	healthchecks := make(map[int]*portForwardHealthcheck)

	for k := range p.portForwards {
		healthchecks[k] = &portForwardHealthcheck{
			lastConnectionTime: time.Now(),
			isDisconnected:     false,
		}
	}

	for {
		select {
		case <-ticker.C:
			for k, pf := range p.portForwards {
				if pf.h == nil {
					continue
				}

				if pf.h() {
					if healthchecks[k].isDisconnected {
						healthchecks[k].lastConnectionTime = time.Now()
						healthchecks[k].isDisconnected = false
						reconnect <- struct{}{}
					}

					continue
				}

				currentWait := time.Now().Sub(healthchecks[k].lastConnectionTime)
				if currentWait > maxWait {
					healthchecks[k].isDisconnected = true
					log.Infof("[port-forward-%d:%d] not connected  for %s seconds, sending disconnect notification", pf.localPort, pf.remotePort, currentWait)
					disconnect <- struct{}{}
					healthchecks[k].lastConnectionTime = time.Now()
				}
			}

		case <-ctx.Done():
			return
		}
	}
}
