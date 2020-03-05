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

		log.Infof("%s not connected, retrying: %s", f.String(), err)
		t := time.NewTicker(1 * time.Second)
		<-t.C
	}
}

func (f *forward) start(c *ssh.ClientConfig, conn *ssh.Client) error {
	log.Infof("starting %s", f.String())

	// Listen on local port
	listener, err := conn.Listen("tcp", f.localAddress)
	if err != nil {
		return fmt.Errorf("failed open %s: %w", f.localAddress, err)
	}

	defer listener.Close()

	// handle incoming connections on forward tunnel
	for {
		// listen on local port
		var d net.Dialer

		remote, err := d.DialContext(f.ctx, "tcp", f.remoteAddress)
		if err != nil {
			return fmt.Errorf("failed to open remote address %s: %w", f.remoteAddress, err)
		}

		f.ready.Do(func() {
			log.Infof("%s connected and ready", f.String())
			f.connected = true
		})

		client, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept traffic from remote address %s: %w", f.remoteAddress, err)
		}

		f.handleClient(client, remote)
	}
}

func (f *forward) handleClient(client net.Conn, remote net.Conn) {
	defer client.Close()
	chDone := make(chan bool, 1)
	log.Debug("starting forward tunnel transfer ")

	// Start local -> remote data transfer
	go func() {
		_, err := io.Copy(client, remote)
		if err != nil {
			log.Infof("error while copying %s->%s: %s", f.localAddress, f.remoteAddress, err)
		}

		chDone <- true
	}()

	// Start remote -> local data transfer
	go func() {
		_, err := io.Copy(remote, client)
		if err != nil {
			log.Infof("error while copying %s->%s: %s", f.remoteAddress, f.localAddress, err)
		}
		chDone <- true
	}()

	log.Infof("started %s successfully", f.String())
	<-chDone
}

func (f *forward) String() string {
	return fmt.Sprintf("forward %s->%s", f.localAddress, f.remoteAddress)
}
