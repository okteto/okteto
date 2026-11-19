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

package linguist

import (
	"sort"
	"strings"
)

const (
	Javascript = "javascript"
	golang     = "go"
	Python     = "python"
	Gradle     = "gradle"
	Maven      = "maven"
	Java       = "java"
	Ruby       = "ruby"
	Csharp     = "csharp"
	Php        = "php"
	Rust       = "rust"

	// Unrecognized is the option returned when the linguist couldn't detect a language
	Unrecognized = "other"
)

// supportedLanguages are the languages that can be selected when the linguist can't detect one
var supportedLanguages = []string{Javascript, golang, Rust, Python, Gradle, Maven, Ruby, Csharp, Php}

// GetSupportedLanguages returns a list of supported languages
func GetSupportedLanguages() []string {
	l := make([]string, len(supportedLanguages), len(supportedLanguages)+1)
	copy(l, supportedLanguages)
	sort.Strings(l)
	l = append(l, Unrecognized)

	return l
}

func NormalizeLanguage(language string) string {
	lower := strings.ToLower(language)
	switch lower {
	case "typescript", "javascript", "jsx", "node", "tsx":
		return Javascript
	case "python":
		return Python
	case "java":
		return Gradle
	case "gradle":
		return Gradle
	case "maven":
		return Maven
	case "ruby":
		return Ruby
	case "go", "golang":
		return golang
	case "c#":
		return Csharp
	case "csharp":
		return Csharp
	case "php":
		return Php
	case "rust":
		return Rust
	default:
		return Unrecognized
	}
}
