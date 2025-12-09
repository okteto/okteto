// Copyright 2025 The Okteto Authors
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

package connector

import (
	"bytes"
	"context"
	"fmt"
	ioutil "io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type PortForwarderOktetoContextInterface interface {
	GetCurrentCertStr() string
	GetCurrentBuilder() string
	GetCurrentToken() string
	GetNamespace() string
	GetGlobalNamespace() string
	GetCurrentUser() string
	GetCurrentCfg() *clientcmdapi.Config
}

// forwarder represents a port forwarder for the buildkit server.
type forwarder struct {
	stopChan  chan struct{}
	readyChan chan struct{}
	localPort int
}

// PortForwarder represents a port forwarder for the buildkit server.
type PortForwarder struct {
	sessionID    string
	okCtx        PortForwarderOktetoContextInterface
	oktetoClient types.OktetoInterface
	k8sClient    kubernetes.Interface
	restConfig   *rest.Config
	forwarder    *forwarder
	ioCtrl       *io.Controller
	mu           sync.Mutex
	isActive     bool
}

// NewPortForwarder creates a new port forwarder. It forwards the port to the buildkit server.
func NewPortForwarder(ctx context.Context, okCtx PortForwarderOktetoContextInterface, ioCtrl *io.Controller) (*PortForwarder, error) {
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, fmt.Errorf("could not create okteto client: %w", err)
	}
	sessionID := uuid.New().String()

	k8sClient, restConfig, err := getPortForwardK8sClient(oktetoClient, okCtx)
	if err != nil {
		return nil, fmt.Errorf("could not get port forward k8s client: %w", err)
	}

	// Use IANA ephemeral port range for buildkit port forwarding to minimize conflicts
	port, err := model.GetAvailablePortInRange(model.Localhost, model.IANAEphemeralPortStart, model.IANAEphemeralPortEnd)
	if err != nil {
		return nil, fmt.Errorf("could not get available port: %w", err)
	}

	return &PortForwarder{
		sessionID:    sessionID,
		oktetoClient: oktetoClient,
		k8sClient:    k8sClient,
		restConfig:   restConfig,
		okCtx:        okCtx,
		forwarder: &forwarder{
			stopChan:  make(chan struct{}, 1),
			readyChan: make(chan struct{}, 1),
			localPort: port,
		},
		ioCtrl: ioCtrl,
	}, nil
}

