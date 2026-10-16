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

package okteto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type lineRecorder struct {
	lines []string
}

func (r *lineRecorder) println(line string) {
	r.lines = append(r.lines, line)
}

func TestPipelineLogLineHandlerSingleMessage(t *testing.T) {
	recorder := &lineRecorder{}
	handler := pipelineLogLineHandler(recorder.println)

	stop := handler(`{"message":"deploying database","stage":"deploy"}`)

	require.False(t, stop)
	require.Equal(t, []string{"deploying database"}, recorder.lines)
}

func TestPipelineLogLineHandlerListOfMessages(t *testing.T) {
	recorder := &lineRecorder{}
	handler := pipelineLogLineHandler(recorder.println)

	stop := handler(`[{"message":"line 1","stage":"deploy"},{"message":"line 2","stage":"deploy"}]`)

	require.False(t, stop)
	require.Equal(t, []string{"line 1", "line 2"}, recorder.lines)
}

func TestPipelineLogLineHandlerStopsOnEOF(t *testing.T) {
	recorder := &lineRecorder{}
	handler := pipelineLogLineHandler(recorder.println)

	stop := handler(`{"message":"EOF","stage":"done"}`)

	require.True(t, stop)
	require.Empty(t, recorder.lines)
}

func TestPipelineLogLineHandlerStopsOnEOFInsideList(t *testing.T) {
	recorder := &lineRecorder{}
	handler := pipelineLogLineHandler(recorder.println)

	stop := handler(`[{"message":"line 1","stage":"deploy"},{"message":"EOF","stage":"done"},{"message":"ignored","stage":"deploy"}]`)

	require.True(t, stop)
	require.Equal(t, []string{"line 1"}, recorder.lines)
}

func TestPipelineLogLineHandlerInvalidJSONDoesNotStop(t *testing.T) {
	recorder := &lineRecorder{}
	handler := pipelineLogLineHandler(recorder.println)

	stop := handler(`not json`)

	require.False(t, stop)
}
