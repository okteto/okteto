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

package connect

import (
	"context"
	"fmt"

	forwardk8s "github.com/okteto/okteto/pkg/k8s/forward"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
)

// forwards sets up SSH port forwarding for syncthing.
// Since connect always uses remote mode (dev.LoadRemote is called before activation),
// we always take the SSH path — no direct k8s port-forward for syncthing.
func (c *connectContext) forwards(ctx context.Context) error {
	oktetoLog.Spinner("Configuring SSH tunnel to your development container...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	k8sClient, restConfig, err := c.K8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}

	oktetoLog.Infof("starting SSH port forwards")
	f := forwardk8s.NewPortForwardManager(ctx, c.Dev.Interface, restConfig, k8sClient, c.Namespace)
	if err := f.Add(forward.Forward{Local: c.Dev.RemotePort, Remote: c.Dev.SSHServerPort}); err != nil {
		return err
	}

	c.Forwarder = ssh.NewForwardManager(ctx, fmt.Sprintf(":%d", c.Dev.RemotePort), c.Dev.Interface, "0.0.0.0", f, c.Namespace)
	if err := c.Forwarder.Add(forward.Forward{Local: c.Sy.RemotePort, Remote: syncthing.ClusterPort}); err != nil {
		return err
	}

	if err := c.Forwarder.Add(forward.Forward{Local: c.Sy.RemoteGUIPort, Remote: syncthing.GUIPort}); err != nil {
		return err
	}

	if err := ssh.AddEntry(c.Dev.Name, c.Dev.Interface, c.Dev.RemotePort); err != nil {
		oktetoLog.Infof("failed to add entry to your SSH config file: %s", err)
		return fmt.Errorf("failed to add entry to your SSH config file")
	}

	return c.Forwarder.Start(c.Pod.Name, c.Namespace)
}
