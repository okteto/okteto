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
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/okteto/okteto/internal/sshtransport"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	k8sForward "github.com/okteto/okteto/pkg/k8s/forward"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	forwardModel "github.com/okteto/okteto/pkg/model/forward"
	gossh "golang.org/x/crypto/ssh"
	"k8s.io/client-go/tools/portforward"
)

const (
	maxSystemPorts = 1024
)

var errAutomaticSSHPortCollision = errors.New("automatically allocated SSH port conflicts with another local forward")

type kubePortForwarder interface {
	StartContext(context.Context, string, string) error
	ForwardedPorts() ([]portforward.ForwardedPort, error)
	Stop()
	GetServiceNameByLabel(string, map[string]string) (string, error)
}

// ForwardManager handles the lifecycle of all the forwards
type ForwardManager struct {
	localInterface     string
	remoteInterface    string
	forwards           map[int]*forward
	globalForwards     map[int]*forward
	reverses           map[int]*reverse
	reservedLocalPorts map[int]struct{}
	ctx                context.Context
	cancel             context.CancelFunc
	sshAddr            string
	sshHost            string
	requestedSSHPort   int
	resolvedSSHPort    int
	automaticSSHPort   bool
	sshAddrErr         error
	pf                 kubePortForwarder
	pool               *pool
	namespace          string
	getClientConfig    func() (*gossh.ClientConfig, error)
	startPool          func(context.Context, context.Context, string, *gossh.ClientConfig) (*pool, error)
	isPortAvailable    func(string, int) bool
	lifecycleMu        sync.Mutex
	stopped            bool
}

// NewForwardManager returns a newly initialized instance of ForwardManager
func NewForwardManager(ctx context.Context, sshAddr, localInterface, remoteInterface string, pf *k8sForward.PortForwardManager, namespace string) *ForwardManager {
	var kubePF kubePortForwarder
	if pf != nil {
		kubePF = pf
	}
	return newForwardManager(ctx, sshAddr, localInterface, remoteInterface, kubePF, namespace)
}

func newForwardManager(ctx context.Context, sshAddr, localInterface, remoteInterface string, pf kubePortForwarder, namespace string) *ForwardManager {
	lifetimeCtx, cancel := context.WithCancel(ctx)
	host, rawPort, err := net.SplitHostPort(sshAddr)
	requestedPort := -1
	if err == nil {
		requestedPort, err = strconv.Atoi(rawPort)
	}
	var endpoint sshtransport.Endpoint
	if err == nil {
		endpoint, err = sshtransport.PlanRequested(host, requestedPort)
	}
	if err == nil {
		host = endpoint.DialHost()
		sshAddr = endpoint.Address()
	}

	return &ForwardManager{
		ctx:                lifetimeCtx,
		cancel:             cancel,
		localInterface:     localInterface,
		remoteInterface:    remoteInterface,
		forwards:           make(map[int]*forward),
		globalForwards:     make(map[int]*forward),
		reverses:           make(map[int]*reverse),
		reservedLocalPorts: make(map[int]struct{}),
		sshAddr:            sshAddr,
		sshHost:            host,
		requestedSSHPort:   requestedPort,
		automaticSSHPort:   requestedPort == 0,
		sshAddrErr:         err,
		pf:                 pf,
		namespace:          namespace,
	}
}

