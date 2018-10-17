package forward

import (
	"bytes"
	"fmt"
	"log"
	"net/http"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

//CNDPortForward holds the information of the port forward
type CNDPortForward struct {
	StopChan   chan struct{}
	ReadyChan  chan struct{}
	LocalPort  int
	RemotePort int
	Out        *bytes.Buffer
}

//NewCNDPortForward initializes and returns a new port forward structure
func NewCNDPortForward(remotePort int) *CNDPortForward {
	return &CNDPortForward{
		LocalPort:  22100,
		RemotePort: remotePort,
		StopChan:   make(chan struct{}, 1),
		ReadyChan:  make(chan struct{}, 1),
		Out:        new(bytes.Buffer),
	}
}

// Start starts a port foward for the specified port. The function will block until
// p.Stop is called
func (p *CNDPortForward) Start(c *kubernetes.Clientset, config *rest.Config, pod *apiv1.Pod) error {
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

	log.Printf("tunneling from %s to %s", p.LocalPort, p.RemotePort)
	return pf.ForwardPorts()
}

// Stop stops the port forwarding
func (p *CNDPortForward) Stop() {
	close(p.StopChan)
	<-p.StopChan
}
