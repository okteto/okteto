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

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

type reverse struct {
	forward
}

// AddReverse adds a reverse forward
func (fm *ForwardManager) AddReverse(f model.Reverse) error {

	if err := fm.canAdd(f.Local); err != nil {
		return err
	}

	fm.reverses[f.Local] = &reverse{
		forward: forward{
			localAddress:  fmt.Sprintf("%s:%d", fm.localInterface, f.Local),
			remoteAddress: fmt.Sprintf("%s:%d", fm.remoteInterface, f.Remote),
			ctx:           fm.ctx,
		},
	}

	return nil
}

func (r *reverse) start() {
	remoteListener, err := r.pool.getListener(r.remoteAddress)
	if err != nil {
		log.Infof("%s -> failed to listen on remote address: %v", r.String(), err)
		return
	}

	defer remoteListener.Close()

	for {

		r.setConnected()

		log.Infof("%s -> waiting for a connection", r.String())
		remoteConn, err := remoteListener.Accept()
		if err != nil {
			log.Infof("%s -> failed to accept connection: %v", r.String(), err)
			continue
		}

		log.Infof("%s -> accepted connection: %v", r.String(), remoteConn)
		go r.handle(remoteConn)

	}
}

func (r *reverse) handle(remote net.Conn) {
	defer remote.Close()

	quit := make(chan struct{}, 1)
	local, err := net.Dial("tcp", r.localAddress)
	if err != nil {
		log.Infof("%s -> failed to listen on local address: %v", r.String(), err)
		return
	}

	defer local.Close()

	go r.transfer(remote, local, quit)
	go r.transfer(local, remote, quit)

	<-quit
	log.Infof("%s -> stopped", r.String())
}

func (r *reverse) String() string {
	return fmt.Sprintf("ssh reverse forward %s<-%s", r.localAddress, r.remoteAddress)
}

func (r *reverse) transfer(from io.Writer, to io.Reader, quit chan struct{}) {
	_, err := io.Copy(from, to)
	if err != nil {
		log.Infof("%s -> data transfer failed: %v", r.String(), err)
	}

	quit <- struct{}{}
}
