package build

import (
	"fmt"
	"github.com/moby/buildkit/client"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"regexp"
)

type ConditionFunc func(vertex *client.Vertex) bool
type TransformerFunc func(vertex *client.Vertex, ss *client.SolveStatus, progress string)

type Rule struct {
	condition   ConditionFunc
	transformer TransformerFunc
}

type BuildKitLogsFilter struct {
	rules []Rule
}

func NewBuildKitLogsFilter(rules []Rule) *BuildKitLogsFilter {
	return &BuildKitLogsFilter{
		rules: rules,
	}
}

func (lf *BuildKitLogsFilter) Run(ss *client.SolveStatus, progress string) {
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

	if len(matches) < 2 {
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
}
