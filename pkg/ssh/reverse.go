package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"golang.org/x/crypto/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// ReverseTunnel contains the information to connect and setup a reverse tunnel
type ReverseTunnel struct {
	SSHHost    string
	SSHPort    int
	LocalPort  int
	RemotePort int
}

// NewReverseTunnel returns a configured reverse tunnel endpoint
func NewReverseTunnel(sshPort, localPort, remotePort int) *ReverseTunnel {
	return &ReverseTunnel{
		SSHHost:    "localhost",
		SSHPort:    sshPort,
		LocalPort:  localPort,
		RemotePort: remotePort,
	}
}

// Start starts the reverse tunnel to the remote server. It will block until the connection sends an EOF
func (r *ReverseTunnel) Start(ctx context.Context) error {
	for {
		err := r.start(ctx)
		if err == nil {
			log.Infof("connection closed cleanly")
			return nil
		}

		log.Infof("reverse tunnel disconnected %d->%d, retrying: %s", r.RemotePort, r.LocalPort, err)
		t := time.NewTicker(3 * time.Second)
		<-t.C
	}
}

func (r *ReverseTunnel) start(ctx context.Context) error {
	log.Infof("starting reverse tunnel %d->%d", r.RemotePort, r.LocalPort)

	// Connect to SSH remote server using serverEndpoint
	config := &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
	}

	serverConn, err := gossh.Dial("tcp", fmt.Sprintf("%s:%d", r.SSHHost, r.SSHPort), config)
	if err != nil {
		return err
	}

	// Listen on remote server port
	listener, err := serverConn.Listen("tcp", fmt.Sprintf("localhost:%d", r.RemotePort))
	if err != nil {
		return fmt.Errorf("failed open remote port %d: %s", r.RemotePort, err)
	}

	defer listener.Close()

	// handle incoming connections on reverse forwarded tunnel
	for {
		// listen on local port
		local, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", r.LocalPort))
		if err != nil {
			return fmt.Errorf("failed to open local port %d: %s", r.RemotePort, err)
		}

		client, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept traffic from remote port %d: %s", r.RemotePort, err)
		}

		r.handleClient(client, local)
	}
}

func (r *ReverseTunnel) handleClient(client net.Conn, local net.Conn) {
	defer client.Close()
	chDone := make(chan error, 1)
	log.Debug("starting reverse tunnel transfer ")

	// Start remote -> local data transfer
	go func() {
		_, err := io.Copy(client, local)
		if err != nil {
			log.Infof("error while copying %s:%d->localhost:%d: %s", r.SSHHost, r.RemotePort, r.LocalPort, err)
		}

		chDone <- nil
	}()

	// Start local -> remote data transfer
	go func() {
		_, err := io.Copy(local, client)
		if err != nil {
			log.Infof("error while copying localhost:%d->%s:%d: %s", r.LocalPort, r.SSHHost, r.RemotePort, err)
		}
		chDone <- nil
	}()

	log.Infof("started reverse tunnel %d->%d successfully", r.RemotePort, r.LocalPort)
	<-chDone
}
