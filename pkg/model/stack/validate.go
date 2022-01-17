// Copyright 2021 The Okteto Authors
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

package stack

func getDependentCyclic(s *Stack) []string {
	visited := make(map[string]bool)
	stack := make(map[string]bool)
	cycle := make([]string, 0)
	for svcName := range s.Services {
		if dfs(s, svcName, visited, stack) {
			for svc, isInStack := range stack {
				if isInStack {
					cycle = append(cycle, svc)
				}
			}
			return cycle
		}
	}
	return cycle
}

func dfs(s *Stack, svcName string, visited, stack map[string]bool) bool {
	isVisited := visited[svcName]
	if !isVisited {
		visited[svcName] = true
		stack[svcName] = true

		svc := s.Services[svcName]
		for dependentSvc := range svc.DependsOn {
			if !visited[dependentSvc] && dfs(s, dependentSvc, visited, stack) {
				return true
			} else if value, ok := stack[dependentSvc]; ok && value {
				return true
			}
		}
	}
	stack[svcName] = false
	return false
}
