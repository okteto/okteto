package forward

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/cloudnativedevelopment/cnd/pkg/log"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

//CNDPortForward holds the information of the port forward
type CNDPortForward struct {
	StopChan       chan struct{}
	IsReady        bool
	LocalPort      int
	RemotePort     int
	LocalPath      string
	DeploymentName string
	Out            *bytes.Buffer
	wg             *sync.WaitGroup
	mux            sync.Mutex
}

//NewCNDPortForward initializes and returns a new port forward structure
func NewCNDPortForward(remoteAddress string) (*CNDPortForward, error) {
	parsed, err := url.Parse(remoteAddress)
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(parsed.Port())

	return &CNDPortForward{
		LocalPort:  port,
		RemotePort: 22000,
		IsReady:    false,
		Out:        new(bytes.Buffer),
	}, nil
}

// Start starts a port foward for the specified port.
func (p *CNDPortForward) Start(
	ctx context.Context, wg *sync.WaitGroup,
	c *kubernetes.Clientset, config *rest.Config, pod *apiv1.Pod) error {

	p.mux.Lock()
	defer p.mux.Unlock()

	req := c.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	p.StopChan = make(chan struct{}, 1)
	p.Out = new(bytes.Buffer)
	ready := make(chan struct{}, 1)
	pf, err := portforward.New(
		dialer,
		[]string{fmt.Sprintf("%d:%d", p.LocalPort, p.RemotePort)},
		p.StopChan,
		ready,
		p.Out,
		p.Out)

	if err != nil {
		return err
	}

	p.IsReady = false
	go func(f *portforward.PortForwarder, local, remote int) {
		err := f.ForwardPorts()
		if err == nil {
			log.Debugf("[port-forward-%d:%d] goroutine forwarding finished", local, remote)
		} else {
			log.Debugf("[port-forward-%d:%d] goroutine forwarding finished with errors: %s", local, remote, err)
		}

		return
	}(pf, p.LocalPort, p.RemotePort)

	<-pf.Ready
	p.IsReady = true
	log.Debugf("[port-forward-%d:%d] connection ready", p.LocalPort, p.RemotePort)

	p.wg.Add(1)
	go func(t context.Context, c *CNDPortForward) {
		for {
			select {
			case <-t.Done():
				log.Debugf("[port-forward-%d:%d] starting cancellation sequence", c.LocalPort, c.RemotePort)
				p.Stop()
				return
			case <-c.StopChan:
				return
			}
		}
	}(ctx, p)

	return nil
}

// Stop cleanly shutdowns the port forwarder
func (p *CNDPortForward) Stop() {
	log.Debugf("[port-forward-%d:%d] stopping", p.LocalPort, p.RemotePort)
	p.mux.Lock()
	defer p.mux.Unlock()

	log.Debugf("[port-forward-%d:%d] logged:\n%s", p.LocalPort, p.RemotePort, p.Out.String())
	if p.StopChan != nil && p.IsReady {
		p.IsReady = false
		close(p.StopChan)
		<-p.StopChan
	}

	log.Debugf("[port-forward-%d:%d] stopped", p.LocalPort, p.RemotePort)
}
