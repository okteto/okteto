// Copyright 2024 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package remoterun

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/remote"
	"golang.org/x/sync/errgroup"
)

type sshForwarder struct {
	getTLSConfig func() *tls.Config
}

func newSSHForwarder() *sshForwarder {
	return &sshForwarder{
		getTLSConfig: newTLSConfigWithSystemCA,
	}
}

// CA is not specified because it should use System CA root. When remote-run command is executed,
// the CA should be already available as the CLI set it in the Dockerfile used as a base to
// run the command
func newTLSConfigWithSystemCA() *tls.Config {
	return &tls.Config{}
}

// startSshForwarder This functions starts the ssh-forwarded process expected to be run on remote-run commands.
// This process just listens a UNIX socket, and forward the messages to the SSH Agent exposed in the hostname
// and port received as parameters
func (s *sshForwarder) startSshForwarder(sshAgentHostname, sshAgentPort, userToken string) {
	// Remove existing socket if it exists
	os.Remove(remote.SshAgentLocalSocket)

	// Listen on the UNIX domain socket
	localListener, err := net.Listen("unix", remote.SshAgentLocalSocket)
	if err != nil {
		oktetoLog.Fatalf("Failed to listen on UNIX socket %s: %v", remote.SshAgentLocalSocket, err)
	}
	defer localListener.Close()
	oktetoLog.Infof("SSH Agent Forwarder listening on %s\n", remote.SshAgentLocalSocket)

	// Set permissions so only the owner can access the socket
	err = os.Chmod(remote.SshAgentLocalSocket, 0600)
	if err != nil {
		oktetoLog.Fatalf("Failed to set permissions on UNIX socket %s: %v", remote.SshAgentLocalSocket, err)
	}

	for {
		// Accept connections from local clients
		localConn, err := localListener.Accept()
		if err != nil {
			oktetoLog.Errorf("Failed to accept local connection: %v", err)
			continue
		}

		// Handle each connection concurrently
		go s.handleConnection(localConn, sshAgentHostname, sshAgentPort, userToken)
	}
}

func (s *sshForwarder) handleConnection(localConn net.Conn, host, port, userToken string) {
	defer localConn.Close()

	cfg := s.getTLSConfig()

	// Connect to the remote SSH agent over TCP
	remoteConn, err := tls.Dial("tcp", fmt.Sprintf("%s:%s", host, port), cfg)
	if err != nil {
		oktetoLog.Errorf("Failed to connect to remote SSH agent: %v", err)
		return
	}
	defer remoteConn.Close()

	// Send the authentication token
	_, err = remoteConn.Write([]byte(userToken + "\n"))
	if err != nil {
		oktetoLog.Errorf("Failed to write auth token to remote SSH agent: %v", err)
		return
	}

	// Read the acknowledgment
	reader := bufio.NewReader(remoteConn)
	ack, err := reader.ReadString('\n')
	if err != nil {
		oktetoLog.Errorf("Failed to read ACK from remote SSH agent: %v", err)
		return
	}
	ack = strings.TrimSpace(ack)
	if ack != "OK" {
		oktetoLog.Errorf("Invalid ACK message. Expected 'OK' but remote SSH agent returned '%s'", ack)
		return
	}

	var eg errgroup.Group

	// Forward data from local to remote
	eg.Go(func() error {
		_, err := io.Copy(remoteConn, localConn)
		return fmt.Errorf("error while sending data to remote SSH agent: %w", err)
	})

	// Forward data from remote to local
	eg.Go(func() error {
		_, err := io.Copy(localConn, remoteConn)
		return fmt.Errorf("error while receiving data from remote SSH agent: %w", err)
	})

	err = eg.Wait()
	if err != nil && !errors.Is(err, net.ErrClosed) {
		oktetoLog.Errorf("Error sending/receiving data to/from remote SSH agent: %v", err)
	}

	// Connections will be closed by deferred calls
}
