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
	"sync"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"golang.org/x/crypto/ssh"
)

// ForwardManager handles the lifecycle of all the forwards
type ForwardManager struct {
	forwards map[int]*forward
	reverses map[int]*reverse
	ctx      context.Context
	sshAddr  string
}

// NewForwardManager returns a newly initialized instance of ForwardManager
func NewForwardManager(ctx context.Context, sshAddr string) *ForwardManager {
	return &ForwardManager{
		ctx:      ctx,
		forwards: make(map[int]*forward),
		reverses: make(map[int]*reverse),
		sshAddr:  sshAddr,
	}
}

func (fm *ForwardManager) canAdd(localPort int) error {
	if _, ok := fm.reverses[localPort]; ok {
		return fmt.Errorf("port %d is already taken, please check your reverse forwards configuration", localPort)
	}

	if _, ok := fm.forwards[localPort]; ok {
		return fmt.Errorf("port %d is already taken, please check your forwards configuration", localPort)
	}

	return nil
}

// Add initializes a remote forward
func (fm *ForwardManager) Add(f *model.Forward) error {

	if err := fm.canAdd(f.Local); err != nil {
		return err
	}

	fm.forwards[f.Local] = &forward{
		localAddress:  fmt.Sprintf("localhost:%d", f.Local),
		remoteAddress: fmt.Sprintf("0.0.0.0:%d", f.Remote),
		ready:         sync.Once{},
		ctx:           fm.ctx,
	}
	return nil
}

// Start starts all the remote forwards and reverse forwards as goroutines
func (fm *ForwardManager) Start() error {
	log.Info("starting forward manager")

	// Connect to SSH remote server using serverEndpoint
	c := getSSHClientConfig()
	conn, err := ssh.Dial("tcp", fm.sshAddr, c)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH host: %s", err)
	}

	for _, ff := range fm.forwards {
		go ff.startWithRetry(c, conn)
	}

	for _, rt := range fm.reverses {
		go rt.startWithRetry(c, conn)
		if err != nil {
			return fmt.Errorf("failed to connect to SSH host: %s", err)
		}

	}

	return nil
}
