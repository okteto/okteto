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

package analytics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_hashString(t *testing.T) {
	input := "test-string"
	require.Equal(t, hashString(input), "ffe65f1d98fafedea3514adc956c8ada5980c6c5d2552fd61f48401aefd5c00e")
}
