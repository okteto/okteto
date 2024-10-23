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
)

// startSshForwarder This functions starts the ssh-forwarded process expected to be run on remote-run commands.
// This process just listens a UNIX socket, and forward the messages to the SSH Agent exposed in the hostname
// and port received as parameters
func startSshForwarder(sshAgentHostname, sshAgentPort, userToken string) {
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
	os.Chmod(remote.SshAgentLocalSocket, 0600)

	for {
		// Accept connections from local clients
		localConn, err := localListener.Accept()
		if err != nil {
			oktetoLog.Errorf("Failed to accept local connection: %v", err)
			continue
		}

		// Handle each connection concurrently
		go handleConnection(localConn, sshAgentHostname, sshAgentPort, userToken)
	}
}

func handleConnection(localConn net.Conn, host, port, userToken string) {
	defer localConn.Close()

	// CA is not specified because it should use System CA root. When remote-run command is executed,
	// the CA should be already available as the CLI set it in the Dockerfile used as a base to
	// run the command
	cfg := &tls.Config{}

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
		oktetoLog.Errorf("Remote SSH agent returned '%s'", ack)
		return
	}

	// Channel to receive errors from goroutines
	errChan := make(chan error, 2)

	// Forward data from local to remote
	go func() {
		_, err := io.Copy(remoteConn, localConn)
		if err != nil && !isClosedNetworkError(err) {
			oktetoLog.Errorf("Error copying from local to remote: %v", err)
		}
		errChan <- err
	}()

	// Forward data from remote to local
	go func() {
		_, err := io.Copy(localConn, remoteConn)
		if err != nil && !isClosedNetworkError(err) {
			oktetoLog.Errorf("Error copying from remote to local: %v", err)
		}
		errChan <- err
	}()

	// Wait for both directions to finish
	<-errChan
	<-errChan

	// Connections will be closed by deferred calls
}

func isClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, net.ErrClosed)
}
