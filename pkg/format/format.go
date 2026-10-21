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

package format

import (
	"regexp"
	"strings"
)

var (
	// validKubeNameRegex is the regex to validate a kubernetes resource name
	validKubeNameRegex = regexp.MustCompile(`[^a-z0-9\-]+`)
)

const (
	// maxK8sResourceMetaLength is the max length a string can have to be considered a kubernetes resource name, label, annotation, etc
	maxK8sResourceMetaLength = 63
)

// ResourceK8sMetaString transforms the name param intro a compatible k8s string to be used as name or meta information in any resource
func ResourceK8sMetaString(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)

	name = validKubeNameRegex.ReplaceAllString(name, "-")

	// trim the repository name for internal use in labels
	if len(name) > maxK8sResourceMetaLength {
		name = name[:maxK8sResourceMetaLength]
	}
	name = strings.TrimSuffix(name, "-")
	name = strings.TrimPrefix(name, "-")
	return name
}
