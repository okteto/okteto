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
	"net"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"golang.org/x/crypto/ssh"
)

type forward struct {
	ctx           context.Context
	localAddress  string
	remoteAddress string
	ready         sync.Once
	connected     bool
}

func (f *forward) startWithRetry(c *ssh.ClientConfig, conn *ssh.Client) {
	for {
		err := f.start(c, conn)
		if err == nil {
			log.Infof("%s exited", f.String())
			return
		}

		log.Debugf("%s exited with error: %s", f.String(), err)
		t := time.NewTicker(200 * time.Millisecond)
		<-t.C
	}
}

func (f *forward) start(c *ssh.ClientConfig, conn *ssh.Client) error {
	// Establish connection with remote server
	remote, err := conn.Dial("tcp", f.remoteAddress)
	if err != nil {
		return fmt.Errorf("failed to connect %s: %w", f.remoteAddress, err)
	}
	defer remote.Close()

	// Start local server to forward traffic to remote connection
	local, err := net.Listen("tcp", f.localAddress)
	if err != nil {
		return fmt.Errorf("failed to listen to local traffic %s: %w", f.localAddress, err)
	}
	defer local.Close()

	// handle incoming connections on forward tunnel
	for {

		client, err := local.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept traffic from remote address %s: %w", f.remoteAddress, err)
		}

		f.handleClient(client, remote)
	}
}

func (f *forward) handleClient(client net.Conn, remote net.Conn) {
	defer client.Close()
	chDone := make(chan bool, 1)
	log.Debugf("starting %s transfer ", f.String())

	// Start local -> remote data transfer
	go func() {
		_, err := io.Copy(client, remote)
		if err != nil {
			if err != io.EOF {
				log.Infof("error while copying %s->%s: %s", f.localAddress, f.remoteAddress, err)
			}
		}

		chDone <- true
	}()

	// Start remote -> local data transfer
	go func() {
		_, err := io.Copy(remote, client)
		if err != nil {
			if err != io.EOF {
				log.Infof("error while copying %s->%s: %s", f.remoteAddress, f.localAddress, err)
			}
		}
		chDone <- true
	}()

	log.Infof("started %s successfully", f.String())
	<-chDone
	log.Infof("%s finished", f.String())
}

func (f *forward) String() string {
	return fmt.Sprintf("ssh forward %s->%s", f.localAddress, f.remoteAddress)
}
