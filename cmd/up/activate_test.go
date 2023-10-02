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
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Cfg: &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	tt := []struct {
		name                 string
		autocreate           bool
		oktetoClientProvider *test.FakeK8sProvider
		expectedErr          error
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
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Cfg: &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	tt := []struct {
		name                 string
		oktetoClientProvider *test.FakeK8sProvider
		expectedErr          error
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
