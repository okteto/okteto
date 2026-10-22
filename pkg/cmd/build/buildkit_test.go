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

package build

import (
	"bytes"
	"testing"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// initBuildLogger resets the logger with the given output format, while an image is being built
func initBuildLogger(t *testing.T, format string) {
	t.Helper()
	oktetoLog.Init(logrus.WarnLevel)
	oktetoLog.SetOutputFormat(format)
	oktetoLog.SetStage("Building service api")
	t.Cleanup(func() {
		oktetoLog.SetStage("")
		oktetoLog.Init(logrus.WarnLevel)
	})
}

func TestPlainProgressWriterStoresForcedPlainProgress(t *testing.T) {
	// plain progress while the output is a terminal: it was forced because dependencies log at the same time
	initBuildLogger(t, oktetoLog.TTYFormat)
	out := &bytes.Buffer{}

	_, err := plainProgressWriter(oktetoLog.PlainFormat, out, &buildWriter{}).Write([]byte("#1 building api\n"))

	require.NoError(t, err)
	require.Contains(t, out.String(), "#1 building api")
	require.Contains(t, oktetoLog.GetOutputBuffer().String(), "#1 building api")
}

func TestPlainProgressWriterDoesNotStorePlainOutput(t *testing.T) {
	initBuildLogger(t, oktetoLog.PlainFormat)
	out := &bytes.Buffer{}

	_, err := plainProgressWriter(oktetoLog.PlainFormat, out, &buildWriter{}).Write([]byte("#1 building api\n"))

	require.NoError(t, err)
	require.Contains(t, out.String(), "#1 building api")
	require.NotContains(t, oktetoLog.GetOutputBuffer().String(), "#1 building api")
}
