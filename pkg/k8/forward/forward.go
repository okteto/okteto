package forward

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/okteto/cnd/pkg/k8/logs"
	log "github.com/sirupsen/logrus"
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
func (p *CNDPortForward) Start(c *kubernetes.Clientset, config *rest.Config, pod *apiv1.Pod, container string) error {
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
			fmt.Printf("Linking '%s' to %s...", p.LocalPath, p.DeploymentName)
			fmt.Println()
			fmt.Printf("Ready! Go to your local IDE and continue coding!")
			fmt.Println()
			p.IsReady = true
			if err := logs.Logs(c, config, pod, container); err != nil {
				log.Errorf("couldn't retrieve logs for %s/%s: %s", pod.Namespace, container, err)
			}
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
