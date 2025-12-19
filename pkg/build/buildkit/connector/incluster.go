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
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
)

// InClusterOktetoContextInterface defines the interface for the okteto context needed by InClusterConnector
type InClusterOktetoContextInterface interface {
	GetCurrentCertStr() string
	GetCurrentBuilder() string
	GetCurrentToken() string
}

// InClusterConnector manages a direct connection to the buildkit server via pod IP.
// This is used for remote commands, installer, and pods in Okteto managed namespaces.
type InClusterConnector struct {
	sessionID    string
	okCtx        InClusterOktetoContextInterface
	oktetoClient types.OktetoInterface
	ioCtrl       *io.Controller
	maxWaitTime  time.Duration
	waiter       *Waiter

	// Connection state
	podIP string
	mu    sync.Mutex

	// Metrics collector for analytics
	metrics *ConnectorMetrics
}

// NewInClusterConnector creates a new in-cluster connector that connects to buildkit via pod IP.
func NewInClusterConnector(ctx context.Context, okCtx InClusterOktetoContextInterface, ioCtrl *io.Controller) (*InClusterConnector, error) {
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, fmt.Errorf("could not create okteto client: %w", err)
	}

	sessionID := uuid.New().String()
	maxWaitTime := env.LoadTimeOrDefault(buildkitQueueWaitTimeoutEnvVar, defaultMaxWaitTimePortForward)
	waiter := NewBuildkitClientWaiter(ioCtrl)

	ic := &InClusterConnector{
		sessionID:    sessionID,
		okCtx:        okCtx,
		oktetoClient: oktetoClient,
		ioCtrl:       ioCtrl,
		maxWaitTime:  maxWaitTime,
		waiter:       waiter,
		metrics:      NewConnectorMetrics(analytics.ConnectorTypeInCluster, sessionID),
	}

	// Verify that the buildkit pod endpoint is available (like PortForwarder)
	response, err := ic.oktetoClient.Buildkit().GetLeastLoadedBuildKitPod(ctx, ic.sessionID)
	if err != nil {
		return nil, fmt.Errorf("could not get least loaded buildkit pod: %w", err)
	}

	if response.PodIP != "" {
		ic.podIP = response.PodIP
	}

	return ic, nil
}

// Start establishes the connection to the buildkit pod.
// It always verifies the connection by checking if the client can return info.
// If the existing pod IP is no longer valid, it requests a new one.
func (ic *InClusterConnector) Start(ctx context.Context) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// If we have a podIP, verify it still works by checking client info
	if ic.podIP != "" {
		if err := ic.verifyConnection(ctx); err != nil {
			ic.ioCtrl.Logger().Infof("existing pod IP %s no longer valid: %s, will request new pod", ic.podIP, err)
			ic.podIP = ""
			ic.metrics.SetPodReused(false)
		} else {
			ic.metrics.SetPodReused(true)
			ic.ioCtrl.Logger().Infof("verified buildkit connection is working, reusing pod IP: %s", ic.podIP)
			return nil
		}
	}

	// Request a new pod from the API
	podIP, err := ic.assignBuildkitPod(ctx)
	if err != nil {
		return fmt.Errorf("failed to assign buildkit pod: %w", err)
	}
	ic.podIP = podIP

	return nil
}

// verifyConnection checks that we can connect to the current pod IP
func (ic *InClusterConnector) verifyConnection(ctx context.Context) error {
	// Use a short timeout for verification
	verifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	c, err := ic.createBuildkitClient(verifyCtx)
	if err != nil {
		return err
	}
	defer c.Close()

	_, err = c.Info(verifyCtx)
	return err
}

