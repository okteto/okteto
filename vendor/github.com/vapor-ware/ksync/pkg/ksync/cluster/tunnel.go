package cluster

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/vapor-ware/ksync/pkg/debug"
)

// Tunnel is the connection between the local host and a specific pod in the
// remote cluster.
type Tunnel struct {
	LocalPort  int32
	RemotePort int32
	PodName    string
	Namespace  string
	stopChan   chan struct{}
	readyChan  chan struct{}
	Out        *bytes.Buffer
}

func (t *Tunnel) String() string {
	return debug.YamlString(t)
}

// Fields returns a set of structured fields for logging.
func (t *Tunnel) Fields() log.Fields {
	return debug.StructFields(t)
}

// NewTunnel constructs a new tunnel for the namespace, pod and port.
func NewTunnel(
	namespace string,
	podName string,
	remotePort int32) *Tunnel {

	return &Tunnel{
		RemotePort: remotePort,
		PodName:    podName,
		Namespace:  namespace,
		stopChan:   make(chan struct{}, 1),
		readyChan:  make(chan struct{}, 1),
		Out:        new(bytes.Buffer),
	}
}

// Close closes an existing tunnel
func (t *Tunnel) Close() {
	close(t.stopChan)
	<-t.stopChan
	log.WithFields(t.Fields()).Debug("tunnel closed")
}

// Start starts a given tunnel connection
func (t *Tunnel) Start() error {
	req := Client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(t.Namespace).
		Name(t.PodName).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(kubeCfg)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	if err != nil {
		return err
	}

	local, err := getAvailablePort()
	if err != nil {
		return errors.Wrap(err, "could not find an available port")
	}
	t.LocalPort = local

	log.WithFields(debug.MergeFields(t.Fields(), log.Fields{
		"url": req.URL(),
	})).Debug("starting tunnel")

	pf, err := portforward.New(
		dialer,
		[]string{fmt.Sprintf("%d:%d", t.LocalPort, t.RemotePort)},
		t.stopChan,
		t.readyChan,
		// TODO: there's better places to put this, really anywhere.
		t.Out,
		t.Out)

	if err != nil {
		return errors.Wrap(err, "unable to forward port")
	}

	errChan := make(chan error)
	go func() {
		errChan <- pf.ForwardPorts()
	}()

	select {
	case err = <-errChan:
		return debug.ErrorOut("error forwarding ports", err, t)
	case <-pf.Ready:
		log.WithFields(t.Fields()).Debug("tunnel running")
		return nil
	}
}

// #nosec
func getAvailablePort() (int32, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close() // nolint: errcheck

	_, p, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return 0, err
	}
	return int32(port), err
}
