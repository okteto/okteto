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

	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log/io"
)

type IngressOktetoContextInterface interface {
	GetCurrentCertStr() string
	GetCurrentBuilder() string
	GetCurrentToken() string
}

type IngressConnector struct {
	okCtx                 IngressOktetoContextInterface
	buildkitClientFactory *ClientFactory
	waiter                *Waiter
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

	return &IngressConnector{
		okCtx:                 okCtx,
		buildkitClientFactory: buildkitClientFactory,
		waiter:                waiter,
	}
}

// NewSibling creates another ingress connector to the same BuildKit endpoint with its own
// client factory and readiness waiter, so concurrent builds don't share their state
func (i *IngressConnector) NewSibling(ioCtrl *io.Controller) *IngressConnector {
	return NewIngressConnector(i.okCtx, ioCtrl)
}

// Start is a no-op for the ingress connector since it doesn't maintain a persistent connection
func (i *IngressConnector) Start(ctx context.Context) error {
	return nil
}

// WaitUntilIsReady waits for the buildkit server to be ready
func (i *IngressConnector) WaitUntilIsReady(ctx context.Context) error {
	return i.waiter.WaitUntilIsUp(ctx, i.GetBuildkitClient)
}

func (i *IngressConnector) GetBuildkitClient(ctx context.Context) (*client.Client, error) {
	return i.buildkitClientFactory.GetBuildkitClient(ctx)
}

// GetType returns the connector type name for logging
func (i *IngressConnector) GetType() string {
	return "ingress"
}

// Stop is a no-op for the ingress connector since it doesn't maintain a persistent connection
func (i *IngressConnector) Stop() {
	// No-op: ingress connector doesn't maintain a persistent connection that needs to be closed
}

// UseOutput makes the connector print its readiness status through ioCtrl and returns
// the controller it used before
func (i *IngressConnector) UseOutput(ioCtrl *io.Controller) *io.Controller {
	previous := i.waiter.logger
	i.waiter.logger = ioCtrl
	i.buildkitClientFactory.logger = ioCtrl
	return previous
}
