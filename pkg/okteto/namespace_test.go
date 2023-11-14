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

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestCreateNamespace(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		err error
		id  string
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
				id:  "",
				err: assert.AnError,
			},
		},
		{
			name: "graphql response is an action",
			cfg: input{
				client: &fakeGraphQLClient{
					mutationResult: &createNamespaceMutation{
						Response: namespaceID{
							Id: "test",
						},
					},
				},
			},
			expected: expected{
				id:  "test",
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nc := &namespaceClient{
				client: tc.cfg.client,
			}
			id, err := nc.Create(context.Background(), "")
			assert.ErrorIs(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.id, id)
		})
	}
}

func TestDeleteNamespace(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		err error
		id  string
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
				id:  "",
				err: assert.AnError,
			},
		},
		{
			name: "graphql response is an action",
			cfg: input{
				client: &fakeGraphQLClient{
					mutationResult: &deleteNamespaceMutation{
						Response: namespaceID{
							Id: "test",
						},
					},
				},
			},
			expected: expected{
				id:  "test",
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nc := &namespaceClient{
				client: tc.cfg.client,
			}
			err := nc.Delete(context.Background(), "")
			assert.ErrorIs(t, err, tc.expected.err)
		})
	}
}

func TestAddMembers(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		err error
		id  string
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
				id:  "",
				err: assert.AnError,
			},
		},
		{
			name: "graphql response is an action",
			cfg: input{
				client: &fakeGraphQLClient{
					mutationResult: &addMembersMutation{
						Response: namespaceID{
							Id: "test",
						},
					},
				},
			},
			expected: expected{
				id:  "test",
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nc := &namespaceClient{
				client: tc.cfg.client,
			}
			err := nc.AddMembers(context.Background(), "", []string{"test"})
			assert.ErrorIs(t, err, tc.expected.err)
		})
	}
}

func TestListNamespaces(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		err    error
		result []types.Namespace
	}
	testCases := []struct {
		cfg      input
		name     string
		expected expected
	}{
		{
			name: "error in graphql",
			cfg: input{
				client: &fakeGraphQLClient{
					err: assert.AnError,
				},
			},
			expected: expected{
				result: nil,
				err:    assert.AnError,
			},
		},
		{
			name: "graphql response is an action",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &listNamespacesQuery{
						Response: []namespaceStatus{
							{
								Id:     "1",
								Status: ProgressingStatus,
							},
							{
								Id:     "2",
								Status: "error",
							},
						},
					},
				},
			},
			expected: expected{
				result: []types.Namespace{
					{
						ID:     "1",
						Status: ProgressingStatus,
					},
					{
						ID:     "2",
						Status: "error",
					},
				},
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nc := &namespaceClient{
				client: tc.cfg.client,
			}
			spaces, err := nc.List(context.Background())
			assert.ErrorIs(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.result, spaces)
		})
	}
}

func TestWakeNamespace(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		err error
		id  string
	}
	testCases := []struct {
		cfg      input
		expected expected
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
				id:  "",
				err: assert.AnError,
			},
		},
		{
			name: "graphql response is an action",
			cfg: input{
				client: &fakeGraphQLClient{
					mutationResult: &wakeNamespaceMutation{
						Response: namespaceID{
							Id: "test",
						},
					},
				},
			},
			expected: expected{
				id:  "test",
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nc := &namespaceClient{
				client: tc.cfg.client,
			}
			err := nc.Wake(context.Background(), "")
			assert.ErrorIs(t, err, tc.expected.err)
		})
	}
}

func TestDestroyAllNamespace(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		err error
		id  string
	}
	testCases := []struct {
		cfg      input
		expected expected
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
				id:  "",
				err: assert.AnError,
			},
		},
		{
			name: "graphql response is an action",
			cfg: input{
				client: &fakeGraphQLClient{
					mutationResult: &namespaceDestroyAllMutation{
						Response: namespaceID{
							Id: "test",
						},
					},
				},
			},
			expected: expected{
				id:  "test",
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nc := &namespaceClient{
				client: tc.cfg.client,
			}
			err := nc.DestroyAll(context.Background(), "", false)
			assert.ErrorIs(t, err, tc.expected.err)
		})
	}
}

func TestSleepNamespace(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		err error
		id  string
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
				id:  "",
				err: assert.AnError,
			},
		},
		{
			name: "graphql response is an action",
			cfg: input{
				client: &fakeGraphQLClient{
					mutationResult: &sleepNamespaceMutation{
						Response: namespaceID{
							Id: "test",
						},
					},
				},
			},
			expected: expected{
				id:  "test",
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nc := &namespaceClient{
				client: tc.cfg.client,
			}
			err := nc.Sleep(context.Background(), "")
			assert.ErrorIs(t, err, tc.expected.err)
		})
	}
}
