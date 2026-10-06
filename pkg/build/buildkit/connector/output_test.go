// Copyright 2026 The Okteto Authors
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
	"testing"

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/stretchr/testify/require"
)

func TestPortForwarderUseOutput(t *testing.T) {
	original, quiet := io.NewIOController(), io.NewIOController()
	pf := &PortForwarder{ioCtrl: original, waiter: &Waiter{logger: original}}

	previous := pf.UseOutput(quiet)

	require.Same(t, original, previous)
	require.Same(t, quiet, pf.ioCtrl)
	require.Same(t, quiet, pf.waiter.logger)
}

func TestInClusterConnectorUseOutput(t *testing.T) {
	original, quiet := io.NewIOController(), io.NewIOController()
	ic := &InClusterConnector{ioCtrl: original}

	previous := ic.UseOutput(quiet)

	require.Same(t, original, previous)
	require.Same(t, quiet, ic.ioCtrl)
}

func TestIngressConnectorUseOutput(t *testing.T) {
	original, quiet := io.NewIOController(), io.NewIOController()
	ingress := &IngressConnector{waiter: &Waiter{logger: original}, buildkitClientFactory: &ClientFactory{logger: original}}

	previous := ingress.UseOutput(quiet)

	require.Same(t, original, previous)
	require.Same(t, quiet, ingress.waiter.logger)
	require.Same(t, quiet, ingress.buildkitClientFactory.logger)
}
