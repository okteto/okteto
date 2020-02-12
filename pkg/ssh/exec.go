// Copyright 2020 The Okteto Authors
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
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

// Exec executes the command over SSH
func Exec(ctx context.Context, remotePort int, tty bool, inR io.Reader, outW, errW io.Writer, command []string) error {
	sshConfig := &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	log.Info("starting SSH connection")
	var connection *ssh.Client
	var err error
	t := time.NewTicker(100 * time.Millisecond)
	for i := 0; i < 100; i++ {
		err = nil
		connection, err = ssh.Dial("tcp", fmt.Sprintf("localhost:%d", remotePort), sshConfig)
		if err == nil {
			break
		}

		log.Debugf("failed to connect to SSH server, will retry: %s", err)
		<-t.C
	}

	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %s", err)
	}

	session, err := connection.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %s", err)
	}

	defer session.Close()

	if tty {
		modes := ssh.TerminalModes{
			ssh.ECHO:  0, // Disable echoing
			ssh.IGNCR: 1, // Ignore CR on input
		}

		width, height, err := terminal.GetSize(0)
		if err != nil {
			return fmt.Errorf("request for terminal size failed: %s", err)
		}

		if err := session.RequestPty("xterm", height, width, modes); err != nil {
			return fmt.Errorf("request for pseudo terminal failed: %s", err)
		}
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("Unable to setup stdin for session: %v", err)
	}
	go io.Copy(stdin, inR)

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("Unable to setup stdout for session: %v", err)
	}
	go io.Copy(outW, stdout)

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("Unable to setup stderr for session: %v", err)
	}
	go io.Copy(errW, stderr)

	cmd := strings.Join(command, " ")
	log.Infof("executing command over SSH: '%s'", cmd)
	return session.Run(cmd)
}
