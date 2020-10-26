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

	k8sforward "github.com/okteto/okteto/pkg/k8s/forward"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

// ForwardManager handles the lifecycle of all the forwards
type ForwardManager struct {
	localInterface  string
	remoteInterface string
	forwards        map[int]*forward
	reverses        map[int]*reverse
	ctx             context.Context
	sshAddr         string
	pf              *k8sforward.PortForwardManager
}

// NewForwardManager returns a newly initialized instance of ForwardManager
func NewForwardManager(ctx context.Context, sshAddr, localInterface, remoteInterface string, pf *k8sforward.PortForwardManager) *ForwardManager {
	return &ForwardManager{
		ctx:             ctx,
		localInterface:  localInterface,
		remoteInterface: remoteInterface,
		forwards:        make(map[int]*forward),
		reverses:        make(map[int]*reverse),
		sshAddr:         sshAddr,
		pf:              pf,
	}
}

func (fm *ForwardManager) canAdd(localPort int) error {
	if _, ok := fm.reverses[localPort]; ok {
		return fmt.Errorf("port %d is listed multiple times, please check your reverse forwards configuration", localPort)
	}

	if _, ok := fm.forwards[localPort]; ok {
		return fmt.Errorf("port %d is listed multiple times, please check your forwards configuration", localPort)
	}

	if !model.IsPortAvailable(localPort) {
		if localPort <= 1024 {
			return fmt.Errorf("local port %d is privileged, it requires root access", localPort)
		}
		return fmt.Errorf("local port %d is already in use in your local machine, please check your configuration", localPort)
	}

	return nil
}

// Add initializes a remote forward
func (fm *ForwardManager) Add(f model.Forward) error {

	if err := fm.canAdd(f.Local); err != nil {
		return err
	}

	fm.forwards[f.Local] = &forward{
		localAddress:  fmt.Sprintf("%s:%d", fm.localInterface, f.Local),
		remoteAddress: fmt.Sprintf("%s:%d", fm.remoteInterface, f.Remote),
		ctx:           fm.ctx,
	}

	if f.Service {
		fm.forwards[f.Local].remoteAddress = fmt.Sprintf("%s:%d", f.ServiceName, f.Remote)
	}

	return nil
}

// Start starts a port-forward to the remote port and then starts forwards and reverse forwards as goroutines
func (fm *ForwardManager) Start(devPod, namespace string) error {
	log.Info("starting SSH forward manager")
	if fm.pf != nil {
		if err := fm.pf.Start(devPod, namespace); err != nil {
			return fmt.Errorf("failed to start SSH port-forward: %w", err)
		}

		log.Info("port forward to dev pod connected")
	}

	c, err := getSSHClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get SSH configuration: %s", err)
	}

	log.Infof("starting SSH connection pool on %s", fm.sshAddr)
	pool, err := startPool(fm.ctx, fm.sshAddr, c)
	if err != nil {
		return err
	}

	for _, ff := range fm.forwards {
		ff.pool = pool
		go ff.start()
	}

	for _, rt := range fm.reverses {
		rt.pool = pool
		go rt.start()
	}

	return nil
}

// Stop sends a stop signal to all the connections
func (fm *ForwardManager) Stop() {
	// TODO stop forwards and reverses

	if fm.pf != nil {
		fm.pf.Stop()
	}

	log.Info("stopped SSH forward manager")
}
