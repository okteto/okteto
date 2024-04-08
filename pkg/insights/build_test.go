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

package insights

import (
	"context"
	"testing"
	"time"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestConvertImageBuildMetadataToEvent(t *testing.T) {
	metadata := &analytics.ImageBuildMetadata{
		DevenvName:    "test-devenv",
		Name:          "test-image",
		Namespace:     "test-namespace",
		RepoURL:       "test-repo",
		CacheHit:      true,
		Success:       true,
		BuildDuration: time.Duration(5) * time.Second,
	}

	publisher := &Publisher{}
	expectedEvent := buildEventJSON{
		DevenvName:    "test-devenv",
		ImageName:     "test-image",
		Namespace:     "test-namespace",
		Repository:    "test-repo",
		SmartBuildHit: true,
		Success:       true,
		Duration:      5.0,
		SchemaVersion: "1.0",
	}

	event := publisher.convertImageBuildMetadataToEvent(metadata)
	assert.Equal(t, expectedEvent, event)
}

func TestTrackImageBuild(t *testing.T) {
	ctx := context.Background()

	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {
				Cfg: &api.Config{},
			},
		},
	}

	ip := &Publisher{
		ioCtrl:            *io.NewIOController(),
		k8sClientProvider: test.NewFakeK8sProvider(),
	}

	meta := &analytics.ImageBuildMetadata{
		Namespace:                "test-namespace",
		Name:                     "test-image",
		DevenvName:               "test-devenv",
		RepoURL:                  "test-repo",
		RepoHash:                 "test-hash",
		BuildContextHash:         "test-context-hash",
		RepoHashDuration:         time.Duration(5) * time.Second,
		BuildContextHashDuration: time.Duration(5) * time.Second,
		CacheHitDuration:         time.Duration(5) * time.Second,
		BuildDuration:            time.Duration(5) * time.Second,
		CacheHit:                 true,
		Success:                  true,
	}

	c, _, err := ip.k8sClientProvider.Provide(&api.Config{})
	require.NoError(t, err)

	events, err := c.EventsV1().Events("test-namespace").List(ctx, v1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, events.Items, 0)

	ip.TrackImageBuild(ctx, meta)

	events, err = c.EventsV1().Events("test-namespace").List(ctx, v1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, events.Items, 1)
	insightEvent := events.Items[0]
	assert.Equal(t, "okteto_insights_build", insightEvent.Reason)
	assert.Equal(t, "Normal", insightEvent.Type)
	assert.Equal(t, "build", insightEvent.Action)
	assert.Contains(t, "build", insightEvent.Name)
}
