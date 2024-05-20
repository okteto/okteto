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

package dag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testNode struct {
	id        string
	dependsOn []string
}

func (n *testNode) ID() string          { return n.id }
func (n *testNode) DependsOn() []string { return n.dependsOn }

func TestTraverseError(t *testing.T) {
	nodes := []Node{
		&testNode{id: "v1", dependsOn: []string{"v2"}},
		&testNode{id: "v2", dependsOn: []string{"v3"}},
		&testNode{id: "v3", dependsOn: []string{"v1"}},
	}
	_, err := From(nodes...)
	require.EqualError(t, err, "edge between 'v1' and 'v3' would create a loop")

	nodes = []Node{
		&testNode{id: "v1", dependsOn: []string{"v2"}},
		&testNode{id: "v2"},
		&testNode{id: "v3"},
		&testNode{id: "v1", dependsOn: []string{"v3"}},
	}
	_, err = From(nodes...)
	require.EqualError(t, err, "the id 'v1' is already known")

	nodes = []Node{
		&testNode{id: "v1", dependsOn: []string{"v2"}},
		&testNode{id: "v2"},
		&testNode{id: "v3", dependsOn: []string{"v3"}},
	}
	_, err = From(nodes...)
	require.EqualError(t, err, "src ('v3') and dst ('v3') equal")

}
func TestTraverse(t *testing.T) {

	var tt = []struct {
		name     string
		nodes    []Node
		expected []string
	}{
		{
			//	v1
			//	^
			//	|
			//	v2
			//	^
			//	|
			//	v3
			//	^
			//	|
			//	v4
			//	^
			//	|
			//	v5
			name: "linear",
			nodes: []Node{
				&testNode{id: "v1"},
				&testNode{id: "v2", dependsOn: []string{"v1"}},
				&testNode{id: "v3", dependsOn: []string{"v2"}},
				&testNode{id: "v4", dependsOn: []string{"v3"}},
				&testNode{id: "v5", dependsOn: []string{"v4"}},
			},
			expected: []string{"v1", "v2", "v3", "v4", "v5"},
		},
		{
			//	v5 --> v4
			//
			//
			//	v2 --> v1
			//	       ^
			//	       |
			//	      v3
			name: "sparse-two-roots",
			nodes: []Node{
				&testNode{id: "v1"},
				&testNode{id: "v2", dependsOn: []string{"v1"}},
				&testNode{id: "v3", dependsOn: []string{"v1"}},
				&testNode{id: "v4"},
				&testNode{id: "v5", dependsOn: []string{"v4"}},
			},
			expected: []string{"v1", "v4", "v2", "v3", "v5"},
		},
		{
			//	v1     v2
			//	^      ^
			//	|      |
			//	v4 --> v3
			//	^
			//	|
			//	v5
			name: "fork",
			nodes: []Node{
				&testNode{id: "v1"},
				&testNode{id: "v2"},
				&testNode{id: "v3", dependsOn: []string{"v2"}},
				&testNode{id: "v4", dependsOn: []string{"v1"}},
				&testNode{id: "v5", dependsOn: []string{"v4"}},
			},
			expected: []string{"v1", "v2", "v4", "v3", "v5"},
		},
		{
			//	v1     v3 <---┐
			//	       ^      |
			//	       |      |
			//  v2 <-- v5 --> v4
			name: "edgy",
			nodes: []Node{
				&testNode{id: "v1"},
				&testNode{id: "v2"},
				&testNode{id: "v3"},
				&testNode{id: "v4", dependsOn: []string{"v3"}},
				&testNode{id: "v5", dependsOn: []string{"v4", "v3", "v2"}},
			},
			expected: []string{"v1", "v2", "v3", "v4", "v5"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := From(tc.nodes...)
			require.NoError(t, err)

			var result []string
			tree.Traverse(func(n Node) {
				result = append(result, n.ID())
			})
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSubtree(t *testing.T) {
	v99 := &testNode{id: "v99"}
	v4 := &testNode{id: "v4", dependsOn: []string{"v3"}}
	v3 := &testNode{id: "v3", dependsOn: []string{"v2"}}
	v2 := &testNode{id: "v2", dependsOn: []string{"v1"}}
	v1 := &testNode{id: "v1", dependsOn: []string{"v0"}}
	v0 := &testNode{id: "v0"}

	tree, err := From(v99, v4, v3, v2, v1, v0)
	require.NoError(t, err)

	tt := []struct {
		name     string
		subnodes []string
		expected []string
	}{
		{
			name:     "orphans",
			subnodes: []string{"v99", "v0"},
			expected: []string{"v99", "v0"},
		},
		{
			name:     "nested1",
			subnodes: []string{"v99", "v1"},
			expected: []string{"v99", "v1", "v0"},
		},
		{
			name:     "nested2",
			subnodes: []string{"v99", "v2"},
			expected: []string{"v99", "v2", "v1", "v0"},
		},
		{
			name:     "nested3",
			subnodes: []string{"v99", "v3"},
			expected: []string{"v99", "v3", "v2", "v1", "v0"},
		},
		{
			name:     "nested4",
			subnodes: []string{"v99", "v4"},
			expected: []string{"v99", "v4", "v3", "v2", "v1", "v0"},
		},
		{
			name:     "all_no_orphan",
			subnodes: []string{"v4"},
			expected: []string{"v4", "v3", "v2", "v1", "v0"},
		},
		{
			name:     "half",
			subnodes: []string{"v2"},
			expected: []string{"v2", "v1", "v0"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			subtree, err := tree.Subtree(tc.subnodes...)
			require.NoError(t, err)
			result := subtree.Ordered()
			require.Len(t, result, len(tc.expected))
			require.ElementsMatch(t, result, tc.expected)
		})
	}
}
