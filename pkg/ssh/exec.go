// Copyright 2021 The Okteto Authors
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

package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/alessio/shellescape"
	dockerterm "github.com/moby/term"
	okErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

// Exec executes the command over SSH
func Exec(ctx context.Context, iface string, remotePort int, tty bool, inR io.Reader, outW, errW io.Writer, command []string) error {
	sshConfig, err := getSSHClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get SSH configuration: %s", err)
	}

	//dockerterm.StdStreams() configures the terminal on windows
	dockerterm.StdStreams()

	var connection *ssh.Client
	t := time.NewTicker(100 * time.Millisecond)
	for i := 0; i < 100; i++ {
		connection, err = dial(ctx, "tcp", fmt.Sprintf("%s:%d", iface, remotePort), sshConfig)
		if err == nil {
			break
		}

		<-t.C
	}

	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %s", err)
	}

	defer connection.Close()
	go func() {
		<-ctx.Done()
		if connection != nil {
			if err := connection.Close(); err != nil {
				if !okErrors.IsClosedNetwork(err) {
					log.Infof("failed to close ssh client for exec: %s", err)
				}
			}
		}
		log.Infof("ssh client for exec closed")
	}()

	session, err := connection.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %s", err)
	}

	defer session.Close()

	if tty {
		modes := ssh.TerminalModes{
			ssh.ECHO:          0,      // Disable echoing
			ssh.ECHOCTL:       0,      // Don't print control chars
			ssh.IGNCR:         1,      // Ignore CR on input
			ssh.TTY_OP_ISPEED: 115200, // baud in
			ssh.TTY_OP_OSPEED: 115200, // baud out
		}

		height, width := 80, 40
		var termFD int
		var ok bool
		if termFD, ok = isTerminal(inR); ok {
			width, height, err = term.GetSize(int(os.Stdout.Fd()))
			log.Infof("terminal width %d height %d", width, height)
			if err != nil {
				log.Infof("request for terminal size failed: %s", err)
			}
		}

		state, err := term.MakeRaw(termFD)
		if err != nil {
			log.Infof("request for raw terminal failed: %s", err)
		}

		defer func() {
			if state == nil {
				return
			}

			if err := term.Restore(termFD, state); err != nil {
				log.Infof("failed to restore terminal: %s", err)
			}

			log.Infof("terminal restored")
		}()

		if err := session.RequestPty("xterm-256color", height, width, modes); err != nil {
			return fmt.Errorf("request for pseudo terminal failed: %s", err)
		}
	}

	sockEnvVar, ok := os.LookupEnv("SSH_AUTH_SOCK")
	if !ok {
		log.Info("SSH_AUTH_SOCK is not set, not forwarding socket")
	} else {
		if err := agent.ForwardToRemote(connection, sockEnvVar); err != nil {
			log.Infof("failed to existing SSH_AUTH_SOCK('%s'): %s", sockEnvVar, err)
		}
		if err := agent.RequestAgentForwarding(session); err != nil {
			log.Infof("failed to forward ssh agent to remote: %s", err)
		}
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stdin for session: %v", err)
	}
	Copy(inR, stdin)

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stdout for session: %v", err)
	}

	go func() {
		if _, err := io.Copy(outW, stdout); err != nil {
			log.Infof("error while writing to stdOut: %s", err)
		}
	}()

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stderr for session: %v", err)
	}

	go func() {
		if _, err := io.Copy(errW, stderr); err != nil {
			log.Infof("error while writing to stdOut: %s", err)
		}
	}()

	cmd := shellescape.QuoteCommand(command)
	log.Infof("executing command over ssh: '%s'", cmd)
	err = session.Run(cmd)
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "status 130") || strings.Contains(err.Error(), "4294967295") {
		return nil
	}
	if strings.Contains(err.Error(), "exit code 137") || strings.Contains(err.Error(), "exit status 137") {
		log.Yellow(`Insufficient memory. Please update your resources on your okteto manifest.
More information is available here: https://okteto.com/docs/reference/manifest/#resources-object-optional`)
	}

	log.Infof("command failed: %s", err)

	return err
}

func isTerminal(r io.Reader) (int, bool) {
	switch v := r.(type) {
	case *os.File:
		return int(v.Fd()), term.IsTerminal(int(v.Fd()))
	default:
		return 0, false
	}
}

func dial(ctx context.Context, network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	d := net.Dialer{Timeout: config.Timeout}
	conn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}
