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

	"github.com/google/uuid"
	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log/io"
)

type IngressOktetoContextInterface interface {
	GetCurrentCertStr() string
	GetCurrentBuilder() string
	GetCurrentToken() string
}

type IngressConnector struct {
	buildkitClientFactory *ClientFactory
	waiter                *Waiter
	metrics               *ConnectorMetrics
}

// NewIngressConnector creates a new ingress connector. It connects to the buildkit server via ingress.
func NewIngressConnector(okCtx IngressOktetoContextInterface, ioCtrl *io.Controller) *IngressConnector {
	buildkitClientFactory := NewBuildkitClientFactory(
		okCtx.GetCurrentCertStr(),
		okCtx.GetCurrentBuilder(),
		okCtx.GetCurrentToken(),
		config.GetCertificatePath(),
		ioCtrl)
	waiter := NewBuildkitClientWaiter(ioCtrl)
	sessionID := uuid.New().String()

	return &IngressConnector{
		buildkitClientFactory: buildkitClientFactory,
		waiter:                waiter,
		metrics:               NewConnectorMetrics(analytics.ConnectorTypeIngress, sessionID),
	}
}

// Start is a no-op for the ingress connector since it doesn't maintain a persistent connection
func (i *IngressConnector) Start(ctx context.Context) error {
	i.metrics.StartTracking()
	return nil
}

// WaitUntilIsReady waits for the buildkit server to be ready
func (i *IngressConnector) WaitUntilIsReady(ctx context.Context) error {
	err := i.waiter.WaitUntilIsUp(ctx, i.GetBuildkitClient)
	i.metrics.SetServiceReadyDuration(i.waiter.GetWaitingTime())
	if err != nil {
		i.metrics.TrackFailure()
	} else {
		i.metrics.TrackSuccess()
	}
	return err
}

func (i *IngressConnector) GetBuildkitClient(ctx context.Context) (*client.Client, error) {
	return i.buildkitClientFactory.GetBuildkitClient(ctx)
}

// Stop is a no-op for the ingress connector since it doesn't maintain a persistent connection
func (i *IngressConnector) Stop() {
	// No-op: ingress connector doesn't maintain a persistent connection that needs to be closed
}

// GetMetrics returns the connector metrics for external configuration
func (i *IngressConnector) GetMetrics() *ConnectorMetrics {
	return i.metrics
}
