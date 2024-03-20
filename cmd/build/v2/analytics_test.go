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

package v2

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestNewEventTracker(t *testing.T) {
	okCtx := &okteto.ContextStateless{
		Store: &okteto.ContextStore{
			Contexts: map[string]*okteto.Context{
				"test": {
					Cfg: &api.Config{},
				},
			},
			CurrentContext: "test",
		},
	}
	tracker := newEventTracker(io.NewIOController(), okCtx)
	assert.NotNil(t, tracker)
	assert.Equal(t, normalEventType, tracker.eventTypeConverter[true])
	assert.Equal(t, warningEventType, tracker.eventTypeConverter[false])
}

func TestConvertImageBuildMetadataToEvent(t *testing.T) {
	metadata := &analytics.ImageBuildMetadata{
		DevenvName:    "test-devenv",
		Name:          "test-image",
		Namespace:     "test-namespace",
		RepoURL:       "test-repo-url",
		CacheHit:      true,
		Success:       true,
		BuildDuration: time.Duration(10) * time.Second,
	}

	expectedEvent := eventJSON{
		DevenvName:    "test-devenv",
		ImageName:     "test-image",
		Namespace:     "test-namespace",
		Repository:    "test-repo-url",
		SmartBuildHit: true,
		Success:       true,
		Duration:      10.0,
		SchemaVersion: "1.0",
	}

	tracker := &eventTracker{}
	event := tracker.convertImageBuildMetadataToEvent(metadata)

	assert.Equal(t, expectedEvent, event)
}

func TestTrack(t *testing.T) {
	metadata := &analytics.ImageBuildMetadata{
		DevenvName:    "test-devenv",
		Name:          "test-image",
		Namespace:     "test-namespace",
		RepoURL:       "test-repo-url",
		CacheHit:      true,
		Success:       true,
		BuildDuration: time.Duration(10) * time.Second,
	}

	tracker := &eventTracker{
		k8sClient: fake.NewSimpleClientset(),
		eventTypeConverter: map[bool]EventType{
			true:  normalEventType,
			false: warningEventType,
		},
	}

	err := tracker.Track(context.Background(), metadata)
	assert.NoError(t, err)

	// Verify that the event was created
	events := tracker.k8sClient.EventsV1().Events(metadata.Namespace)
	eventList, err := events.List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, eventList.Items, 1)

	// Verify the event details
	event := eventList.Items[0]
	assert.Equal(t, "okteto-build-", event.GenerateName)
	assert.Equal(t, metadata.Namespace, event.Namespace)
	assert.Equal(t, buildEventReason, event.Reason)
	assert.Equal(t, reportingController, event.ReportingController)
	assert.Equal(t, config.VersionString, event.ReportingInstance)
	assert.Equal(t, string(normalEventType), event.Type)
	assert.Equal(t, "build", event.Action)
	assert.Equal(t, corev1.ObjectReference{Namespace: metadata.Namespace}, event.Regarding)

	// Verify the event note
	expectedEventJSON, _ := json.Marshal(tracker.convertImageBuildMetadataToEvent(metadata))
	assert.Equal(t, string(expectedEventJSON), event.Note)
}
