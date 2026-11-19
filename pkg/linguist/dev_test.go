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

package linguist

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSupportedLanguages(t *testing.T) {
	expected := []string{"csharp", "go", "gradle", "javascript", "maven", "php", "python", "ruby", "rust", Unrecognized}

	require.Equal(t, expected, GetSupportedLanguages())
}

func TestGetSupportedLanguagesDoesNotMutateTheList(t *testing.T) {
	first := GetSupportedLanguages()
	first[0] = "modified"

	require.Equal(t, "csharp", GetSupportedLanguages()[0])
	require.Equal(t, Javascript, supportedLanguages[0])
}
