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

package deploy

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewEventTracker(t *testing.T) {
	k8sProvider := test.NewFakeK8sProvider()
	ioCtrl := io.NewIOController()

	tracker := newEventTracker(k8sProvider, ioCtrl)

	assert.NotNil(t, tracker)
	assert.Equal(t, normalEventType, tracker.eventTypeConverter[true])
	assert.Equal(t, warningEventType, tracker.eventTypeConverter[false])
	assert.Equal(t, ioCtrl, tracker.ioCtrl)
	assert.Equal(t, k8sProvider, tracker.k8sClientProvider)
}

func TestEventTracker_track(t *testing.T) {
	ctx := context.Background()

	name := "test-devenv"
	namespace := "test-namespace"
	success := true
	cfgName := pipeline.TranslatePipelineName(name)
	cfg := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfgName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"phases":     `[{"name": "phase1"}, {"name": "phase2"}]`,
			"repository": "test-repo-url",
		},
	}
	eventType := normalEventType

	k8sClienProvider := test.NewFakeK8sProvider(cfg)

	tracker := &eventTracker{
		k8sClientProvider: k8sClienProvider,
		eventTypeConverter: map[bool]EventType{
			true:  normalEventType,
			false: warningEventType,
		},
	}

	err := tracker.track(ctx, name, namespace, success)
	assert.NoError(t, err)

	c, _, err := k8sClienProvider.Provide(nil)
	assert.NoError(t, err)

	eventList, err := c.EventsV1().Events(namespace).List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, eventList.Items, 1)

	event := eventList.Items[0]
	assert.Equal(t, "okteto-deploy-", event.GenerateName)
	assert.Equal(t, namespace, event.Namespace)
	assert.Equal(t, deployEventReason, event.Reason)
	assert.Equal(t, reportingController, event.ReportingController)
	assert.Equal(t, config.VersionString, event.ReportingInstance)
	assert.Equal(t, string(eventType), event.Type)
	assert.Equal(t, "build", event.Action)
	assert.Equal(t, corev1.ObjectReference{Namespace: namespace}, event.Regarding)

	expectedEventJSON := &eventJSON{
		DevenvName:    name,
		Repository:    cfg.Data["repository"],
		Namespace:     namespace,
		Phase:         []phaseJSON{{Name: "phase1"}, {Name: "phase2"}},
		Success:       success,
		SchemaVersion: "1.0",
	}
	expectedEventBytes, _ := json.Marshal(expectedEventJSON)
	assert.Equal(t, string(expectedEventBytes), event.Note)
}
