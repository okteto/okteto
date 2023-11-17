// Copyright 2023 The Okteto Authors
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

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type forward struct {
	pool          *pool
	localAddress  string
	remoteAddress string
	lock          sync.Mutex
	c             bool
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

func (f *forward) setDisconnected() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.c = false
}

func (f *forward) start(ctx context.Context) {
	localListener, err := net.Listen("tcp", f.localAddress)
	if err != nil {
		oktetoLog.Infof("%s -> failed to listen: %s", f.String(), err)
		return
	}

	go func() {
		<-ctx.Done()
		f.setDisconnected()
		if err := localListener.Close(); err != nil {
			oktetoLog.Infof("%s -> failed to close: %s", f.String(), err)
		}
		oktetoLog.Infof("%s -> done", f.String())
	}()

	f.setConnected()

	tick := time.NewTicker(100 * time.Millisecond)
	for {
		localConn, err := localListener.Accept()
		if err != nil {
			if !f.connected() {
				return
			}

			oktetoLog.Infof("%s -> failed to accept connection: %v", f.String(), err)
			<-tick.C
			continue
		}
		go f.handle(localConn)
	}

}

func (f *forward) handle(local net.Conn) {
	defer func() {
		if err := local.Close(); err != nil {
			oktetoLog.Debugf("Error closing local connection: %s", err)
		}
	}()

	remote, err := f.pool.get(f.remoteAddress)
	if err != nil {
		oktetoLog.Infof("%s -> failed to dial remote connection: %s", f.String(), err)
		return
	}
	defer func() {
		if err := remote.Close(); err != nil {
			oktetoLog.Debugf("Error closing remote connection: %s", err)
		}
	}()

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
		if !oktetoErrors.IsClosedNetwork(err) {
			oktetoLog.Infof("%s -> data transfer failed: %v", f.String(), err)
		}
	}

	quit <- struct{}{}
}
