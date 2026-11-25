// Copyright 2026 The Okteto Authors
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

package analytics

import (
	"regexp"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

var (
	uuidPattern      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	categoryPattern  = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
)

// propertyRule is the shape a property value must have to be sent, and the
// value sent instead when it doesn't.
type propertyRule struct {
	pattern  *regexp.Regexp
	fallback string
}

// safePropertyRules are the properties whose raw value could hold a namespace
// name, a repository URL with credentials, a user-defined name or an error
// message. Their call sites already send a UID, a hash or a category; these
// rules are a last check, so a new call site can't send the raw value.
var safePropertyRules = map[string]propertyRule{
	"namespace":   {pattern: uuidPattern},
	"preview":     {pattern: uuidPattern},
	"repoURL":     {pattern: sha256HexPattern},
	"name":        {pattern: sha256HexPattern},
	"service":     {pattern: sha256HexPattern},
	"errorReason": {pattern: categoryPattern, fallback: "unknown"},
	"error":       {pattern: categoryPattern, fallback: "unknown"},
}

// dropUnsafeProperties replaces every property in safePropertyRules whose value
// isn't an empty string or a string of the expected shape with the rule's
// fallback. It logs the property name, never its value.
func dropUnsafeProperties(event string, props map[string]any) {
	for key, rule := range safePropertyRules {
		value, ok := props[key]
		if !ok {
			continue
		}
		if s, isString := value.(string); isString && (s == "" || rule.pattern.MatchString(s)) {
			continue
		}
		oktetoLog.Infof("analytics: dropped unsafe %q property from the %q event", key, event)
		props[key] = rule.fallback
	}
}
