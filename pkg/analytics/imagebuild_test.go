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
	"time"

	"github.com/stretchr/testify/require"
)

func Test_ImageBuildMetadata_toProps(t *testing.T) {
	m := &ImageBuildMetadata{
		Name:                     "test-service",
		RepoURL:                  "http://myrepo.url",
		CacheHit:                 true,
		CacheHitDuration:         5 * time.Second,
		BuildDuration:            5 * time.Second,
		WaitForBuildkitAvailable: 12 * time.Second,
		BuildkitDuration:         25 * time.Second,
		ContextTransferDuration:  3 * time.Second,
		BuildContextSize:         20_000_000,
		BuildContextHash:         "contextHash",
		BuildContextHashDuration: 5 * time.Second,
		ConnectionType:           "proxy",
		Initiator:                "me",
		Success:                  true,
	}

	expectedProps := map[string]any{
		"name":                            "665653223b1e8bfa2d462b3adb06d49f8984052e5df03d7fd2365293a102fce8",
		"repoURL":                         "e405794625b21c12a1254f8f1c464e8f46c26f5a5a3b8843231f81d86ee1c3c1",
		"cacheHit":                        true,
		"cacheHitDurationSeconds":         float64(5),
		"waitForBuildkitAvailable":        float64(12),
		"buildDurationSeconds":            float64(5),
		"cloneDurationSeconds":            float64(0),
		"buildContextHash":                "contextHash",
		"buildContextHashDurationSeconds": float64(5),
		"initiator":                       "me",
		"buildkitDurationSeconds":         25,
		"buildContextDurationMs":          int64(3000),
		"buildContextSizeBytes":           int64(20_000_000),
		"connectionType":                  "proxy",
	}

	require.Equal(t, expectedProps, m.toProps())
}

func Test_NewImageBuildMetadata(t *testing.T) {
	require.Empty(t, NewImageBuildMetadata())
	require.IsType(t, &ImageBuildMetadata{}, NewImageBuildMetadata())
}

func Test_ImageBuildMetadata_toProps_omitsZeroPortedFields(t *testing.T) {
	props := (&ImageBuildMetadata{Name: "api", Success: true}).toProps()

	require.NotContains(t, props, "buildkitDurationSeconds")
	require.NotContains(t, props, "buildContextDurationMs")
	require.NotContains(t, props, "buildContextSizeBytes")
	require.NotContains(t, props, "connectionType")
	require.NotContains(t, props, "errorReason")
}

func Test_ImageBuildMetadata_toProps_withError(t *testing.T) {
	props := (&ImageBuildMetadata{Success: false, ErrorReason: "image_not_found"}).toProps()

	require.NotContains(t, props, "result", "imageBuild already has success")
	require.Equal(t, "image_not_found", props["errorReason"])
}

func Test_ImageBuildMetadata_toProps_omitsErrorReasonOnSuccess(t *testing.T) {
	props := (&ImageBuildMetadata{Success: true, ErrorReason: "image_not_found"}).toProps()

	require.NotContains(t, props, "errorReason")
}
