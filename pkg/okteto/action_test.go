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

package okteto

import (
	"context"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestGetAction(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		action *types.Action
		err    error
	}
	testCases := []struct {
		expected expected
		cfg      input
		name     string
	}{
		{
			name: "error in graphql",
			cfg: input{
				client: &fakeGraphQLClient{
					err: assert.AnError,
				},
			},
			expected: expected{
				action: nil,
				err:    assert.AnError,
			},
		},
		{
			name: "graphql response is an action",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &getActionQueryStruct{
						Action: actionStruct{
							Id:     "id",
							Name:   "name",
							Status: "progressing",
						},
					},
				},
			},
			expected: expected{
				action: &types.Action{
					ID:     "id",
					Name:   "name",
					Status: "progressing",
				},
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pc := pipelineClient{
				client: tc.cfg.client,
			}
			action, err := pc.GetAction(context.Background(), "", "")
			assert.ErrorIs(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.action, action)
		})
	}
}

func TestWaitForActionToFinish(t *testing.T) {
	type input struct {
		client *fakeGraphQLMultipleCallsClient
	}
	type expected struct {
		err error
	}
	testCases := []struct {
		expected expected
		cfg      input
		name     string
	}{
		{
			name: "timeout is reached",
			cfg: input{
				client: &fakeGraphQLMultipleCallsClient{
					queryResults: []interface{}{},
					errs:         []error{},
				},
			},
			expected: expected{
				err: pipelineTimeoutError{
					pipelineName: "",
					timeout:      5 * time.Second,
				},
			},
		},
		{
			name: "error getting action",
			cfg: input{
				client: &fakeGraphQLMultipleCallsClient{
					queryResults: []interface{}{
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "progressing",
							},
						},
					},
					errs: []error{
						assert.AnError,
					},
				},
			},
			expected: expected{
				err: assert.AnError,
			},
		},
		{
			name: "progressing -> error",
			cfg: input{
				client: &fakeGraphQLMultipleCallsClient{
					queryResults: []interface{}{
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "progressing",
							},
						},
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "error",
							},
						},
					},
					errs: []error{},
				},
			},
			expected: expected{
				err: pipelineFailedError{
					pipelineName: "",
				},
			},
		},
		{
			name: "progressing -> successful",
			cfg: input{
				client: &fakeGraphQLMultipleCallsClient{
					queryResults: []interface{}{
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "progressing",
							},
						},
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "deployed",
							},
						},
					},
					errs: []error{},
				},
			},
			expected: expected{
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			numberOfCalls := len(tc.cfg.client.queryResults)
			timerChannel := make(chan time.Time)
			tickerChannel := make(chan time.Time)
			pc := pipelineClient{
				client: tc.cfg.client,
				provideTicker: func(_ time.Duration) *time.Ticker {
					return &time.Ticker{
						C: tickerChannel,
					}
				},
				provideTimer: func(d time.Duration) *time.Timer {
					return &time.Timer{
						C: timerChannel,
					}
				},
			}

			go func() {
				for i := 0; i < numberOfCalls; i++ {
					tickerChannel <- time.Now()
				}
				timerChannel <- time.Now()
			}()
			err := pc.WaitForActionToFinish(context.Background(), "", "", "", 5*time.Second)
			assert.ErrorIs(t, err, tc.expected.err)
		})
	}
}

func TestWaitForActionProgressing(t *testing.T) {
	type input struct {
		client *fakeGraphQLMultipleCallsClient
	}
	type expected struct {
		err error
	}
	testCases := []struct {
		expected expected
		cfg      input
		name     string
	}{
		{
			name: "timeout is reached",
			cfg: input{
				client: &fakeGraphQLMultipleCallsClient{
					queryResults: []interface{}{},
					errs:         []error{},
				},
			},
			expected: expected{
				err: pipelineTimeoutError{
					pipelineName: "",
					timeout:      5 * time.Second,
				},
			},
		},
		{
			name: "error getting action",
			cfg: input{
				client: &fakeGraphQLMultipleCallsClient{
					queryResults: []interface{}{
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "progressing",
							},
						},
					},
					errs: []error{
						assert.AnError,
					},
				},
			},
			expected: expected{
				err: assert.AnError,
			},
		},
		{
			name: "queued -> error",
			cfg: input{
				client: &fakeGraphQLMultipleCallsClient{
					queryResults: []interface{}{
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "queued",
							},
						},
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "error",
							},
						},
					},
					errs: []error{},
				},
			},
			expected: expected{
				err: pipelineFailedError{
					pipelineName: "",
				},
			},
		},
		{
			name: "progressing -> successful",
			cfg: input{
				client: &fakeGraphQLMultipleCallsClient{
					queryResults: []interface{}{
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "progressing",
							},
						},
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "deployed",
							},
						},
					},
					errs: []error{},
				},
			},
			expected: expected{
				err: nil,
			},
		},
		{
			name: "queued -> progressing -> successful",
			cfg: input{
				client: &fakeGraphQLMultipleCallsClient{
					queryResults: []interface{}{
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "queued",
							},
						},
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "progressing",
							},
						},
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "deployed",
							},
						},
					},
					errs: []error{},
				},
			},
			expected: expected{
				err: nil,
			},
		},
		{
			name: "successful",
			cfg: input{
				client: &fakeGraphQLMultipleCallsClient{
					queryResults: []interface{}{
						&getActionQueryStruct{
							Action: actionStruct{
								Id:     "id",
								Name:   "name",
								Status: "deployed",
							},
						},
					},
					errs: []error{},
				},
			},
			expected: expected{
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			numberOfCalls := len(tc.cfg.client.queryResults)
			timerChannel := make(chan time.Time)
			tickerChannel := make(chan time.Time)
			pc := pipelineClient{
				client: tc.cfg.client,
				provideTicker: func(_ time.Duration) *time.Ticker {
					return &time.Ticker{
						C: tickerChannel,
					}
				},
				provideTimer: func(d time.Duration) *time.Timer {
					return &time.Timer{
						C: timerChannel,
					}
				},
			}

			go func() {
				for i := 0; i < numberOfCalls; i++ {
					tickerChannel <- time.Now()
				}
				timerChannel <- time.Now()
			}()
			err := pc.WaitForActionProgressing(context.Background(), "", "", "", 5*time.Second)
			assert.ErrorIs(t, err, tc.expected.err)
		})
	}
}
