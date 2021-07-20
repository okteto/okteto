// Copyright 2021 The Okteto Authors
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
	"runtime"
	"time"

	"github.com/okteto/okteto/pkg/errors"
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
	pool            *pool
	namespace       string
}

// NewForwardManager returns a newly initialized instance of ForwardManager
func NewForwardManager(ctx context.Context, sshAddr, localInterface, remoteInterface string, pf *k8sforward.PortForwardManager, namespace string) *ForwardManager {
	return &ForwardManager{
		ctx:             ctx,
		localInterface:  localInterface,
		remoteInterface: remoteInterface,
		forwards:        make(map[int]*forward),
		reverses:        make(map[int]*reverse),
		sshAddr:         sshAddr,
		pf:              pf,
		namespace:       namespace,
	}
}

func (fm *ForwardManager) canAdd(localPort int, checkAvailable bool) error {
	if _, ok := fm.reverses[localPort]; ok {
		return fmt.Errorf("port %d is listed multiple times, please check your reverse forwards configuration", localPort)
	}

	if _, ok := fm.forwards[localPort]; ok {
		return fmt.Errorf("port %d is listed multiple times, please check your forwards configuration", localPort)
	}

	if !checkAvailable {
		return nil
	}

	if !model.IsPortAvailable(fm.localInterface, localPort) {
		if localPort <= 1024 {
			os := runtime.GOOS
			switch os {
			case "darwin":
				if fm.localInterface == model.Localhost {
					return fmt.Errorf("local port %d is privileged. Define 'interface: 0.0.0.0' in your okteto manifest and try again", localPort)
				}
			case "linux":
				return fmt.Errorf("local port %d is privileged. Try running \"sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/okteto\" and try again", localPort)
			}
		}
		return fmt.Errorf("local port %d is already in-use in your local machine", localPort)
	}

	return nil
}

// Add initializes a remote forward
func (fm *ForwardManager) Add(f model.Forward) error {

	if err := fm.canAdd(f.Local, true); err != nil {
		return err
	}

	fm.forwards[f.Local] = &forward{
		localAddress:  fmt.Sprintf("%s:%d", fm.localInterface, f.Local),
		remoteAddress: fmt.Sprintf("%s:%d", fm.remoteInterface, f.Remote),
	}

	if f.Service {
		fm.forwards[f.Local].remoteAddress = fmt.Sprintf("%s:%d", f.ServiceName, f.Remote)
	}

	return nil
}

// Start starts a port-forward to the remote port and then starts forwards and reverse forwards as goroutines
func (fm *ForwardManager) Start(devPod, namespace string) error {
	log.Info("starting SSH forward manager")

	ticker := time.NewTicker(200 * time.Millisecond)
	to := time.Now().Add(10 * time.Second)
	retries := 0

	for {
		retries++
		log.Infof("SSH forward manager retry %d", retries)
		if fm.pf != nil {
			if err := fm.pf.Start(devPod, namespace); err != nil {
				return fmt.Errorf("failed to start SSH port-forward: %w", err)
			}

			log.Info("k8s port forward to dev pod connected")
		}

		c, err := getSSHClientConfig()
		if err != nil {
			return fmt.Errorf("failed to get SSH configuration: %s", err)
		}

		log.Infof("starting SSH connection pool on %s", fm.sshAddr)
		pool, err := startPool(fm.ctx, fm.sshAddr, c)
		if err == nil {
			fm.pool = pool
			break
		}
		log.Infof("error starting SSH connection pool on %s: %s", fm.sshAddr, err.Error())
		if time.Now().After(to) && retries > 10 {
			return errors.ErrSSHConnectError
		}

		if fm.pf != nil {
			fm.pf.Stop()
		}

		select {
		case <-ticker.C:
			continue
		case <-fm.ctx.Done():
			log.Infof("ForwardManager.Start cancelled")
			return fmt.Errorf("ForwardManager.Start cancelled")
		}

	}

	for _, ff := range fm.forwards {
		ff.pool = fm.pool
		go ff.start(fm.ctx)

	}

	for _, rt := range fm.reverses {
		rt.pool = fm.pool
		go rt.start(fm.ctx)
	}

	return nil
}

// Stop sends a stop signal to all the connections
func (fm *ForwardManager) Stop() {

	if fm.pool != nil {
		fm.pool.stop()
	}

	if fm.pf != nil {
		fm.pf.Stop()
	}

	log.Info("stopped SSH forward manager")
}

func (fm *ForwardManager) TransformLabelsToServiceName(f model.Forward) (model.Forward, error) {
	serviceName, err := fm.pf.GetServiceNameByLabel(fm.namespace, f.Labels)
	if err != nil {
		return f, err
	}
	f.ServiceName = serviceName
	return f, nil
}
