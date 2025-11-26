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

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log/io"
)

type DirectOktetoContextInterface interface {
	GetCurrentCertStr() string
	GetCurrentBuilder() string
	GetCurrentToken() string
}

type DirectConnector struct {
	buildkitClientFactory *ClientFactory
	waiter                *Waiter
}

// NewDirectConnector creates a new direct connector. It connects to the buildkit server directly.
func NewDirectConnector(okCtx DirectOktetoContextInterface, ioCtrl *io.Controller) *DirectConnector {
	buildkitClientFactory := NewBuildkitClientFactory(
		okCtx.GetCurrentCertStr(),
		okCtx.GetCurrentBuilder(),
		okCtx.GetCurrentToken(),
		config.GetCertificatePath(),
		ioCtrl)
	waiter := NewBuildkitClientWaiter(buildkitClientFactory, ioCtrl)

	return &DirectConnector{
		buildkitClientFactory: buildkitClientFactory,
		waiter:                waiter,
	}
}

// WaitUntilIsReady waits for the buildkit server to be ready
func (d *DirectConnector) WaitUntilIsReady(ctx context.Context) error {
	return d.waiter.WaitUntilIsUp(ctx)
}

// GetClientFactory returns the client factory
func (d *DirectConnector) GetClientFactory() *ClientFactory {
	return d.buildkitClientFactory
}

// GetWaiter returns the waiter
func (d *DirectConnector) GetWaiter() *Waiter {
	return d.waiter
}
