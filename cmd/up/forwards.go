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

package up

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/okteto/okteto/internal/sshtransport"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	forwardk8s "github.com/okteto/okteto/pkg/k8s/forward"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type k8sForwardManagerFactory func(context.Context, string, *rest.Config, kubernetes.Interface, string) *forwardk8s.PortForwardManager
type sshForwardManagerFactory func(context.Context, string, string, string, *forwardk8s.PortForwardManager, string) forwarder
type sshPortForwardAdder func(*forwardk8s.PortForwardManager, forward.Forward) error
type sshEntryAdder func(name, host string, port int) error
type k8sConfigGetter func() *clientcmdapi.Config

func (up *upContext) createK8sForwardManager(ctx context.Context, iface string, restConfig *rest.Config, client kubernetes.Interface, namespace string) *forwardk8s.PortForwardManager {
	if up.newK8sForwardManager != nil {
		return up.newK8sForwardManager(ctx, iface, restConfig, client, namespace)
	}
	return forwardk8s.NewPortForwardManager(ctx, iface, restConfig, client, namespace)
}

func (up *upContext) createSSHForwardManager(ctx context.Context, sshAddr, localInterface, remoteInterface string, pf *forwardk8s.PortForwardManager, namespace string) forwarder {
	if up.newSSHForwardManager != nil {
		return up.newSSHForwardManager(ctx, sshAddr, localInterface, remoteInterface, pf, namespace)
	}
	return ssh.NewForwardManager(ctx, sshAddr, localInterface, remoteInterface, pf, namespace)
}

func (up *upContext) forwards(ctx context.Context) error {
	msg := "Configuring SSH tunnel to your development container..."
	if up.Dev.IsHybridModeEnabled() {
		msg = "Configuring reverse tunnel to your development environment..."
	}
	oktetoLog.Spinner(msg)
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	if up.Dev.RemoteModeEnabled() {
		return up.sshForwards(ctx)
	}

	k8sClient, restConfig, err := up.K8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}

	oktetoLog.Infof("starting port forwards")
	up.Forwarder = forwardk8s.NewPortForwardManager(ctx, up.Dev.Interface, restConfig, k8sClient, up.Namespace)

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

	err = up.Forwarder.Start(up.Pod.Name, up.Namespace)
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
	var cfg *clientcmdapi.Config
	if up.getK8sConfig != nil {
		cfg = up.getK8sConfig()
	} else {
		cfg = okteto.GetContext().Cfg
	}
	k8sClient, restConfig, err := up.K8sClientProvider.Provide(cfg)
	if err != nil {
		return err
	}

	// Port zero is intentionally preserved here. client-go will allocate and
	// hold the local listener atomically, and its concrete port is published
	// only after the forward reports ready.
	sshEndpoint, err := sshtransport.PlanRequested(up.Dev.Interface, up.Dev.RemotePort)
	if err != nil {
		return err
	}

	oktetoLog.Infof("starting SSH port forwards")
	pf := up.createK8sForwardManager(ctx, sshEndpoint.BindHost(), restConfig, k8sClient, up.Namespace)
	if pf == nil {
		return fmt.Errorf("failed to create SSH port-forward manager")
	}
	addSSHPortForward := up.addSSHPortForward
	if addSSHPortForward == nil {
		addSSHPortForward = func(pf *forwardk8s.PortForwardManager, f forward.Forward) error { return pf.Add(f) }
	}
	if err := addSSHPortForward(pf, forward.Forward{Local: up.Dev.RemotePort, Remote: up.Dev.SSHServerPort}); err != nil {
		return err
	}

	up.Forwarder = up.createSSHForwardManager(ctx, sshEndpoint.Address(), up.Dev.Interface, "0.0.0.0", pf, up.Namespace)
	if up.Forwarder == nil {
		return fmt.Errorf("failed to create SSH forward manager")
	}
	if len(up.Manifest.GlobalForward) > 0 {
		reserver, ok := up.Forwarder.(interface{ ReserveLocalPort(int) error })
		if !ok {
			up.Forwarder.Stop()
			return fmt.Errorf("SSH forward manager cannot reserve global-forward ports")
		}
		for _, globalForward := range up.Manifest.GlobalForward {
			if err := reserver.ReserveLocalPort(globalForward.Local); err != nil {
				up.Forwarder.Stop()
				return err
			}
		}
	}
	if err := up.Forwarder.Add(forward.Forward{Local: up.Sy.RemotePort, Remote: syncthing.ClusterPort}); err != nil {
		return err
	}

	if err := up.Forwarder.Add(forward.Forward{Local: up.Sy.RemoteGUIPort, Remote: syncthing.GUIPort}); err != nil {
		return err
	}

	addForwards := up.addToForwarderFn
	if addForwards == nil {
		addForwards = addToForwarder
	}
	if err := addForwards(up); err != nil {
		return err
	}

	err = up.Forwarder.Start(up.Pod.Name, up.Namespace)
	if err != nil {
		return err
	}

	portProvider, ok := up.Forwarder.(interface{ SSHPort() (int, error) })
	if !ok {
		up.Forwarder.Stop()
		return fmt.Errorf("SSH forward manager did not expose its bound port")
	}
	boundPort, err := portProvider.SSHPort()
	if err != nil {
		up.Forwarder.Stop()
		return fmt.Errorf("failed to resolve bound SSH port: %w", err)
	}
	addEntry := up.sshEntryAdder
	if addEntry == nil {
		addEntry = ssh.AddEntry
	}
	if err := addEntry(up.Dev.Name, sshEndpoint.DialHost(), boundPort); err != nil {
		up.Forwarder.Stop()
		oktetoLog.Infof("failed to add entry to your SSH config file: %s", err)
		return fmt.Errorf("failed to add entry to your SSH config file")
	}
	up.Dev.RemotePort = boundPort

	if isNeededGlobalForwarder(up.Manifest.GlobalForward) {
		up.GlobalForwarderStatus = make(chan error, 1)
		go up.setGlobalForwardsIfRequiredLoop(ctx)
	}

	return nil
}

func addToForwarder(up *upContext) error {
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(10 * time.Second)
	var forwardErr error
	alreadyAdded := map[int]bool{}
	for {
		select {
		case <-ticker.C:
			forwardErr = nil

			for idx, f := range up.Dev.Forward {
				if _, ok := alreadyAdded[f.Local]; ok {
					continue
				}
				if f.Labels != nil {
					forwardWithServiceName, err := up.Forwarder.TransformLabelsToServiceName(f)
					if err != nil {
						oktetoLog.Infof("could not create forward port: %s", err)
						forwardErr = err
						continue
					}
					up.Dev.Forward[idx] = forwardWithServiceName
					f = forwardWithServiceName
					alreadyAdded[f.Local] = true
				}
				if err := up.Forwarder.Add(f); err != nil {
					oktetoLog.Infof("could not create forward port: %s", err)
					forwardErr = err
					continue
				}
				alreadyAdded[f.Local] = true
			}
			if forwardErr != nil {
				continue
			}

			for _, r := range up.Dev.Reverse {
				if _, ok := alreadyAdded[r.Local]; ok {
					continue
				}
				if err := up.Forwarder.AddReverse(r); err != nil {
					oktetoLog.Infof("could not create reverse port: %s", err)
					forwardErr = err
					continue
				}
				alreadyAdded[r.Local] = true
			}

			if forwardErr != nil {
				continue
			}
			return nil
		case <-to.C:
			if forwardErr != nil {
				return forwardErr
			}
			return fmt.Errorf("could not create local ports after %s", up.Dev.Timeout.Resources.String())
		}
	}
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
