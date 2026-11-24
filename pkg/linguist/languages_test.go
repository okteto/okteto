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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSupportedLanguages(t *testing.T) {
	expected := []string{Csharp, golang, Gradle, Javascript, Maven, Php, Python, Ruby, Rust, Unrecognized}

	require.Equal(t, expected, GetSupportedLanguages())
}

func TestNormalizeLanguage(t *testing.T) {
	tests := []struct {
		language string
		expected string
	}{
		{language: "typescript", expected: Javascript},
		{language: "javascript", expected: Javascript},
		{language: "jsx", expected: Javascript},
		{language: "node", expected: Javascript},
		{language: "tsx", expected: Javascript},
		{language: "python", expected: Python},
		{language: "java", expected: Gradle},
		{language: "gradle", expected: Gradle},
		{language: "maven", expected: Maven},
		{language: "ruby", expected: Ruby},
		{language: "go", expected: golang},
		{language: "golang", expected: golang},
		{language: "c#", expected: Csharp},
		{language: "csharp", expected: Csharp},
		{language: "php", expected: Php},
		{language: "rust", expected: Rust},
		{language: "TypeScript", expected: Javascript},
		{language: "GO", expected: golang},
		{language: "cobol", expected: Unrecognized},
		{language: "", expected: Unrecognized},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			require.Equal(t, tt.expected, NormalizeLanguage(tt.language))
		})
	}
}
