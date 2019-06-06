package forward

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/okteto/okteto/pkg/log"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

//PortForward holds the information of the port forward
type PortForward struct {
	stopChan   chan struct{}
	isReady    bool
	localPort  int
	remotePort int
	out        *bytes.Buffer
}

func (p *PortForward) start(config *rest.Config, requestURL *url.URL, pod string, ready chan struct{}) error {

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

func (p *PortForward) stop() {
	log.Debugf("[port-forward-%d:%d] stopping", p.localPort, p.remotePort)
	log.Debugf("[port-forward-%d:%d] logged errors: %s", p.localPort, p.remotePort, p.out.String())
	if p.stopChan == nil {
		log.Debugf("[port-forward-%d:%d] was nil", p.localPort, p.remotePort)
		return
	}

	close(p.stopChan)
	<-p.stopChan
	log.Debugf("[port-forward-%d:%d] stopped", p.localPort, p.remotePort)
}
