package dag

import (
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
