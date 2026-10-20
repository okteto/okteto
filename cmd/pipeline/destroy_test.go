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

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/okteto/okteto/internal/test/client"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDestroyPipelineSuccessful(t *testing.T) {
	ctx := context.Background()
	response := &client.FakePipelineResponses{
		DestroyResponse: &types.GitDeployResponse{
			Action: &types.Action{
				ID:   "test",
				Name: "test",
			},
		},
	}
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
		},
	}
	opts := &DestroyOptions{
		Name: "test",
	}
	err := pc.ExecuteDestroyPipeline(ctx, opts)
	assert.NoError(t, err)
}

func TestDestroyPipelineSuccessfulWithWait(t *testing.T) {
	ctx := context.Background()
	response := &client.FakePipelineResponses{
		DestroyResponse: &types.GitDeployResponse{
			Action: &types.Action{
				ID:   "test",
				Name: "test",
			},
		},
	}
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
			StreamClient:   client.NewFakeStreamClient(&client.FakeStreamResponse{}),
		},
	}
	opts := &DestroyOptions{
		Name: "test",
		Wait: true,
	}
	err := pc.ExecuteDestroyPipeline(ctx, opts)
	assert.NoError(t, err)
}

func TestDestroyPipelineSuccessfulWithWaitStreamErr(t *testing.T) {
	ctx := context.Background()
	response := &client.FakePipelineResponses{
		DestroyResponse: &types.GitDeployResponse{
			Action: &types.Action{
				ID:   "test",
				Name: "test",
			},
		},
	}
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
			StreamClient:   client.NewFakeStreamClient(&client.FakeStreamResponse{StreamErr: errors.New("error")}),
		},
	}
	opts := &DestroyOptions{
		Name: "test",
		Wait: true,
	}
	err := pc.ExecuteDestroyPipeline(ctx, opts)
	assert.NoError(t, err)
}

func TestDestroyNonExistentPipeline(t *testing.T) {
	ctx := context.Background()
	response := &client.FakePipelineResponses{
		DestroyErr: fmt.Errorf("not found"),
	}
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
		},
	}
	opts := &DestroyOptions{
		Name: "no exists",
	}
	err := pc.ExecuteDestroyPipeline(ctx, opts)
	assert.NoError(t, err)
}

func TestDestroyNonExistentPipelineWithWait(t *testing.T) {
	ctx := context.Background()
	response := &client.FakePipelineResponses{
		DestroyErr: fmt.Errorf("not found"),
		DestroyResponse: &types.GitDeployResponse{
			Action: &types.Action{},
		},
	}
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
			StreamClient:   client.NewFakeStreamClient(&client.FakeStreamResponse{}),
		},
	}

	opts := &DestroyOptions{
		Name: "no exists",
		Wait: true,
	}
	err := pc.ExecuteDestroyPipeline(ctx, opts)
	assert.NoError(t, err)
}

func TestDestroyExistentPipelineWithError(t *testing.T) {
	ctx := context.Background()
	pipelineErr := fmt.Errorf("test error")
	response := &client.FakePipelineResponses{
		DestroyErr: pipelineErr,
	}
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
		},
	}

	opts := &DestroyOptions{
		Name: "no exists",
	}
	err := pc.ExecuteDestroyPipeline(ctx, opts)
	assert.ErrorIs(t, err, pipelineErr)
}

func TestDestroyExistentPipelineTimeoutError(t *testing.T) {
	ctx := context.Background()
	pipelineErr := fmt.Errorf("test error")
	response := &client.FakePipelineResponses{
		DestroyResponse: &types.GitDeployResponse{
			Action: &types.Action{},
		},
		WaitErr: pipelineErr,
	}
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
			StreamClient:   client.NewFakeStreamClient(&client.FakeStreamResponse{}),
		},
	}

	opts := &DestroyOptions{
		Name:    "test",
		Wait:    true,
		Timeout: 0 * time.Second,
	}
	err := pc.ExecuteDestroyPipeline(ctx, opts)
	assert.ErrorIs(t, err, pipelineErr)
}

// initLogBuffer resets the output buffer. Lines are only recorded in it under a stage
func initLogBuffer(t *testing.T) {
	t.Helper()
	oktetoLog.Init(logrus.WarnLevel)
	oktetoLog.SetStage("Destroying")
	t.Cleanup(func() { oktetoLog.SetStage("") })
}

func newDestroyWaitTestCommand(stream *slowStreamClient) *Command {
	return &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(&client.FakePipelineResponses{}),
			StreamClient:   stream,
		},
	}
}

func TestWaitUntilDestroyedKeepsTheLastLogLines(t *testing.T) {
	initLogBuffer(t)
	pc := newDestroyWaitTestCommand(&slowStreamClient{lines: []string{"last line"}, delay: 100 * time.Millisecond})

	err := pc.waitUntilDestroyed(context.Background(), "db", "test", &types.Action{ID: "a", Name: "a"}, time.Second)

	require.NoError(t, err)
	require.Contains(t, oktetoLog.GetOutputBuffer().String(), "last line")
}

func TestWaitUntilDestroyedDoesNotWarnWhenTheStreamIsCancelled(t *testing.T) {
	previous := logsDrainTimeout
	logsDrainTimeout = 50 * time.Millisecond
	t.Cleanup(func() { logsDrainTimeout = previous })
	initLogBuffer(t)
	// the stream never reaches the end: it is cancelled after the drain timeout
	pc := newDestroyWaitTestCommand(&slowStreamClient{delay: time.Hour})

	err := pc.waitUntilDestroyed(context.Background(), "db", "test", &types.Action{ID: "a", Name: "a"}, time.Second)

	require.NoError(t, err)
	require.NotContains(t, oktetoLog.GetOutputBuffer().String(), "error streaming pipeline logs")
}