// assignBuildkitPod gets the least loaded buildkit pod with queue waiting support
func (ic *InClusterConnector) assignBuildkitPod(ctx context.Context) (string, error) {
	pollInterval := initialPollIntervalPortForward

	// Start tracking metrics
	ic.metrics.StartTracking()

	sp := ic.ioCtrl.Out().Spinner("Waiting for BuildKit pod to become available...")
	sp.Start()
	defer sp.Stop()

	for {
		if time.Since(ic.metrics.StartTime) >= ic.maxWaitTime {
			ic.metrics.SetWaitingForPodTimedOut(true)
			ic.metrics.TrackFailure()
			return "", fmt.Errorf("timeout waiting for buildkit pod after %v: please contact your cluster administrator to increase the maximum number of BuildKit instances or adjust the metrics thresholds", ic.maxWaitTime)
		}

		response, err := ic.oktetoClient.Buildkit().GetLeastLoadedBuildKitPod(ctx, ic.sessionID)
		if err != nil {
			ic.metrics.TrackFailure()
			return "", fmt.Errorf("could not get least loaded buildkit pod: %w", err)
		}

		// Capture queue metrics
		ic.metrics.RecordQueueStatus(response.QueuePosition, response.Reason)

		if response.PodIP != "" {
			ic.ioCtrl.Logger().Infof("assigned buildkit pod IP: %s", response.PodIP)
			ic.metrics.TrackSuccess()
			return response.PodIP, nil
		}

		if response.TotalInQueue > 0 {
			friendlyReason := waitReasonMessages[response.Reason]
			if friendlyReason == "" {
				friendlyReason = response.Reason
			}
			ic.ioCtrl.Logger().Infof("Waiting for BuildKit: %s (position %d of %d in queue)",
				response.Reason, response.QueuePosition, response.TotalInQueue)
			sp.Stop()
			sp = ic.ioCtrl.Out().Spinner(fmt.Sprintf("Waiting for BuildKit: %s (position %d of %d in queue)",
				friendlyReason, response.QueuePosition, response.TotalInQueue))
			sp.Start()
		}

		select {
		case <-time.After(pollInterval):
			pollInterval = time.Duration(float64(pollInterval) * backoffMultiplierPortForward)
			if pollInterval > maxPollIntervalPortForward {
				pollInterval = maxPollIntervalPortForward
			}
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled while waiting for buildkit pod: %w", ctx.Err())
		}
	}
}

// createBuildkitClient creates a buildkit client with TLS certificates (like PortForwarder)
func (ic *InClusterConnector) createBuildkitClient(ctx context.Context) (*client.Client, error) {
	address := fmt.Sprintf("tcp://%s:%d", ic.podIP, buildkitPort)
	ic.ioCtrl.Logger().Infof("creating buildkit client for address: %s", address)

	// Get the original hostname from the builder URL for TLS verification
	originalURL, err := url.Parse(ic.okCtx.GetCurrentBuilder())
	if err != nil {
		ic.ioCtrl.Logger().Infof("failed to parse original builder URL: %s", err)
		return nil, fmt.Errorf("failed to parse original builder URL: %w", err)
	}
	originalHostname := originalURL.Hostname()

	clientFactory := NewBuildkitClientFactory(
		ic.okCtx.GetCurrentCertStr(),
		address,
		ic.okCtx.GetCurrentToken(),
		config.GetCertificatePath(),
		ic.ioCtrl,
	)
	clientFactory.SetTLSServerName(originalHostname)
	ic.ioCtrl.Logger().Infof("TLS verification will use server name: %s", originalHostname)

	return clientFactory.GetBuildkitClient(ctx)
}

// GetBuildkitClient returns a new buildkit client (does not cache, connection is lazy)
func (ic *InClusterConnector) GetBuildkitClient(ctx context.Context) (*client.Client, error) {
	ic.mu.Lock()
	podIP := ic.podIP
	ic.mu.Unlock()

	if podIP == "" {
		return nil, fmt.Errorf("no buildkit pod IP available")
	}

	return ic.createBuildkitClient(ctx)
}

// WaitUntilIsReady waits for the buildkit server to be ready.
func (ic *InClusterConnector) WaitUntilIsReady(ctx context.Context) error {
	ic.mu.Lock()
	hasPodIP := ic.podIP != ""
	ic.mu.Unlock()

	if !hasPodIP {
		if err := ic.Start(ctx); err != nil {
			return fmt.Errorf("failed to start in-cluster connection: %w", err)
		}
	}
	return ic.waiter.WaitUntilIsUp(ctx, ic.GetBuildkitClient)
}

// Stop clears podIP to force a new pod assignment and connection verification on next Start()
func (ic *InClusterConnector) Stop() {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	if ic.podIP == "" {
		ic.ioCtrl.Logger().Infof("in-cluster connection has no pod IP assigned")
		return
	}

	ic.ioCtrl.Logger().Infof("clearing pod IP %s, will request new pod on next Start()", ic.podIP)
	ic.podIP = ""
}

// GetMetrics returns the connector metrics for external configuration
func (ic *InClusterConnector) GetMetrics() *ConnectorMetrics {
	return ic.metrics
}
