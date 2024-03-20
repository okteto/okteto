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

package deploy

import (
	"context"
	"encoding/json"
	"time"

	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/k8s/events"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	v1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EventType string

const (
	normalEventType  EventType = "Normal"
	warningEventType EventType = "Warning"

	deployEventReason   = "okteto_insights_deploy"
	reportingController = "cli"
)

type eventTracker struct {
	eventTypeConverter map[bool]EventType
	k8sClientProvider  okteto.K8sClientProvider
	ioCtrl             *io.Controller
}

func newEventTracker(k8sProvider okteto.K8sClientProvider, ioCtrl *io.Controller) *eventTracker {
	return &eventTracker{
		eventTypeConverter: map[bool]EventType{
			true:  normalEventType,
			false: warningEventType,
		},
		ioCtrl:            ioCtrl,
		k8sClientProvider: k8sProvider,
	}
}

type eventJSON struct {
	DevenvName    string      `json:"devenv_name"`
	Repository    string      `json:"repository"`
	Namespace     string      `json:"namespace"`
	SchemaVersion string      `json:"schemaVersion"`
	Phase         []phaseJSON `json:"phase"`
	Success       bool        `json:"success"`
}

type phaseJSON struct {
	Name     string  `json:"name"`
	Duration float64 `json:"duration"`
}

func (e *eventTracker) track(ctx context.Context, name, namespace string, success bool) error {
	c, _, err := e.k8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}
	cfgName := pipeline.TranslatePipelineName(name)
	cfg, err := configmaps.Get(ctx, cfgName, namespace, c)
	if err != nil {
		return err
	}
	eventType := e.eventTypeConverter[success]

	val, ok := cfg.Data["phases"]
	// If there is no phases, we don't track the event
	if !ok {
		return nil
	}

	var phases []phaseJSON
	if err := json.Unmarshal([]byte(val), &phases); err != nil {
		return err
	}

	eventJSON := &eventJSON{
		DevenvName:    name,
		Repository:    cfg.Data["repository"],
		Namespace:     namespace,
		Phase:         phases,
		Success:       success,
		SchemaVersion: "1.0",
	}

	eventBytes, err := json.Marshal(eventJSON)
	if err != nil {
		return err
	}

	event := &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "okteto-deploy-",
			Namespace:    cfg.Namespace,
		},
		EventTime:           metav1.NewMicroTime(time.Now().UTC()),
		Reason:              deployEventReason,
		ReportingController: reportingController,
		ReportingInstance:   config.VersionString,
		Type:                string(eventType),
		Note:                string(eventBytes),
		Action:              "build",
		Regarding: v1.ObjectReference{
			Namespace: namespace,
		},
	}

	return events.Create(ctx, event, c)

}
