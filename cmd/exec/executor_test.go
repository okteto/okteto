// Copyright 2024 The Okteto Authors
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

package exec

import (
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestExec_getExecutor(t *testing.T) {
	e := executorProvider{
		ioCtrl:            io.NewIOController(),
		k8sClientProvider: test.NewFakeK8sProvider(),
	}

	dev := &model.Dev{} // Create a sample dev object
	podName := "test-pod"

	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Cfg: api.NewConfig(),
			},
		},
		CurrentContext: "test",
	}
	// Test case 1: Hybrid mode enabled
	dev.Mode = constants.OktetoHybridModeFieldValue
	executor, err := e.provide(dev, podName)
	assert.NoError(t, err)
	assert.NotNil(t, executor)
	assert.IsType(t, &hybridExecutor{}, executor)

	// Test case 2: Remote mode enabled
	dev.Mode = constants.OktetoSyncModeFieldValue
	dev.RemotePort = 22000
	executor, err = e.provide(dev, podName)
	assert.NoError(t, err)
	assert.NotNil(t, executor)
	assert.IsType(t, &sshExecutor{}, executor)

	// Test case 3: Neither hybrid nor remote mode enabled
	dev.Mode = constants.OktetoSyncModeFieldValue
	dev.RemotePort = 0
	t.Setenv("OKTETO_EXECUTE_SSH", "false")
	executor, err = e.provide(dev, podName)
	assert.NoError(t, err)
	assert.NotNil(t, executor)
	assert.IsType(t, &k8sExecutor{}, executor)
}
