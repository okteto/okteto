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
	e := Exec{
		ioCtrl:            io.NewIOController(),
		k8sClientProvider: test.NewFakeK8sProvider(),
	} // Create an instance of the Exec struct

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
	executor, err := e.getExecutor(dev, podName)
	assert.NoError(t, err)
	assert.NotNil(t, executor)
	assert.IsType(t, &hybridExecutor{}, executor)

	// Test case 2: Remote mode enabled
	dev.Mode = constants.OktetoSyncModeFieldValue
	dev.RemotePort = 22000
	executor, err = e.getExecutor(dev, podName)
	assert.NoError(t, err)
	assert.NotNil(t, executor)
	assert.IsType(t, &sshExecutor{}, executor)

	// Test case 3: Neither hybrid nor remote mode enabled
	dev.Mode = constants.OktetoSyncModeFieldValue
	dev.RemotePort = 0
	t.Setenv("OKTETO_EXECUTE_SSH", "false")
	executor, err = e.getExecutor(dev, podName)
	assert.NoError(t, err)
	assert.NotNil(t, executor)
	assert.IsType(t, &k8sExecutor{}, executor)
}
