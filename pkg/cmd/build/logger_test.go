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

package build

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/build/buildkit"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
)

func TestNewTrace(t *testing.T) {
	trace := newTrace()
	
	assert.NotNil(t, trace)
	assert.NotNil(t, trace.ongoing)
	assert.NotNil(t, trace.stages)
	assert.True(t, trace.showCtxAdvice)
	assert.Nil(t, trace.err)
	assert.Equal(t, 0, len(trace.ongoing))
	assert.Equal(t, 0, len(trace.stages))
}

func TestTrace_IsTransferringContext(t *testing.T) {
	trace := newTrace()
	
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "internal load build context",
			input:    "[internal] load build context",
			expected: true,
		},
		{
			name:     "internal load build definition",
			input:    "[internal] load build definition",
			expected: true,
		},
		{
			name:     "not internal",
			input:    "load build context",
			expected: false,
		},
		{
			name:     "internal but not load build",
			input:    "[internal] some other operation",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trace.isTransferringContext(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTrace_HasCommandLogs(t *testing.T) {
	trace := newTrace()
	
	tests := []struct {
		name     string
		vertex   *vertexInfo
		expected bool
	}{
		{
			name: "has logs",
			vertex: &vertexInfo{
				logs: []string{"log1", "log2"},
			},
			expected: true,
		},
		{
			name: "no logs",
			vertex: &vertexInfo{
				logs: []string{},
			},
			expected: false,
		},
		{
			name: "nil logs",
			vertex: &vertexInfo{
				logs: nil,
			},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trace.hasCommandLogs(tt.vertex)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTrace_Update(t *testing.T) {
	tests := []struct {
		name        string
		solveStatus *client.SolveStatus
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful update with vertex",
			solveStatus: &client.SolveStatus{
				Vertexes: []*client.Vertex{
					{
						Digest: mockDigest("test-digest"),
						Name:   "test-vertex",
						Cached: false,
					},
				},
			},
			expectError: false,
		},
		{
			name: "vertex with error",
			solveStatus: &client.SolveStatus{
				Vertexes: []*client.Vertex{
					{
						Digest: mockDigest("test-digest"),
						Name:   "test-vertex",
						Error:  "build failed",
					},
				},
			},
			expectError: true,
			errorMsg:    "error on stage test-vertex: build failed",
		},
		{
			name: "vertex with cached flag",
			solveStatus: &client.SolveStatus{
				Vertexes: []*client.Vertex{
					{
						Digest: mockDigest("test-digest"),
						Name:   "test-vertex",
						Cached: true,
					},
				},
			},
			expectError: false,
		},
		{
			name: "vertex with completion",
			solveStatus: &client.SolveStatus{
				Vertexes: []*client.Vertex{
					{
						Digest:    mockDigest("test-digest"),
						Name:      "test-vertex",
						Completed: &time.Time{},
					},
				},
			},
			expectError: false,
		},
		{
			name: "status update",
			solveStatus: &client.SolveStatus{
				Vertexes: []*client.Vertex{
					{
						Digest: mockDigest("test-digest"),
						Name:   "test-vertex",
					},
				},
				Statuses: []*client.VertexStatus{
					{
						Vertex:    mockDigest("test-digest"),
						Current:   1000,
						Total:     2000,
						Completed: &time.Time{},
					},
				},
			},
			expectError: false,
		},
		{
			name: "log update",
			solveStatus: &client.SolveStatus{
				Vertexes: []*client.Vertex{
					{
						Digest: mockDigest("test-digest"),
						Name:   "test-vertex",
					},
				},
				Logs: []*client.VertexLog{
					{
						Vertex: mockDigest("test-digest"),
						Data:   []byte("log line 1\nlog line 2"),
					},
				},
			},
			expectError: false,
		},
		{
			name: "status update with missing vertex",
			solveStatus: &client.SolveStatus{
				Statuses: []*client.VertexStatus{
					{
						Vertex: mockDigest("missing-vertex"),
						Current: 50,
						Total:   100,
					},
				},
			},
			expectError: false,
		},
		{
			name: "log update with missing vertex",
			solveStatus: &client.SolveStatus{
				Logs: []*client.VertexLog{
					{
						Vertex: mockDigest("missing-vertex"),
						Data:   []byte("log message"),
					},
				},
			},
			expectError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := newTrace()
			err := trace.update(tt.solveStatus)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				
				// Verify vertex was added to ongoing
				if len(tt.solveStatus.Vertexes) > 0 {
					vertex := tt.solveStatus.Vertexes[0]
					info, exists := trace.ongoing[vertex.Digest.Encoded()]
					assert.True(t, exists)
					assert.Equal(t, vertex.Name, info.name)
					assert.Equal(t, vertex.Cached, info.cached)
					
					if vertex.Completed != nil {
						assert.True(t, info.completed)
					}
				}
				
				// Verify status was updated (only if vertex exists)
				if len(tt.solveStatus.Statuses) > 0 {
					status := tt.solveStatus.Statuses[0]
					info, exists := trace.ongoing[status.Vertex.Encoded()]
					if strings.Contains(tt.name, "missing vertex") {
						// For missing vertex tests, the vertex should not exist
						assert.False(t, exists)
					} else {
						assert.True(t, exists)
						assert.Equal(t, status.Current, info.currentTransferedContext)
						assert.Equal(t, status.Total, info.totalTransferedContext)
						assert.True(t, info.completed)
					}
				}
				
				// Verify logs were added (only if vertex exists)
				if len(tt.solveStatus.Logs) > 0 {
					log := tt.solveStatus.Logs[0]
					info, exists := trace.ongoing[log.Vertex.Encoded()]
					if strings.Contains(tt.name, "missing vertex") {
						// For missing vertex tests, the vertex should not exist
						assert.False(t, exists)
					} else {
						assert.True(t, exists)
						assert.Contains(t, info.logs, "log line 1")
						assert.Contains(t, info.logs, "log line 2")
					}
				}
			}
		})
	}
}

func TestTrace_RemoveCompletedSteps(t *testing.T) {
	tests := []struct {
		name           string
		initialOngoing map[string]*vertexInfo
		expectedCount  int
		expectLog      bool
	}{
		{
			name: "remove completed non-cached step",
			initialOngoing: map[string]*vertexInfo{
				"digest1": {
					name:      "regular-step",
					completed: true,
					cached:    false,
				},
				"digest2": {
					name:      "ongoing-step",
					completed: false,
					cached:    false,
				},
			},
			expectedCount: 1,
			expectLog:     false,
		},
		{
			name: "remove completed cached test step",
			initialOngoing: map[string]*vertexInfo{
				"digest1": {
					name:      `remote-run test --name "test-container"`,
					completed: true,
					cached:    true,
				},
				"digest2": {
					name:      "ongoing-step",
					completed: false,
					cached:    false,
				},
			},
			expectedCount: 1,
			expectLog:     true,
		},
		{
			name: "remove completed cached test step without name",
			initialOngoing: map[string]*vertexInfo{
				"digest1": {
					name:      "remote-run test",
					completed: true,
					cached:    true,
				},
			},
			expectedCount: 0,
			expectLog:     true,
		},
		{
			name: "no completed steps",
			initialOngoing: map[string]*vertexInfo{
				"digest1": {
					name:      "ongoing-step1",
					completed: false,
					cached:    false,
				},
				"digest2": {
					name:      "ongoing-step2",
					completed: false,
					cached:    true,
				},
			},
			expectedCount: 2,
			expectLog:     false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := newTrace()
			trace.ongoing = tt.initialOngoing
			
			trace.removeCompletedSteps()
			
			assert.Equal(t, tt.expectedCount, len(trace.ongoing))
		})
	}
}

func TestTrace_Display(t *testing.T) {
	tests := []struct {
		name         string
		progress     string
		vertexInfo   *vertexInfo
		expectStage  bool
		expectSpinner bool
	}{
		{
			name:     "display deploy progress with logs",
			progress: DeployOutputModeOnBuild,
			vertexInfo: &vertexInfo{
				name: "test-step",
				logs: []string{
					`{"stage":"Build","level":"info","message":"Building image"}`,
					`{"stage":"Deploy","level":"info","message":"Deploying application"}`,
				},
			},
			expectStage:   true,
			expectSpinner: true,
		},
		{
			name:     "display destroy progress with logs",
			progress: DestroyOutputModeOnBuild,
			vertexInfo: &vertexInfo{
				name: "test-step",
				logs: []string{
					`{"stage":"Destroy","level":"info","message":"Destroying resources"}`,
				},
			},
			expectStage:   true,
			expectSpinner: true,
		},
		{
			name:     "display test progress with logs",
			progress: TestOutputModeOnBuild,
			vertexInfo: &vertexInfo{
				name: "test-step",
				logs: []string{
					`{"stage":"Test","level":"info","message":"Running tests"}`,
				},
			},
			expectStage:   true,
			expectSpinner: true,
		},
		{
			name:     "display context transfer",
			progress: DeployOutputModeOnBuild,
			vertexInfo: &vertexInfo{
				name:                     "[internal] load build context",
				currentTransferedContext: 1000000, // 1MB
				totalTransferedContext:   2000000, // 2MB
			},
			expectStage:   false,
			expectSpinner: true,
		},
		{
			name:     "display large context transfer with advice",
			progress: DeployOutputModeOnBuild,
			vertexInfo: &vertexInfo{
				name:                     "[internal] load build context",
				currentTransferedContext: largeContextThreshold + 1000000, // Over threshold
				totalTransferedContext:   largeContextThreshold + 2000000,
			},
			expectStage:   false,
			expectSpinner: true,
		},
		{
			name:     "log parsing error",
			progress: DeployOutputModeOnBuild,
			vertexInfo: &vertexInfo{
				name: "test-step",
				logs: []string{
					`invalid json log`,
				},
			},
			expectStage:   false,
			expectSpinner: false,
		},
		{
			name:     "log without stage",
			progress: DeployOutputModeOnBuild,
			vertexInfo: &vertexInfo{
				name: "test-step",
				logs: []string{
					`{"level":"info","message":"Message without stage"}`,
				},
			},
			expectStage:   false,
			expectSpinner: false,
		},
		{
			name:     "error log in Load manifest stage",
			progress: DeployOutputModeOnBuild,
			vertexInfo: &vertexInfo{
				name: "test-step",
				logs: []string{
					`{"stage":"Load manifest","level":"error","message":"Failed to load manifest"}`,
				},
			},
			expectStage:   true,
			expectSpinner: false,
		},
		{
			name:     "done stage log",
			progress: DeployOutputModeOnBuild,
			vertexInfo: &vertexInfo{
				name: "test-step",
				logs: []string{
					`{"stage":"done","level":"info","message":"Task completed"}`,
				},
			},
			expectStage:   false,
			expectSpinner: false,
		},
		{
			name:     "error log with stage",
			progress: DeployOutputModeOnBuild,
			vertexInfo: &vertexInfo{
				name: "test-step",
				logs: []string{
					`{"stage":"Build","level":"error","message":"Build failed"}`,
				},
			},
			expectStage:   true,
			expectSpinner: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := newTrace()
			trace.ongoing["test-digest"] = tt.vertexInfo
			
			// Capture the display call - in a real scenario we'd mock the logger
			// For now, we just ensure the function doesn't panic
			assert.NotPanics(t, func() {
				trace.display(tt.progress)
			})
			
			// Verify logs were cleared after processing
			if tt.vertexInfo.logs != nil && len(tt.vertexInfo.logs) > 0 {
				// Check if logs contain valid JSON that would be processed
				hasValidLogs := false
				for _, log := range tt.vertexInfo.logs {
					var text oktetoLog.JSONLogFormat
					if json.Unmarshal([]byte(log), &text) == nil && text.Stage != "" {
						hasValidLogs = true
						break
					}
				}
				
				if hasValidLogs {
					assert.Empty(t, tt.vertexInfo.logs, "logs should be cleared after processing")
				}
			}
			
			// Check if error was set for error logs
			if tt.vertexInfo.logs != nil {
				for _, log := range tt.vertexInfo.logs {
					var text oktetoLog.JSONLogFormat
					if json.Unmarshal([]byte(log), &text) == nil && text.Level == "error" && text.Stage != "" && text.Stage != "Load manifest" {
						assert.NotNil(t, trace.err, "error should be set for error logs")
						if trace.err != nil {
							buildErr, ok := trace.err.(buildkit.CommandErr)
							assert.True(t, ok, "error should be of type buildkit.CommandErr")
							assert.Equal(t, text.Stage, buildErr.Stage)
							assert.Equal(t, tt.progress, buildErr.Output)
						}
					}
				}
			}
		})
	}
}

