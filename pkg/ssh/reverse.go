package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"golang.org/x/crypto/ssh"
)

type remoteForward struct {
	localPort  int
	remotePort int
}

// RemoteForwardManager handles the lifecycle of all the remote forwards
type RemoteForwardManager struct {
	remoteForwards map[int]*remoteForward
	ctx            context.Context
	sshUser        string
	sshHost        string
	sshPort        int
}

// NewRemoteForwardManager returns a newly initialized instance of RemoteRemoteForwardManager
func NewRemoteForwardManager(ctx context.Context, sshPort int) *RemoteForwardManager {
	return &RemoteForwardManager{
		ctx:            ctx,
		remoteForwards: make(map[int]*remoteForward),
		sshUser:        "root",
		sshHost:        "localhost",
		sshPort:        sshPort,
	}
}

// Add initializes a remote forward
func (r *RemoteForwardManager) Add(f *model.RemoteForward) error {

	localPort := f.Local
	remotePort := f.Remote

	if _, ok := r.remoteForwards[localPort]; ok {
		return fmt.Errorf("port %d is already taken, please check your remote forward configuration", localPort)
	}

	r.remoteForwards[localPort] = &remoteForward{localPort: localPort, remotePort: remotePort}
	return nil
}

// Start starts all the remote forwards as goroutines
func (r *RemoteForwardManager) Start() error {
	log.Info("starting remote forward manager")

	// Connect to SSH remote server using serverEndpoint
	c := &ssh.ClientConfig{
		User:            r.sshUser,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshAddr := fmt.Sprintf("%s:%d", r.sshHost, r.sshPort)

	for _, rt := range r.remoteForwards {
		go rt.startWithRetry(r.ctx, c, sshAddr)
	}

	return nil
}

func (r *remoteForward) startWithRetry(ctx context.Context, c *ssh.ClientConfig, sshAddr string) error {
	log.Infof("starting remote forward tunnel %d->%d", r.remotePort, r.localPort)

	for {
		err := r.start(ctx, c, sshAddr)
		if err == nil {
			log.Infof("remote forward tunnel %d->%d exited", r.remotePort, r.localPort)
			return nil
		}

		log.Infof("remote forward tunnel %d->%d not connected, retrying: %s", r.remotePort, r.localPort, err)
		t := time.NewTicker(3 * time.Second)
		<-t.C
	}
}

func (r *remoteForward) start(ctx context.Context, c *ssh.ClientConfig, sshAddr string) error {
	log.Infof("starting remote forward tunnel %d->%d", r.remotePort, r.localPort)

	serverConn, err := ssh.Dial("tcp", sshAddr, c)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH host: %s", err)
	}

	// Listen on remote server port
	listener, err := serverConn.Listen("tcp", fmt.Sprintf("localhost:%d", r.remotePort))
	if err != nil {
		return fmt.Errorf("failed open remote port %d: %s", r.remotePort, err)
	}

	defer listener.Close()

	// handle incoming connections on remote forward tunnel
	for {
		// listen on local port
		var d net.Dialer

		local, err := d.DialContext(ctx, "tcp", fmt.Sprintf("localhost:%d", r.localPort))
		if err != nil {
			return fmt.Errorf("failed to open local port %d: %s", r.remotePort, err)
		}

		client, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept traffic from remote port %d: %s", r.remotePort, err)
		}

		r.handleClient(client, local)
	}
}

func (r *remoteForward) handleClient(client net.Conn, local net.Conn) {
	defer client.Close()
	chDone := make(chan error, 1)
	log.Debug("starting remote forward tunnel transfer ")

	// Start remote -> local data transfer
	go func() {
		_, err := io.Copy(client, local)
		if err != nil {
			log.Infof("error while copying %d->%d: %s", r.remotePort, r.localPort, err)
		}

		chDone <- nil
	}()

	// Start local -> remote data transfer
	go func() {
		_, err := io.Copy(local, client)
		if err != nil {
			log.Infof("error while copying %d->%d: %s", r.localPort, r.remotePort, err)
		}
		chDone <- nil
	}()

	log.Infof("started remote forward tunnel %d->%d successfully", r.remotePort, r.localPort)
	<-chDone
}
