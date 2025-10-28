// Copyright 2025 The Okteto Authors
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

package v2

import (
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/dag"
)

type buildGraph struct {
	nodes []dag.Node
}

type buildNode struct {
	id        string
	dependsOn []string
}

func (b *buildNode) ID() string {
	return b.id
}

func (b *buildNode) DependsOn() []string {
	return b.dependsOn
}

func (b *buildGraph) AddNode(node dag.Node) {
	b.nodes = append(b.nodes, node)
}

func (b *buildGraph) GetNodes() []dag.Node {
	return b.nodes
}

func (b *buildGraph) GetGraph() (*dag.Tree, error) {
	return dag.From(b.nodes...)
}

func newBuildGraph(buildManifest build.ManifestBuild, services []string) *buildGraph {
	graph := &buildGraph{}
	for _, svc := range services {
		graph.AddNode(&buildNode{id: svc, dependsOn: buildManifest[svc].DependsOn})
	}
	return graph
}