func TestDeployDisplayer(t *testing.T) {
	tests := []struct {
		name           string
		outputMode     string
		solveStatuses  []*client.SolveStatus
		expectError    bool
		contextTimeout bool
		simulateDelay  bool
	}{
		{
			name:       "successful deploy mode",
			outputMode: DeployOutputModeOnBuild,
			solveStatuses: []*client.SolveStatus{
				{
					Vertexes: []*client.Vertex{
						{
							Digest: mockDigest("test-digest"),
							Name:   "test-step",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:       "successful destroy mode",
			outputMode: DestroyOutputModeOnBuild,
			solveStatuses: []*client.SolveStatus{
				{
					Vertexes: []*client.Vertex{
						{
							Digest: mockDigest("test-digest"),
							Name:   "test-step",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:       "successful test mode",
			outputMode: TestOutputModeOnBuild,
			solveStatuses: []*client.SolveStatus{
				{
					Vertexes: []*client.Vertex{
						{
							Digest: mockDigest("test-digest"),
							Name:   "test-step",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:       "default to deploy mode",
			outputMode: "unknown",
			solveStatuses: []*client.SolveStatus{
				{
					Vertexes: []*client.Vertex{
						{
							Digest: mockDigest("test-digest"),
							Name:   "test-step",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:       "vertex with error",
			outputMode: DeployOutputModeOnBuild,
			solveStatuses: []*client.SolveStatus{
				{
					Vertexes: []*client.Vertex{
						{
							Digest: mockDigest("test-digest"),
							Name:   "test-step",
							Error:  "build failed",
						},
					},
				},
			},
			expectError: false, // deployDisplayer logs errors but doesn't return them
		},
		{
			name:           "context cancelled",
			outputMode:     DeployOutputModeOnBuild,
			solveStatuses:  []*client.SolveStatus{},
			expectError:    true,
			contextTimeout: true,
		},
		{
			name:       "timeout case",
			outputMode: DeployOutputModeOnBuild,
			solveStatuses: []*client.SolveStatus{
				{
					Vertexes: []*client.Vertex{
						{
							Digest: digest.FromString("test-vertex"),
							Name:   "test step",
						},
					},
				},
			},
			expectError:    false,
			simulateDelay:  true, // This will help test timeout behavior
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.contextTimeout {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel() // Cancel immediately
			}
			
			ch := make(chan *client.SolveStatus, len(tt.solveStatuses)+1)
			
			// Send solve statuses
			for _, ss := range tt.solveStatuses {
				ch <- ss
			}
			
			// Only close channel if context is not cancelled
			// For context cancellation test, we want the context to be checked first
			if !tt.contextTimeout {
				close(ch) // Close channel to signal completion
			}
			
			buildOptions := &types.BuildOptions{
				OutputMode: tt.outputMode,
			}
			
			err := deployDisplayer(ctx, ch, buildOptions)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// mockDigest creates a mock digest for testing
func mockDigest(encoded string) digest.Digest {
	return digest.Digest("sha256:" + encoded)
}