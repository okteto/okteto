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
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestWaitUntilAppAwaken(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Cfg: &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	tt := []struct {
		expectedErr          error
		oktetoClientProvider *test.FakeK8sProvider
		name                 string
		autocreate           bool
	}{
		{
			name:        "dev is autocreate",
			autocreate:  true,
			expectedErr: nil,
		},
		{
			name:       "failed to provide k8s client",
			autocreate: false,
			oktetoClientProvider: &test.FakeK8sProvider{
				ErrProvide: assert.AnError,
			},
			expectedErr: assert.AnError,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			up := &upContext{
				Dev: &model.Dev{
					Autocreate: tc.autocreate,
				},
				K8sClientProvider: tc.oktetoClientProvider,
			}
			err := up.waitUntilAppIsAwaken(context.Background(), nil)
			assert.ErrorIs(t, tc.expectedErr, err)
		})
	}
}

func TestWaitUntilDevelopmentContainerIsRunning(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Cfg: &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	tt := []struct {
		expectedErr          error
		oktetoClientProvider *test.FakeK8sProvider
		name                 string
	}{
		{
			name: "failed to provide k8s client",
			oktetoClientProvider: &test.FakeK8sProvider{
				ErrProvide: assert.AnError,
			},
			expectedErr: assert.AnError,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			up := &upContext{
				Dev:               &model.Dev{},
				K8sClientProvider: tc.oktetoClientProvider,
			}
			err := up.waitUntilDevelopmentContainerIsRunning(context.Background(), nil)
			assert.ErrorIs(t, tc.expectedErr, err)
		})
	}
}
