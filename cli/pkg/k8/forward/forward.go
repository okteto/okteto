package forward

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"cli/cnd/pkg/log"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

//CNDPortForward holds the information of the port forward
type CNDPortForward struct {
	stopChan   chan struct{}
	isReady    bool
	localPort  int
	remotePort int
	out        *bytes.Buffer
}

func (p *CNDPortForward) start(config *rest.Config, requestURL *url.URL, pod *apiv1.Pod, ready chan struct{}) error {

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", requestURL)
	p.stopChan = make(chan struct{}, 1)
	p.out = new(bytes.Buffer)
	addresses := []string{"localhost"}

	a := os.Getenv("OKTETO_ADDRESS")
	if len(a) > 0 {
		addresses = append(addresses, a)
	}

	pf, err := portforward.NewOnAddresses(
		dialer,
		addresses,
		[]string{fmt.Sprintf("%d:%d", p.localPort, p.remotePort)},
		p.stopChan,
		ready,
		ioutil.Discard,
		p.out)

	if err != nil {
		return err
	}

	return pf.ForwardPorts()
}

func (p *CNDPortForward) stop() {
	log.Debugf("[port-forward-%d:%d] stopping", p.localPort, p.remotePort)
	log.Debugf("[port-forward-%d:%d] logged errors: %s", p.localPort, p.remotePort, p.out.String())
	close(p.stopChan)
	<-p.stopChan
	log.Debugf("[port-forward-%d:%d] stopped", p.localPort, p.remotePort)
}
