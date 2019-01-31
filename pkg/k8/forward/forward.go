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

	p.wg = wg
	p.wg.Add(1)

	go func() {
		<-ctx.Done()
		log.Debugf("initiating portforward cancellation sequence")
		p.Stop()
		return
	}()

	p.IsReady = false
	go func(f *portforward.PortForwarder) {
		f.ForwardPorts()
		log.Debugf("forwardPorts goroutine finished")
		return
	}(pf)

	<-pf.Ready
	p.IsReady = true
	return nil
}

// Stop cleanly shutdowns the port forwarder
func (p *CNDPortForward) Stop() {
	p.mux.Lock()
	defer p.mux.Unlock()

	defer p.wg.Done()
	log.Debugf("[port-forward-%d:%d]: %s", p.LocalPort, p.RemotePort, p.Out.String())
	if p.StopChan != nil {
		close(p.StopChan)
		<-p.StopChan
	}
	log.Debug("port forward clean shutdown")
}
