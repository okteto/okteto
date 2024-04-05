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

// package dag provides generic interface for working with dependencies resolution
// via Directed Acyclic Graphs (DAGs)
//
// It is required for clients of this package to implement the Node interface
// which consists of two methods: ID() and DependsOn():
//   - ID: It's the ID of the node and how it will be identified in the tree
//   - DependsOn: It's the dependencies of this node and should come BEFORE this node
//
// IDs should be unique and there should not be any cycles. An example of a cycle
// is: A->B, B->C, C->A
//
// Usage:
// ```go
//
//	  // testNode is an custom implementation for resolving dependencies
//		 type testNode struct {
//			  id        string
//			  dependsOn []string
//		 }
//
//		 func (n *testNode) ID() string          { return n.id }
//		 func (n *testNode) DependsOn() []string { return n.dependsOn }
//
//		 nodes := []dag.Node{
//		   &testNode{id: "v1"},
//		   &testNode{id: "v2", dependsOn: []string{"v1"}},
//		   &testNode{id: "v3", dependsOn: []string{"v2"}},
//		   &testNode{id: "v4", dependsOn: []string{"v3"}},
//		   &testNode{id: "v5", dependsOn: []string{"v4"}},
//		 }
//	 result := []string{}
//	 tree, _ := dag.From(nodes...)
//	 tree.Traverse(func(n dag.Node) {
//	   result = append(result, n.ID())
//	 })
//	 fmt.Println(strings.Join(result, ",")) // v1,v2,v3,v4,v5
//
// ```
//
// Traverse() takes care of walking the DAG and calling the callback in order
// based on the DependsOn()
package dag
