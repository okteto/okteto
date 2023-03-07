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

package registry

import (
	"fmt"
	"regexp"
	"strings"
)

type Replacer struct {
	registryURL string
}

func NewRegistryReplacer(registryURL string) Replacer {
	return Replacer{
		registryURL: registryURL,
	}
}

func (r Replacer) Replace(image, registryType, namespace string) string {
	// Check if the registryType is the start of the sentence or has a whitespace before it
	var re = regexp.MustCompile(fmt.Sprintf(`(^|\s)(%s)`, registryType))
	if re.MatchString(image) {
		return strings.Replace(image, registryType, fmt.Sprintf("%s/%s", r.registryURL, namespace), 1)
	}
	return image
}
