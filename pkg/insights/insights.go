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
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/events"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// reportingController represents the controller that is reporting the event
	reportingController = "cli"

	// insightLabel represents the label used to identify the insights events
	insightLabel = "events.okteto.com"
)

// Publisher is a struct that represents the insights publisher
type Publisher struct {
	k8sClientProvider okteto.K8sClientProvider
	ioCtrl            io.Controller
}

// NewInsightsPublisher creates a new insights publisher
func NewInsightsPublisher(k8sClientProvider okteto.K8sClientProvider, ioCtrl io.Controller) *Publisher {
	return &Publisher{
		k8sClientProvider: k8sClientProvider,
		ioCtrl:            ioCtrl,
	}
}

// trackEvent tracks an event in the cluster
// namespace: the namespace where the event is happening
// insightType: the type of the event (for example: build, deploy, etc.)
// data: the data of the event as JSON string
func (ip *Publisher) trackEvent(ctx context.Context, namespace, insightType, data string) {
	k8sClient, _, err := ip.k8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		ip.ioCtrl.Logger().Infof("could not get k8s client: %s", err)
		return
	}

	event := &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("okteto-%s-", insightType),
			Namespace:    namespace,
			Labels: map[string]string{
				insightLabel: "true",
			},
		},
		EventTime:           metav1.NewMicroTime(time.Now().UTC()),
		Reason:              fmt.Sprintf("okteto_insights_%s", insightType),
		ReportingController: reportingController,
		ReportingInstance:   config.VersionString,
		Type:                "Normal",
		Note:                data,
		Action:              insightType,
		Regarding: corev1.ObjectReference{
			Namespace: namespace,
		},
	}

	if err := events.Create(ctx, event, k8sClient); err != nil {
		ip.ioCtrl.Logger().Infof("failed to create event: %s", err)
	}
}
