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

//go:build !windows

package pipeline

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/cmd/pipeline"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/require"
)

// A Ctrl+C after a pipeline wait finished must not reach the channels it registered: os/signal panics
// when it sends to a closed channel, and dependencies finish while the main deploy is still running
func TestWaitUntilRunningUnregistersItsSignalChannel(t *testing.T) {
	// keeps the test process alive when the SIGINT arrives
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)
	t.Cleanup(func() { signal.Stop(interrupted) })
	pc := newWaitTestCommand(map[string]string{"svc": okteto.RunningStatus})

	err := pc.waitUntilRunning(context.Background(), "test", "test", &types.Action{ID: "test", Name: "test"}, time.Second, true, globalDeployOutput{})
	require.NoError(t, err)

	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
	select {
	case <-interrupted:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "SIGINT not received")
	}
}

// interruptAfter sends a SIGINT to the test process after d
func interruptAfter(t *testing.T, d time.Duration) {
	t.Helper()
	time.AfterFunc(d, func() { _ = syscall.Kill(os.Getpid(), syscall.SIGINT) })
}

func TestWaitForExistingDeployIsInterruptedDuringTheHealthCheck(t *testing.T) {
	previous := existingDeployPollInterval
	existingDeployPollInterval = 10 * time.Millisecond
	t.Cleanup(func() { existingDeployPollInterval = previous })
	// the resources never become ready
	pc := newWaitTestCommand(map[string]string{"svc": okteto.ProgressingStatus}, existingDeployConfigMap(pipeline.DeployedStatus))
	c, _, err := pc.k8sClientProvider.Provide(nil)
	require.NoError(t, err)
	opts := &DeployOptions{Name: "test", Namespace: "test", Timeout: time.Minute}
	interruptAfter(t, 200*time.Millisecond)

	err = pc.waitForExistingDeploy(context.Background(), c, pipeline.TranslatePipelineName("test"), opts)

	require.ErrorIs(t, err, oktetoErrors.ErrIntSig)
}

func TestWaitUntilRunningIsInterruptedWhileDrainingTheLogs(t *testing.T) {
	// the logs stream never reaches the end, so the wait drains it for logsDrainTimeout
	pc := newSlowStreamCommand(&slowStreamClient{delay: time.Hour})
	interruptAfter(t, 200*time.Millisecond)
	start := time.Now()

	err := pc.waitUntilRunning(context.Background(), "test", "test", &types.Action{ID: "test", Name: "test"}, time.Minute, true, globalDeployOutput{})

	require.ErrorIs(t, err, oktetoErrors.ErrIntSig)
	require.Less(t, time.Since(start), logsDrainTimeout)
}
