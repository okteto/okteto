// Copyright 2022 The Okteto Authors
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

package up

import (
	"context"
	"errors"
	"fmt"
	"time"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	forwardk8s "github.com/okteto/okteto/pkg/k8s/forward"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
)

func (up *upContext) forwards(ctx context.Context) error {
	oktetoLog.Spinner("Configuring SSH tunnel to your development container...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	if up.Dev.RemoteModeEnabled() {
		return up.sshForwards(ctx)
	}

	oktetoLog.Infof("starting port forwards")
	up.Forwarder = forwardk8s.NewPortForwardManager(ctx, up.Dev.Interface, up.RestConfig, up.Client, up.Dev.Namespace)

	for idx, f := range up.Dev.Forward {
		if f.Labels != nil {
			forwardWithServiceName, err := up.Forwarder.TransformLabelsToServiceName(f)
			if err != nil {
				return err
			}
			up.Dev.Forward[idx] = forwardWithServiceName
			f = forwardWithServiceName
		}
		if err := up.Forwarder.Add(f); err != nil {
			return err
		}
	}

	if err := up.Forwarder.Add(forward.Forward{Local: up.Sy.RemotePort, Remote: syncthing.ClusterPort}); err != nil {
		return err
	}

	if err := up.Forwarder.Add(forward.Forward{Local: up.Sy.RemoteGUIPort, Remote: syncthing.GUIPort}); err != nil {
		return err
	}

	err := up.Forwarder.Start(up.Pod.Name, up.Dev.Namespace)
	if err != nil {
		return err
	}

	if isNeededGlobalForwarder(up.Manifest.GlobalForward) {
		up.GlobalForwarderStatus = make(chan error, 1)
		go up.setGlobalForwardsIfRequiredLoop(ctx)
	}

	return nil
}

func (up *upContext) sshForwards(ctx context.Context) error {
	oktetoLog.Infof("starting SSH port forwards")
	f := forwardk8s.NewPortForwardManager(ctx, up.Dev.Interface, up.RestConfig, up.Client, up.Dev.Namespace)
	if err := f.Add(forward.Forward{Local: up.Dev.RemotePort, Remote: up.Dev.SSHServerPort}); err != nil {
		return err
	}

	up.Forwarder = ssh.NewForwardManager(ctx, fmt.Sprintf(":%d", up.Dev.RemotePort), up.Dev.Interface, "0.0.0.0", f, up.Dev.Namespace)

	if err := up.Forwarder.Add(forward.Forward{Local: up.Sy.RemotePort, Remote: syncthing.ClusterPort}); err != nil {
		return err
	}

	if err := up.Forwarder.Add(forward.Forward{Local: up.Sy.RemoteGUIPort, Remote: syncthing.GUIPort}); err != nil {
		return err
	}

	for idx, f := range up.Dev.Forward {
		if f.Labels != nil {
			forwardWithServiceName, err := up.Forwarder.TransformLabelsToServiceName(f)
			if err != nil {
				return err
			}
			up.Dev.Forward[idx] = forwardWithServiceName
			f = forwardWithServiceName
		}
		if err := up.Forwarder.Add(f); err != nil {
			return err
		}
	}

	for _, r := range up.Dev.Reverse {
		if err := up.Forwarder.AddReverse(r); err != nil {
			return err
		}
	}

	if err := ssh.AddEntry(up.Dev.Name, up.Dev.Interface, up.Dev.RemotePort); err != nil {
		oktetoLog.Infof("failed to add entry to your SSH config file: %s", err)
		return fmt.Errorf("failed to add entry to your SSH config file")
	}

	err := up.Forwarder.Start(up.Pod.Name, up.Dev.Namespace)
	if err != nil {
		return err
	}

	if isNeededGlobalForwarder(up.Manifest.GlobalForward) {
		up.GlobalForwarderStatus = make(chan error, 1)
		go up.setGlobalForwardsIfRequiredLoop(ctx)
	}

	return nil
}

func (up *upContext) setGlobalForwardsIfRequiredLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)

	for {
		if !isNeededGlobalForwarder(up.Manifest.GlobalForward) {
			return
		}

		select {
		case <-ticker.C:
			err := addGlobalForwards(up)
			if err != nil {
				up.GlobalForwarderStatus <- err
				return
			}

			err = up.Forwarder.StartGlobalForwarding()
			if err != nil {
				up.GlobalForwarderStatus <- err
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func isNeededGlobalForwarder(globalForwards []forward.GlobalForward) bool {
	for _, f := range globalForwards {
		if !f.IsAdded {
			return true
		}
	}

	return false
}

func addGlobalForwards(up *upContext) error {
	for idx, gf := range up.Manifest.GlobalForward {
		if gf.IsAdded {
			continue
		}

		f := forward.Forward{
			Local:       gf.Local,
			Remote:      gf.Remote,
			Service:     true,
			IsGlobal:    true,
			ServiceName: gf.ServiceName,
			Labels:      gf.Labels,
		}

		if gf.Labels != nil {
			forwardWithServiceName, err := up.Forwarder.TransformLabelsToServiceName(f)
			if err != nil {
				return err
			}
			up.Manifest.GlobalForward[idx].ServiceName = forwardWithServiceName.ServiceName
			f = forwardWithServiceName
		}

		err := up.Forwarder.Add(f)
		if err != nil {
			if !errors.Is(err, oktetoErrors.ErrPortAlreadyAllocated) {
				return err
			}
		} else {
			up.Manifest.GlobalForward[idx].IsAdded = true
		}
	}

	return nil
}
