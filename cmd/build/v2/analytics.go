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
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/events"
	"github.com/okteto/okteto/pkg/log/io"
	v1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type EventType string

const (
	normalEventType  EventType = "Normal"
	warningEventType EventType = "Warning"

	buildEventReason    = "okteto_insights_build"
	reportingController = "cli"
)

type eventTracker struct {
	eventTypeConverter map[bool]EventType
	k8sClient          kubernetes.Interface
	ioCtrl             *io.Controller
}

func newEventTracker(c kubernetes.Interface, ioCtrl *io.Controller) *eventTracker {
	return &eventTracker{
		eventTypeConverter: map[bool]EventType{
			true:  normalEventType,
			false: warningEventType,
		},
		ioCtrl:    ioCtrl,
		k8sClient: c,
	}
}

type eventJSON struct {
	DevenvName    string  `json:"devenv_name"`
	ImageName     string  `json:"image_name"`
	Namespace     string  `json:"namespace"`
	Repository    string  `json:"repository"`
	SmartBuildHit bool    `json:"smartBuildHit"`
	Success       bool    `json:"success"`
	SchemaVersion string  `json:"schemaVersion"`
	Duration      float64 `json:"duration"`
}

func (e *eventTracker) track(ctx context.Context, metadata *analytics.ImageBuildMetadata) error {
	if e.k8sClient == nil {
		return fmt.Errorf("k8s client is not available")
	}
	eventType := e.eventTypeConverter[metadata.Success]

	eventJSON, err := json.Marshal(convertImageBuildMetadataToEvent(metadata))
	if err != nil {
		return fmt.Errorf("failed to marshal event metadata: %s", err)
	}
	event := &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "okteto-build-",
			Namespace:    metadata.Namespace,
		},
		EventTime:           metav1.NewMicroTime(time.Now().UTC()),
		Reason:              buildEventReason,
		ReportingController: reportingController,
		ReportingInstance:   config.VersionString,
		Type:                string(eventType),
		Note:                string(eventJSON),
		Action:              "build",
		Regarding: v1.ObjectReference{
			Namespace: metadata.Namespace,
		},
	}

	return events.Create(ctx, event, e.k8sClient)

}

func convertImageBuildMetadataToEvent(metadata *analytics.ImageBuildMetadata) eventJSON {
	return eventJSON{
		DevenvName:    metadata.DevenvName,
		ImageName:     metadata.Name,
		Namespace:     metadata.Namespace,
		Repository:    metadata.RepoURL,
		SmartBuildHit: metadata.CacheHit,
		Success:       metadata.Success,
		Duration:      metadata.BuildDuration.Seconds(),
		SchemaVersion: "1.0",
	}
}
