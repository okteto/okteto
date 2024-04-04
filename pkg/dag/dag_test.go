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

			result := []string{}
			tree.Traverse(func(n Node) {
				result = append(result, n.ID())
			})
			assert.Equal(t, tc.expected, result)
		})
	}
}
