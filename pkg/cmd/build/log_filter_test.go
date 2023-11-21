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
	"reflect"
	"testing"
	"time"

	"github.com/moby/buildkit/client"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestNewBuildKitLogsFilterEmptyRules(t *testing.T) {
	lf := NewBuildKitLogsFilter([]Rule{})

	v := &client.Vertex{
		Name:  "test log message",
		Error: assert.AnError.Error(),
	}

	ss := &client.SolveStatus{
		Vertexes: []*client.Vertex{v},
	}

	lf.Run(ss, oktetoLog.TTYFormat)

	assert.Equal(t, "test log message", v.Name)
	assert.Equal(t, assert.AnError.Error(), v.Error)
	assert.Equal(t, 1, len(ss.Vertexes))
}

func TestNewBuildKitLogsFilter(t *testing.T) {
	now := time.Now()

	rules := []Rule{
		{
			condition:   BuildKitMissingCacheCondition,
			transformer: BuildKitMissingCacheTransformer,
		},
	}

	lf := NewBuildKitLogsFilter(rules)

	v := &client.Vertex{
		Name:      "importing cache manifest from test-registry.com/test-account/test-repo",
		Error:     "test-registry.com/test-account/test-repo: not found",
		Started:   &now,
		Completed: &now,
	}

	ss := &client.SolveStatus{
		Vertexes: []*client.Vertex{v},
	}

	lf.Run(ss, oktetoLog.TTYFormat)

	expected := &client.Vertex{
		Name:      "[skip] cache image not available: test-registry.com/test-account/test-repo",
		Error:     "",
		Started:   &now,
		Completed: &now,
	}

	if !reflect.DeepEqual(expected, v) {
		t.Errorf("expected %v, got %v", expected, v)
	}

	assert.Equal(t, 1, len(ss.Vertexes))
}

func TestBuildKitMissingCacheCondition(t *testing.T) {
	tests := []struct {
		v        *client.Vertex
		name     string
		expected bool
	}{
		{
			name: "no match found (error)",
			v: &client.Vertex{
				Name:  "importing cache manifest from test-registry.com/test-account/test-repo",
				Error: "",
			},
			expected: false,
		},
		{
			name: "no match found (name)",
			v: &client.Vertex{
				Name:  "something else",
				Error: "test-registry.com/test-account/test-repo: not found",
			},
			expected: false,
		},
		{
			name: "match found",
			v: &client.Vertex{
				Name:  "importing cache manifest from test-registry.com/test-account/test-repo",
				Error: "test-registry.com/test-account/test-repo: not found",
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := BuildKitMissingCacheCondition(tc.v)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBuildKitMissingCacheTransformerNotMatching(t *testing.T) {
	now := time.Now()

	v := &client.Vertex{
		Name:      "this log should not be transformed",
		Error:     assert.AnError.Error(),
		Started:   &now,
		Completed: &now,
	}

	expectedV := &client.Vertex{
		Name:      "this log should not be transformed",
		Error:     assert.AnError.Error(),
		Started:   &now,
		Completed: &now,
	}

	ss := &client.SolveStatus{
		Vertexes: []*client.Vertex{v},
		Statuses: nil,
		Logs:     nil,
	}

	expectedSs := &client.SolveStatus{
		Vertexes: []*client.Vertex{v},
		Statuses: nil,
		Logs:     nil,
	}

	BuildKitMissingCacheTransformer(v, ss, oktetoLog.TTYFormat)

	if !reflect.DeepEqual(expectedV, v) {
		t.Errorf("expected %v, got %v", v, v)
	}

	if !reflect.DeepEqual(expectedSs, ss) {
		t.Errorf("expected %v, got %v", ss, ss)
	}
}

func TestBuildKitMissingCacheTransformerTTY(t *testing.T) {
	now := time.Now()

	v := &client.Vertex{
		Name:      "importing cache manifest from test-registry.com/test-account/test-repo",
		Error:     "test-registry.com/test-account/test-repo: not found",
		Started:   &now,
		Completed: &now,
	}

	expected := &client.Vertex{
		Name:      "[skip] cache image not available: test-registry.com/test-account/test-repo",
		Error:     "",
		Started:   &now,
		Completed: &now,
	}

	ss := &client.SolveStatus{
		Vertexes: []*client.Vertex{v},
		Statuses: nil,
		Logs:     nil,
	}

	BuildKitMissingCacheTransformer(v, ss, oktetoLog.TTYFormat)

	if !reflect.DeepEqual(expected, v) {
		t.Errorf("expected %v, got %v", expected, v)
	}

	assert.Equal(t, 1, len(ss.Vertexes))
}

func TestBuildKitMissingCacheTransformerPlain(t *testing.T) {
	now := time.Now()

	v := &client.Vertex{
		Name:      "importing cache manifest from test-registry.com/test-account/test-repo",
		Error:     "test-registry.com/test-account/test-repo: not found",
		Started:   &now,
		Completed: &now,
	}

	expected := &client.Vertex{
		Name:      "importing cache manifest from test-registry.com/test-account/test-repo",
		Error:     "",
		Started:   &now,
		Completed: nil,
	}

	expectedNew := &client.Vertex{
		Name:      "[skip] cache image not available: test-registry.com/test-account/test-repo",
		Error:     "",
		Started:   &now,
		Completed: &now,
	}

	ss := &client.SolveStatus{
		Vertexes: []*client.Vertex{v},
		Statuses: nil,
		Logs:     nil,
	}

	BuildKitMissingCacheTransformer(v, ss, oktetoLog.PlainFormat)
	newVertex := ss.Vertexes[len(ss.Vertexes)-1]

	assert.Equal(t, 2, len(ss.Vertexes))

	if !reflect.DeepEqual(expected, v) {
		t.Errorf("expected %v, got %v", expected, v)
	}

	if !reflect.DeepEqual(expectedNew, newVertex) {
		t.Errorf("expected %v, got %v", expectedNew, newVertex)
	}
}