func (fm *ForwardManager) canAdd(localPort int, checkAvailable bool) error {
	if fm.resolvedSSHPort > 0 && localPort == fm.resolvedSSHPort {
		return fmt.Errorf("port %d conflicts with the local SSH tunnel", localPort)
	}
	if !fm.automaticSSHPort && fm.requestedSSHPort > 0 && localPort == fm.requestedSSHPort {
		return fmt.Errorf("port %d conflicts with the local SSH tunnel", localPort)
	}
	if _, ok := fm.reverses[localPort]; ok {
		return fmt.Errorf("port %d is listed multiple times, please check your reverse forwards configuration", localPort)
	}

	if _, ok := fm.forwards[localPort]; ok {
		return fmt.Errorf("port %d is listed multiple times, please check your forwards configuration", localPort)
	}

	if _, ok := fm.globalForwards[localPort]; ok {
		return fmt.Errorf("port %d is listed multiple times, please check your global forwards configuration", localPort)
	}

	if !checkAvailable {
		return nil
	}

	isPortAvailable := fm.isPortAvailable
	if isPortAvailable == nil {
		isPortAvailable = model.IsPortAvailable
	}
	if !isPortAvailable(fm.localInterface, localPort) {
		if localPort <= maxSystemPorts {
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

		return fmt.Errorf("local port %d is already in-use in your local machine: %w", localPort, oktetoErrors.ErrPortAlreadyAllocated)
	}

	return nil
}

// ReserveLocalPort prevents automatic SSH allocation from selecting a port
// that will be bound by a global forward after startup.
func (fm *ForwardManager) ReserveLocalPort(localPort int) error {
	if localPort < 1 || localPort > 65535 {
		return fmt.Errorf("invalid reserved local port %d", localPort)
	}
	if _, reserved := fm.reservedLocalPorts[localPort]; reserved {
		return fmt.Errorf("port %d is listed multiple times in global forwards", localPort)
	}
	if err := fm.canAdd(localPort, false); err != nil {
		return err
	}
	fm.reservedLocalPorts[localPort] = struct{}{}
	return nil
}

// Add initializes a remote forward
func (fm *ForwardManager) Add(f forwardModel.Forward) error {
	if _, reserved := fm.reservedLocalPorts[f.Local]; reserved && !f.IsGlobal {
		return fmt.Errorf("port %d conflicts with a configured global forward", f.Local)
	}

	forwardsToUpdate := fm.forwards
	if f.IsGlobal {
		forwardsToUpdate = fm.globalForwards
	}

	if err := fm.canAdd(f.Local, true); err != nil {
		return err
	}

	forwardsToUpdate[f.Local] = &forward{
		localAddress:  net.JoinHostPort(fm.localInterface, strconv.Itoa(f.Local)),
		remoteAddress: net.JoinHostPort(fm.remoteInterface, strconv.Itoa(f.Remote)),
	}

	if f.Service {
		forwardsToUpdate[f.Local].remoteAddress = net.JoinHostPort(f.ServiceName, strconv.Itoa(f.Remote))
	}
	delete(fm.reservedLocalPorts, f.Local)

	return nil
}

// Start starts a port-forward to the remote port and then starts forwards and reverse forwards as goroutines
func (fm *ForwardManager) Start(devPod, namespace string) error {
	oktetoLog.Info("starting SSH forward manager")
	fm.lifecycleMu.Lock()
	stopped := fm.stopped
	fm.lifecycleMu.Unlock()
	if stopped {
		return errors.New("SSH forward manager is stopped")
	}
	if err := fm.ctx.Err(); err != nil {
		return fmt.Errorf("SSH forward manager context is done: %w", err)
	}
	if fm.sshAddrErr != nil {
		return fmt.Errorf("invalid SSH endpoint %q: %w", fm.sshAddr, fm.sshAddrErr)
	}
	getClientConfig := fm.getClientConfig
	if getClientConfig == nil {
		getClientConfig = getSSHClientConfig
	}
	c, err := getClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get SSH configuration: %w", err)
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeoutDuration := c.Timeout
	if timeoutDuration <= 0 {
		timeoutDuration = 10 * time.Second
	}
	startupCtx, cancel := context.WithTimeout(fm.ctx, timeoutDuration)
	defer cancel()
	retries := 0

	for {
		retries++
		oktetoLog.Infof("SSH forward manager retry %d", retries)
		if fm.pf != nil {
			if err := fm.pf.StartContext(startupCtx, devPod, namespace); err != nil {
				fm.pf.Stop()
				return fmt.Errorf("failed to start SSH port-forward: %w", err)
			}
			ports, err := fm.pf.ForwardedPorts()
			if err == nil {
				err = fm.setSSHAddressFromForwardedPorts(ports)
			}
			if err != nil {
				fm.pf.Stop()
				if !errors.Is(err, errAutomaticSSHPortCollision) {
					return fmt.Errorf("failed to resolve SSH port-forward endpoint: %w", err)
				}
				oktetoLog.Infof("retrying automatic SSH port allocation after collision: %s", err)
				if startupCtx.Err() != nil {
					return fmt.Errorf("failed to allocate a non-conflicting SSH port: %w", err)
				}
				select {
				case <-ticker.C:
					continue
				case <-startupCtx.Done():
					return fmt.Errorf("ForwardManager.Start cancelled: %w", startupCtx.Err())
				}
			}

			oktetoLog.Info("k8s port forward to dev pod connected")
		}

		sshPort, err := fm.SSHPort()
		if err == nil {
			_, err = sshtransport.LocalAddress(fm.sshHost, sshPort)
		}
		if err != nil {
			if fm.pf != nil {
				fm.pf.Stop()
			}
			return fmt.Errorf("invalid concrete SSH endpoint: %w", err)
		}

		oktetoLog.Infof("starting SSH connection pool on %s", fm.sshAddr)
		startConnectionPool := fm.startPool
		if startConnectionPool == nil {
			startConnectionPool = startPoolWithContexts
		}
		pool, err := startConnectionPool(fm.ctx, startupCtx, fm.sshAddr, c)
		if err == nil {
			if fm.pf != nil {
				ports, liveErr := fm.pf.ForwardedPorts()
				if liveErr == nil {
					liveErr = fm.verifyResolvedSSHPort(ports)
				}
				if liveErr != nil {
					pool.stop()
					err = fmt.Errorf("SSH port-forward lost ownership during connection setup: %w", liveErr)
				} else if err = fm.activatePool(pool, startupCtx); err == nil {
					return nil
				} else {
					pool.stop()
					if fm.pf != nil {
						fm.pf.Stop()
					}
					return err
				}
			} else if err = fm.activatePool(pool, startupCtx); err == nil {
				return nil
			} else {
				pool.stop()
				return err
			}
		}
		oktetoLog.Infof("error starting SSH connection pool on %s: %s", fm.sshAddr, err.Error())
		if startupCtx.Err() != nil {
			if fm.pf != nil {
				fm.pf.Stop()
			}
			return oktetoErrors.ErrSSHConnectError
		}

		if fm.pf != nil {
			fm.pf.Stop()
		}

		select {
		case <-ticker.C:
			continue
		case <-startupCtx.Done():
			oktetoLog.Infof("ForwardManager.Start cancelled")
			return fmt.Errorf("ForwardManager.Start cancelled: %w", startupCtx.Err())
		}
	}
}

func (fm *ForwardManager) activatePool(p *pool, startupCtx context.Context) error {
	fm.lifecycleMu.Lock()
	defer fm.lifecycleMu.Unlock()

	if fm.stopped {
		return errors.New("SSH forward manager was stopped during startup")
	}
	if err := fm.ctx.Err(); err != nil {
		return fmt.Errorf("SSH forward manager context is done: %w", err)
	}
	if err := startupCtx.Err(); err != nil {
		return fmt.Errorf("SSH forward manager startup timed out: %w", err)
	}
	if fm.pool != nil {
		return errors.New("SSH forward manager already has an active connection pool")
	}

	fm.pool = p
	for _, ff := range fm.forwards {
		ff.pool = p
		go ff.start(fm.ctx)
	}
	for _, rt := range fm.reverses {
		rt.pool = p
		go rt.start(fm.ctx)
	}

	return nil
}

func (fm *ForwardManager) setSSHAddressFromForwardedPorts(ports []portforward.ForwardedPort) error {
	if len(ports) != 1 {
		return fmt.Errorf("expected exactly one SSH port-forward, got %d", len(ports))
	}

	resolvedPort := int(ports[0].Local)
	if !fm.automaticSSHPort && resolvedPort != fm.requestedSSHPort {
		return fmt.Errorf("SSH port-forward bound port %d instead of requested port %d", resolvedPort, fm.requestedSSHPort)
	}
	if _, ok := fm.forwards[resolvedPort]; ok {
		return fm.sshPortCollisionError(resolvedPort)
	}
	if _, ok := fm.globalForwards[resolvedPort]; ok {
		return fm.sshPortCollisionError(resolvedPort)
	}
	if _, ok := fm.reverses[resolvedPort]; ok {
		return fm.sshPortCollisionError(resolvedPort)
	}
	if _, ok := fm.reservedLocalPorts[resolvedPort]; ok {
		return fm.sshPortCollisionError(resolvedPort)
	}

	address, err := sshtransport.ConcreteAddress(fm.sshHost, resolvedPort)
	if err != nil {
		return err
	}
	fm.sshAddr = address
	fm.resolvedSSHPort = resolvedPort
	return nil
}

func (fm *ForwardManager) sshPortCollisionError(port int) error {
	if fm.automaticSSHPort {
		return fmt.Errorf("%w: port %d", errAutomaticSSHPortCollision, port)
	}
	return fmt.Errorf("SSH tunnel port %d conflicts with another local forward", port)
}

func (fm *ForwardManager) verifyResolvedSSHPort(ports []portforward.ForwardedPort) error {
	if len(ports) != 1 {
		return fmt.Errorf("expected exactly one live SSH port-forward, got %d", len(ports))
	}
	if got := int(ports[0].Local); got != fm.resolvedSSHPort {
		return fmt.Errorf("live SSH port-forward changed from port %d to %d", fm.resolvedSSHPort, got)
	}
	return nil
}

// SSHPort returns the concrete local port used by the internal SSH transport.
// It is only valid after Start has completed successfully.
func (fm *ForwardManager) SSHPort() (int, error) {
	host, rawPort, err := net.SplitHostPort(fm.sshAddr)
	if err != nil {
		return 0, fmt.Errorf("invalid SSH endpoint %q: %w", fm.sshAddr, err)
	}

	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return 0, fmt.Errorf("invalid SSH endpoint port %q", rawPort)
	}
	if _, err := sshtransport.ConcreteAddress(host, port); err != nil {
		return 0, err
	}

	return port, nil
}

