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
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/okteto/okteto/pkg/log"
)

type forward struct {
	ctx           context.Context
	localAddress  string
	remoteAddress string
	c             bool
	lock          sync.Mutex
	pool          *pool
}

func (f *forward) connected() bool {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.c
}

func (f *forward) setConnected() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.c = true

}

func (f *forward) start() {
	localListener, err := net.Listen("tcp", f.localAddress)
	if err != nil {
		log.Infof("%s -> failed to listen on local address: %v", f.String(), err)
		return
	}

	defer localListener.Close()

	f.setConnected()

	for {
		localConn, err := localListener.Accept()
		if err != nil {
			log.Infof("%s -> failed to accept connection: %v", f.String(), err)
			continue
		}

		go f.handle(localConn)

	}
}

func (f *forward) handle(local net.Conn) {
	defer local.Close()

	remote, err := f.pool.get(f.remoteAddress)
	if err != nil {
		log.Infof("%s -> %s", f.String(), err)
		return
	}

	defer remote.Close()

	quit := make(chan struct{}, 1)

	go f.transfer(remote, local, quit)
	go f.transfer(local, remote, quit)

	<-quit
}

func (f *forward) String() string {
	return fmt.Sprintf("ssh forward %s->%s", f.localAddress, f.remoteAddress)
}

func (f *forward) transfer(from io.Writer, to io.Reader, quit chan struct{}) {
	_, err := io.Copy(from, to)
	if err != nil {
		if unwrapError := errors.Unwrap(err); unwrapError != nil {
			err = unwrapError
		}
		m := err.Error()
		if !strings.Contains(m, "use of closed network connection") {
			log.Infof("%s -> data transfer failed: %v", f.String(), err)
		}
	}

	quit <- struct{}{}
}
