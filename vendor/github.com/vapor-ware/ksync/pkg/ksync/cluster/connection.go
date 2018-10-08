package cluster

import (
	"fmt"
	"time"

	"github.com/cenkalti/backoff"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/vapor-ware/ksync/pkg/debug"
)

var (
	maxReadyRetries = uint64(10)
)

// Connection creates and manages the tunnels and gRPC connection between the
// local host a ksync pod running on the remote cluster
type Connection struct {
	NodeName string

	service *Service
	tunnels []*Tunnel
}

// NewConnection is the constructor for Connection. You specify the node you'd
// like to establish a connection to here.
func NewConnection(nodeName string) *Connection {
	return &Connection{
		NodeName: nodeName,
		service:  NewService(),
		tunnels:  []*Tunnel{},
	}
}

func (c *Connection) String() string {
	return debug.YamlString(c)
}

// Fields returns a set of structured fields for logging.
func (c *Connection) Fields() log.Fields {
	return debug.StructFields(c)
}

func (c *Connection) opts() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTimeout(5 * time.Second), // nolint: megacheck
		grpc.WithBlock(),
		grpc.WithInsecure(),
	}
}

func (c *Connection) waitForHealthy() error {
	test := func() error {
		ready, err := c.service.IsHealthy(c.NodeName)
		if err != nil {
			return backoff.Permanent(err)
		}

		if !ready {
			return fmt.Errorf("ksync pod on %s not ready", c.NodeName)
		}

		return nil
	}

	return backoff.Retry(
		test,
		backoff.WithMaxRetries(backoff.NewExponentialBackOff(), maxReadyRetries))
}

func (c *Connection) connection(port int32) (int32, error) {
	if err := c.waitForHealthy(); err != nil {
		return 0, err
	}

	podName, err := c.service.PodName(c.NodeName)
	if err != nil {
		return 0, debug.ErrorOut("cannot get pod name", err, c)
	}

	tun := NewTunnel(c.service.Namespace, podName, port)

	if err := tun.Start(); err != nil {
		return 0, debug.ErrorOut("unable to start tunnel", err, c)
	}

	c.tunnels = append(c.tunnels, tun)

	return tun.LocalPort, nil
}

// Radar creates a new tunnel and gRPC connection to the radar container
// running in the ksync pod specified by Container.NodeName
func (c *Connection) Radar() (*grpc.ClientConn, error) {
	localPort, err := c.connection(c.service.RadarPort)
	if err != nil {
		return nil, debug.ErrorLocation(err)
	}

	return grpc.Dial(fmt.Sprintf("127.0.0.1:%d", localPort), c.opts()...)
}

// Syncthing creates a tunnel for both the API and sync ports to the
// syncthing container running in the ksync pod specified by Container.NodeName
func (c *Connection) Syncthing() (int32, int32, error) {
	apiPort, err := c.connection(c.service.SyncthingAPI)
	if err != nil {
		return 0, 0, err
	}

	listenerPort, err := c.connection(c.service.SyncthingListener)
	if err != nil {
		return 0, 0, err
	}

	return apiPort, listenerPort, nil
}

// Stop cleans all the established tunnels up. It should be called when this
// connection is no longer needed.
func (c *Connection) Stop() error {
	for _, tun := range c.tunnels {
		tun.Close()
	}
	log.WithFields(c.Fields()).Debug("stopped connection")
	return nil
}