// Start establishes the port forward connection to the buildkit pod
func (pf *PortForwarder) Start(ctx context.Context, podName string, remotePort int) error {
	pf.forwarder.stopChan = make(chan struct{}, 1)
	pf.forwarder.readyChan = make(chan struct{}, 1)

	// Use the CoreV1 REST client from the clientset, which already has GroupVersion configured
	url := pf.k8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pf.okCtx.GetGlobalNamespace()).
		Name(podName).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(pf.restConfig)
	if err != nil {
		return fmt.Errorf("failed to create SPDY round tripper: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)

	ports := []string{fmt.Sprintf("%d:%d", pf.forwarder.localPort, remotePort)}
	out := new(bytes.Buffer)

	// Use 127.0.0.1 explicitly to avoid IPv6 resolution issues with "localhost"
	forwarder, err := portforward.NewOnAddresses(
		dialer,
		[]string{"127.0.0.1"},
		ports,
		pf.forwarder.stopChan,
		pf.forwarder.readyChan,
		ioutil.Discard,
		out,
	)
	if err != nil {
		return fmt.Errorf("failed to create port forwarder: %w", err)
	}

	// Start the port forward in a goroutine
	go func() {
		if err := forwarder.ForwardPorts(); err != nil {
			pf.ioCtrl.Logger().Infof("port forward to buildkit finished with error: %s", err)
		}
	}()

	return nil
}

// Stop closes the port forward connection gracefully
func (pf *PortForwarder) Stop() {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	if pf.forwarder == nil || pf.forwarder.stopChan == nil {
		pf.ioCtrl.Logger().Infof("port forward connection is not active")
		pf.isActive = false
		return
	}

	// Safely close the channel to avoid panic on double close
	select {
	case <-pf.forwarder.stopChan:
		// Channel already closed, nothing to do
	default:
		// Channel is still open, close it
		close(pf.forwarder.stopChan)
		pf.ioCtrl.Logger().Infof("port forward connection stopped")
	}
	pf.isActive = false
}

func getPortForwardK8sClient(oktetoClient *okteto.Client, okCtx PortForwarderOktetoContextInterface) (kubernetes.Interface, *rest.Config, error) {
	kubetoken, err := oktetoClient.Kubetoken().GetKubeToken(okteto.GetContext().Name, okCtx.GetNamespace(), "buildkit")
	if err != nil {
		return nil, nil, fmt.Errorf("could not get kubetoken: %w", err)
	}

	portForwardCfg := okCtx.GetCurrentCfg().DeepCopy()

	portForwardCfg.AuthInfos[okCtx.GetCurrentUser()].Token = kubetoken.Status.Token
	return okteto.NewK8sClientProvider().Provide(portForwardCfg)
}

// WaitUntilIsReady waits for the buildkit server to be ready
func (pf *PortForwarder) WaitUntilIsReady(ctx context.Context) error {
	// If already active, reuse the existing port forward
	pf.mu.Lock()
	if pf.isActive {
		pf.mu.Unlock()
		pf.ioCtrl.Logger().Infof("reusing existing port forward on 127.0.0.1:%d", pf.forwarder.localPort)
		return nil
	}
	pf.mu.Unlock()

	const (
		maxWaitTime         = 10 * time.Minute
		initialPollInterval = 1 * time.Second
		maxPollInterval     = 10 * time.Second
		backoffMultiplier   = 2.0
	)

	startTime := time.Now()
	pollInterval := initialPollInterval

	sp := pf.ioCtrl.Out().Spinner("Waiting for BuildKit pod to become available...")
	sp.Start()
	defer sp.Stop()
	for {
		if time.Since(startTime) >= maxWaitTime {
			return fmt.Errorf("timeout waiting for buildkit pod after %v: please contact your cluster administrator to increase the maximum number of BuildKit instances or adjust the metrics thresholds", maxWaitTime)
		}

		response, err := pf.oktetoClient.Buildkit().GetLeastLoadedBuildKitPod(ctx, pf.sessionID)
		if err != nil {
			return fmt.Errorf("could not get least loaded buildkit pod: %w", err)
		}

		if response.PodName != "" {
			pf.ioCtrl.Logger().Info("BuildKit pod is ready, establishing connection...")

			const buildkitPort = 1234
			if err := pf.Start(ctx, response.PodName, buildkitPort); err != nil {
				return fmt.Errorf("could not start port forward: %w", err)
			}

			pf.ioCtrl.Logger().Infof("waiting for port forward to be ready to pod %s", response.PodName)
			select {
			case <-pf.forwarder.readyChan:
				pf.mu.Lock()
				pf.isActive = true
				pf.mu.Unlock()
				pf.ioCtrl.Logger().Infof("port forward to buildkit pod %s is ready on 127.0.0.1:%d", response.PodName, pf.forwarder.localPort)
				return nil
			case <-ctx.Done():
				pf.Stop()
				return fmt.Errorf("context cancelled while waiting for port forward: %w", ctx.Err())
			}
		}

		if response.TotalInQueue > 0 {
			pf.ioCtrl.Logger().Infof("Waiting for BuildKit: %s (position %d of %d in queue)", response.Reason, response.QueuePosition, response.TotalInQueue)
			sp.Stop()
			sp = pf.ioCtrl.Out().Spinner(fmt.Sprintf("Waiting for BuildKit: %s (position %d of %d in queue)", response.Reason, response.QueuePosition, response.TotalInQueue))
			sp.Start()
		}

		select {
		case <-time.After(pollInterval):
			pollInterval = time.Duration(float64(pollInterval) * backoffMultiplier)
			if pollInterval > maxPollInterval {
				pollInterval = maxPollInterval
			}
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for buildkit pod: %w", ctx.Err())
		}
	}
}

// GetClientFactory returns the client factory
func (pf *PortForwarder) GetClientFactory() *ClientFactory {
	// Use 127.0.0.1 explicitly to match the port forward binding
	localAddress := fmt.Sprintf("127.0.0.1:%d", pf.forwarder.localPort)
	pf.ioCtrl.Logger().Infof("using buildkit via local port forward: %s", localAddress)
	originalURL, err := url.Parse(pf.okCtx.GetCurrentBuilder())
	if err != nil {
		pf.ioCtrl.Logger().Infof("failed to parse original builder URL: %s", err)
		return nil
	}
	originalHostname := originalURL.Hostname()
	buildkitClientFactory := NewBuildkitClientFactory(
		pf.okCtx.GetCurrentCertStr(), // Keep certificate for TLS
		"tcp://"+localAddress,        // Local forwarded address
		pf.okCtx.GetCurrentToken(),   // Keep token for auth
		config.GetCertificatePath(),
		pf.ioCtrl)

	buildkitClientFactory.SetTLSServerName(originalHostname)
	pf.ioCtrl.Logger().Infof("TLS verification will use server name: %s", originalHostname)
	return buildkitClientFactory
}

// GetWaiter returns the waiter
func (pf *PortForwarder) GetWaiter() *Waiter {
	// Create waiter for the port-forwarded buildkit client
	return NewBuildkitClientWaiter(pf.GetClientFactory(), pf.ioCtrl)
}
