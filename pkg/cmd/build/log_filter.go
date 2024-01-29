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
	"fmt"
	"regexp"

	"github.com/moby/buildkit/client"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type ConditionFunc func(vertex *client.Vertex) bool
type TransformerFunc func(vertex *client.Vertex, ss *client.SolveStatus, progress string)

type Rule struct {
	condition   ConditionFunc
	transformer TransformerFunc
}

type BuildkitLogsFilter struct {
	rules []Rule
}

func NewBuildKitLogsFilter(rules []Rule) *BuildkitLogsFilter {
	return &BuildkitLogsFilter{
		rules: rules,
	}
}

func (lf *BuildkitLogsFilter) Run(ss *client.SolveStatus, progress string) {
	for _, vertex := range ss.Vertexes {
		for _, rule := range lf.rules {
			if rule.condition(vertex) {
				rule.transformer(vertex, ss, progress)
			}
		}
	}
}

// BuildKitMissingCacheCondition checks if the log contains a missing cache error
var BuildKitMissingCacheCondition ConditionFunc = func(vertex *client.Vertex) bool {
	importPattern := `importing cache manifest from .*`

	if !regexp.MustCompile(importPattern).MatchString(vertex.Name) {
		return false
	}

	errorPattern := `.*: not found`

	return regexp.MustCompile(errorPattern).MatchString(vertex.Error)
}

// BuildKitMissingCacheTransformer transforms the missing cache error log to a more user-friendly message
var BuildKitMissingCacheTransformer TransformerFunc = func(vertex *client.Vertex, ss *client.SolveStatus, progress string) {
	errorPattern := `(.*): not found`
	re := regexp.MustCompile(errorPattern)
	matches := re.FindStringSubmatch(vertex.Error)

	minMatches := 2
	if len(matches) < minMatches {
		return
	}

	msg := fmt.Sprintf("[skip] cache image not available: %s", matches[1])

	vertex.Error = ""

	if progress == oktetoLog.TTYFormat {
		vertex.Name = msg
	} else {
		vertex.Completed = nil
		started := vertex.Started
		completed := started
		newVertex := &client.Vertex{
			Name:      msg,
			Started:   started,
			Completed: completed,
		}

		ss.Vertexes = append(ss.Vertexes, newVertex)
	}

	oktetoLog.Info(msg)
}
