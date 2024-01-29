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

package io

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestK8sLoggerInitialisation(t *testing.T) {
	k := NewK8sLogger()
	require.NotNil(t, k)
	require.Equal(t, os.Stdout, k.out.out)
	require.Equal(t, os.Stdin, k.in.in)
}

func Test_IsEnabled(t *testing.T) {
	k := NewK8sLogger()
	require.False(t, k.IsEnabled())

	t.Setenv(OktetoK8sLoggerEnabledEnvVar, "true")
	require.True(t, k.IsEnabled())
}

func Test_GetK8sLoggerFilePath(t *testing.T) {
	okHome := filepath.Clean("test-okteto-home")
	k8sLogsFilepath := GetK8sLoggerFilePath(okHome)
	assert.Equal(t, filepath.Join(okHome, K8sLogsFileName), k8sLogsFilepath)
}
