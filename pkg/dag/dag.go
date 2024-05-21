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
	"fmt"

	"github.com/heimdalr/dag"
)

type Node interface {
	ID() string
	DependsOn() []string
}

type callback func(Node)

func (cb callback) Visit(vx dag.Vertexer) {
	_, value := vx.Vertex()
	cb(value.(Node))
}

type Tree struct {
	graph *dag.DAG
}

func (tree *Tree) Subtree(nodeIds ...string) (*Tree, error) {
	if len(nodeIds) < 1 {
		return tree, nil
	}

	selection := make(map[string]Node)

	for _, nodeId := range nodeIds {
		nI, err := tree.graph.GetVertex(nodeId)
		if err != nil {
			return nil, err
		}
		// Add the node to the subnode list
		n, ok := nI.(Node)
		if !ok {
			return nil, fmt.Errorf("fail to cast tree vertex to Node type")
		}
		selection[nodeId] = n

		// resolve depends_on by getting nodes' ancestors
		ancestor, err := tree.graph.GetAncestors(nodeId)
		if err != nil {
			return nil, err
		}
		for childID, nI := range ancestor {
			n, ok := nI.(Node)
			if !ok {
				return nil, fmt.Errorf("fail to cast tree vertex to Node type")
			}
			selection[childID] = n
		}
	}

	var subnodes []Node
	for _, n := range selection {
		subnodes = append(subnodes, n)
	}

	return From(subnodes...)
}

func (tree *Tree) Traverse(fn func(n Node)) {
	tree.graph.OrderedWalk(callback(fn))
}

func From(nodes ...Node) (*Tree, error) {
	tree := &Tree{graph: dag.NewDAG()}

	for _, n := range nodes {
		if _, err := tree.graph.AddVertex(n); err != nil {
			return nil, err
		}
	}

	for _, n := range nodes {
		for _, dep := range n.DependsOn() {
			if err := tree.graph.AddEdge(dep, n.ID()); err != nil {
				return nil, err
			}
		}
	}
	return tree, nil
}

// Ordered returns a list of node ids ordered by dependsOn starting by the root
// (nodes with no dependencies) and traversing the whole tree
func (tree *Tree) Ordered() (s []string) {
	tree.Traverse(func(n Node) {
		s = append(s, n.ID())
	})
	return
}
