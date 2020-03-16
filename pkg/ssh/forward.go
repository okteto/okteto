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

func (f *forward) start(config *ssh.ClientConfig, serverAddr string) {
	localListener, err := net.Listen("tcp", f.localAddress)
	if err != nil {
		log.Infof("%s -> failed to listen on local address: %v", f.String(), err)
		return
	}

	defer localListener.Close()

	for {
		log.Infof("%s -> waiting for a connection", f.String())
		localConn, err := localListener.Accept()
		if err != nil {
			log.Infof("%s -> failed to accept connection: %v", f.String(), err)
			continue
		}

		log.Infof("%s -> accepted connection: %v", f.String(), localConn)
		go f.handle(localConn, config, serverAddr)
	}
}

func (f *forward) handle(local net.Conn, config *ssh.ClientConfig, serverAddr string) {
	defer local.Close()

	sshConn, err := ssh.Dial("tcp", serverAddr, config)
	if err != nil {
		log.Infof("%s -> ssh connection failed: %s", f.String(), err)
		return
	}

	defer sshConn.Close()

	log.Infof("%s -> started SSH connection", f.String())

	remote, err := sshConn.Dial("tcp", f.remoteAddress)
	if err != nil {
		log.Infof("%s -> forwarding failed: %s", f.String(), err)
		return
	}

	defer remote.Close()

	quit := make(chan struct{}, 1)

	go f.transfer(remote, local, quit)
	go f.transfer(local, remote, quit)

	<-quit
	log.Infof("%s -> stopped", f.String())
}

func (f *forward) String() string {
	return fmt.Sprintf("ssh forward %s->%s", f.localAddress, f.remoteAddress)
}

func (f *forward) transfer(from io.Writer, to io.Reader, quit chan struct{}) {
	_, err := io.Copy(from, to)
	if err != nil {
		log.Infof("%s -> data transfer failed: %v", f.String(), err)
	}

	quit <- struct{}{}
}
