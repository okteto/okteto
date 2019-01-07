package forward

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

//CNDPortForward holds the information of the port forward
type CNDPortForward struct {
	StopChan       chan struct{}
	ReadyChan      chan struct{}
	IsReady        bool
	LocalPort      int
	RemotePort     int
	LocalPath      string
	DeploymentName string
	Out            *bytes.Buffer
}

//NewCNDPortForward initializes and returns a new port forward structure
func NewCNDPortForward(localPath, remoteAddress, deploymentName string) (*CNDPortForward, error) {
	parsed, err := url.Parse(remoteAddress)
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(parsed.Port())

	return &CNDPortForward{
		LocalPort:      port,
		RemotePort:     22000,
		StopChan:       make(chan struct{}, 1),
		ReadyChan:      make(chan struct{}, 1),
		Out:            new(bytes.Buffer),
		LocalPath:      localPath,
		DeploymentName: deploymentName,
		IsReady:        false,
	}, nil
}

// Start starts a port foward for the specified port. The function will block until
// p.Stop is called
func (p *CNDPortForward) Start(c *kubernetes.Clientset, config *rest.Config, pod *apiv1.Pod, ready chan<- bool, wg *sync.WaitGroup) error {
	defer wg.Done()
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

	pf, err := portforward.New(
		dialer,
		[]string{fmt.Sprintf("%d:%d", p.LocalPort, p.RemotePort)},
		p.StopChan,
		p.ReadyChan,
		p.Out,
		p.Out)

	if err != nil {
		return err
	}

	go func() {
		select {
		case <-pf.Ready:
			p.IsReady = true
			ready <- p.IsReady
		}
	}()

	return pf.ForwardPorts()
}

// Stop stops the port forwarding
func (p *CNDPortForward) Stop() {
	if p.StopChan != nil {
		close(p.StopChan)
		<-p.StopChan
	}
}
