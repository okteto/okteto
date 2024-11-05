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
	"errors"
	"fmt"
	"regexp"

	"github.com/moby/buildkit/client"
	okErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type ConditionFunc func(vertex *client.Vertex) bool
type TransformerFunc func(vertex *client.Vertex, ss *client.SolveStatus, progress string)

type LogRule struct {
	condition   ConditionFunc
	transformer TransformerFunc
}

type ErrorRule struct {
	condition ConditionFunc
}

type BuildkitLogsFilter struct {
	transformRules []LogRule
	errorRules     []ErrorRule
}

func NewBuildKitLogsFilter(transformRules []LogRule, errorRules []ErrorRule) *BuildkitLogsFilter {
	return &BuildkitLogsFilter{
		transformRules: transformRules,
		errorRules:     errorRules,
	}
}

func (lf *BuildkitLogsFilter) Run(ss *client.SolveStatus, progress string) {
	for _, vertex := range ss.Vertexes {
		for _, rule := range lf.transformRules {
			if rule.condition(vertex) {
				rule.transformer(vertex, ss, progress)
			}
		}
	}
}

func (lf *BuildkitLogsFilter) GetError(ss *client.SolveStatus) error {
	for _, vertex := range ss.Vertexes {
		for _, rule := range lf.errorRules {
			if rule.condition(vertex) {
				return okErrors.UserError{
					E:    errors.New("docker frontend image not found"),
					Hint: "Check your OKTETO_BUILDKIT_FRONTEND_IMAGE environment variable",
				}
			}
		}
	}
	return nil
}

// BuildKitFrontendNotFoundErr checks if the log contains a frontend not found error
var BuildKitFrontendNotFoundErr ConditionFunc = func(vertex *client.Vertex) bool {
	if vertex.Error == "" {
		return false
	}
	importPattern := `resolve image config for docker-image://.*`

	if !regexp.MustCompile(importPattern).MatchString(vertex.Name) {
		return false
	}

	errorPattern := `.*: not found`

	return regexp.MustCompile(errorPattern).MatchString(vertex.Error)
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
