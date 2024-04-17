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

package deployable

import (
	"fmt"
	"sort"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestEnvStepper(t *testing.T) {
	fs := afero.NewMemMapFs()
	f, err := afero.TempFile(fs, "", "")
	require.NoError(t, err)

	defer f.Close()

	stepper := NewEnvStepper(f.Name())
	stepper.WithFS(fs)

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("ENV%02v", i)
		value := fmt.Sprintf("%v", i)
		keyVal := fmt.Sprintf("%0s=%s", key, value)

		f.WriteString(keyVal + "\n")

		envvar, err := stepper.Step()
		require.NoError(t, err)

		// sort first. Order is not guaranteeed
		sort.Strings(envvar)
		require.Equal(t, keyVal, envvar[i])
	}

	result := stepper.Map()

	for i := 0; i < 10; i++ {
		val, ok := result[fmt.Sprintf("ENV%02v", i)]
		require.True(t, ok)
		require.Equal(t, fmt.Sprintf("%v", i), val)
	}

}
