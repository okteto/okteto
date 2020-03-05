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
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"golang.org/x/crypto/ssh"
)

type reverse struct {
	forward
}

// AddReverse adds a reverse forward
func (fm *ForwardManager) AddReverse(f *model.Reverse) error {

	if err := fm.canAdd(f.Local); err != nil {
		return err
	}

	fm.reverses[f.Local] = &reverse{
		forward: forward{
			localAddress:  fmt.Sprintf("%s:%d", fm.localInterface, f.Local),
			remoteAddress: fmt.Sprintf("%s:%d", fm.remoteInterface, f.Remote),
			ready:         sync.Once{},
			ctx:           fm.ctx,
		},
	}

	return nil
}

func (r *reverse) startWithRetry(c *ssh.ClientConfig, conn *ssh.Client) {
	for {
		err := r.start(c, conn)
		if err == nil {
			log.Infof("%s exited", r.String())
			return
		}

		log.Infof("%s not connected, retrying: %s", r.String(), err)
		t := time.NewTicker(1 * time.Second)
		<-t.C
	}
}

func (r *reverse) start(c *ssh.ClientConfig, conn *ssh.Client) error {
	log.Infof("starting %s", r.String())

	// Listen on remote server port
	listener, err := conn.Listen("tcp", r.remoteAddress)
	if err != nil {
		return fmt.Errorf("failed open remote address %s: %w", r.remoteAddress, err)
	}

	defer listener.Close()

	// handle incoming connections on remote forward tunnel
	for {
		// listen on local port
		var d net.Dialer

		local, err := d.DialContext(r.ctx, "tcp", r.localAddress)
		if err != nil {
			return fmt.Errorf("failed to open local address %s: %w", r.localAddress, err)
		}

		r.ready.Do(func() {
			log.Infof("%s connected and ready", r.String())
			r.connected = true
		})

		client, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept traffic from remote address %s: %w", r.remoteAddress, err)
		}

		r.handleClient(client, local)
	}
}

func (r *reverse) handleClient(client net.Conn, local net.Conn) {
	defer client.Close()
	chDone := make(chan bool, 1)
	log.Debugf("starting %s tunnel transfer ", r.String())

	// Start remote -> local data transfer
	go func() {
		_, err := io.Copy(client, local)
		if err != nil {
			log.Infof("error while copying %s->%s: %s", r.remoteAddress, r.localAddress, err)
		}

		chDone <- true
	}()

	// Start local -> remote data transfer
	go func() {
		_, err := io.Copy(local, client)
		if err != nil {
			log.Infof("error while copying %s->%s: %s", r.remoteAddress, r.localAddress, err)
		}
		chDone <- true
	}()

	log.Infof("started %s successfully", r.String())
	<-chDone
}

func (r *reverse) String() string {
	return fmt.Sprintf("reverse forward %s<-%s", r.localAddress, r.remoteAddress)
}
