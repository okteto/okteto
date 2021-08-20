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

package up

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/k8s/forward"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
)

func (up *upContext) forwards(ctx context.Context) error {
	spinner := utils.NewSpinner("Configuring SSH tunnel to your development container...")
	spinner.Start()
	up.spinner = spinner
	defer spinner.Stop()

	if up.Dev.RemoteModeEnabled() {
		return up.sshForwards(ctx)
	}

	log.Infof("starting port forwards")
	up.Forwarder = forward.NewPortForwardManager(ctx, up.Dev.Interface, up.RestConfig, up.Client, up.Dev.Namespace)

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

	if err := up.Forwarder.Add(model.Forward{Local: up.Sy.RemotePort, Remote: syncthing.ClusterPort}); err != nil {
		return err
	}

	if err := up.Forwarder.Add(model.Forward{Local: up.Sy.RemoteGUIPort, Remote: syncthing.GUIPort}); err != nil {
		return err
	}

	return up.Forwarder.Start(up.Pod.Name, up.Dev.Namespace)
}

func (up *upContext) sshForwards(ctx context.Context) error {
	log.Infof("starting SSH port forwards")
	f := forward.NewPortForwardManager(ctx, up.Dev.Interface, up.RestConfig, up.Client, up.Dev.Namespace)
	if err := f.Add(model.Forward{Local: up.Dev.RemotePort, Remote: up.Dev.SSHServerPort}); err != nil {
		return err
	}

	up.Forwarder = ssh.NewForwardManager(ctx, fmt.Sprintf(":%d", up.Dev.RemotePort), up.Dev.Interface, "0.0.0.0", f, up.Dev.Namespace)

	if err := up.Forwarder.Add(model.Forward{Local: up.Sy.RemotePort, Remote: syncthing.ClusterPort}); err != nil {
		return err
	}

	if err := up.Forwarder.Add(model.Forward{Local: up.Sy.RemoteGUIPort, Remote: syncthing.GUIPort}); err != nil {
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
		log.Infof("failed to add entry to your SSH config file: %s", err)
		return fmt.Errorf("failed to add entry to your SSH config file")
	}

	return up.Forwarder.Start(up.Pod.Name, up.Dev.Namespace)
}