// Stop sends a stop signal to all the connections
func (fm *ForwardManager) Stop() {
	fm.lifecycleMu.Lock()
	fm.stopped = true
	if fm.cancel != nil {
		fm.cancel()
	}
	p := fm.pool
	fm.lifecycleMu.Unlock()

	if p != nil {
		p.stop()
	}

	if fm.pf != nil {
		fm.pf.Stop()
	}

	oktetoLog.Info("stopped SSH forward manager")
}

func (fm *ForwardManager) TransformLabelsToServiceName(f forwardModel.Forward) (forwardModel.Forward, error) {
	serviceName, err := fm.pf.GetServiceNameByLabel(fm.namespace, f.Labels)
	if err != nil {
		return f, err
	}
	f.ServiceName = serviceName
	return f, nil
}

// StartGlobalForwarding implements from the interface types.forwarder
// nolint:unparam
func (fm *ForwardManager) StartGlobalForwarding() error {
	fm.lifecycleMu.Lock()
	if fm.stopped || fm.pool == nil {
		fm.lifecycleMu.Unlock()
		return errors.New("SSH forward manager is not running")
	}
	p := fm.pool
	fm.lifecycleMu.Unlock()

	for _, gf := range fm.globalForwards {
		gf.pool = p
		go gf.start(fm.ctx)
	}

	return nil
}
