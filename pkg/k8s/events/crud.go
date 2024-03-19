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

package events

import (
	"context"
	"fmt"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func List(ctx context.Context, namespace, podName string, c kubernetes.Interface) ([]apiv1.Event, error) {
	events, err := c.CoreV1().Events(namespace).List(
		ctx,
		metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s", podName),
		},
	)
	if err != nil {
		return nil, err
	}
	return events.Items, err
}

func GetUnhealthyEventFailure(ctx context.Context, namespace, podName string, c kubernetes.Interface) string {
	events, err := List(ctx, namespace, podName, c)
	if err != nil {
		return ""
	}
	previousKilling := false
	for i := len(events) - 1; i >= 0; i-- {
		if previousKilling {
			if events[i].Reason == "Unhealthy" && strings.Contains(events[i].Message, "probe failed") {
				return events[i].Message
			}
		}
		if events[i].Reason == "Killing" && (strings.Contains(events[i].Message, "failed liveness probe") || strings.Contains(events[i].Message, "failed readiness probe")) {
			previousKilling = true
		}
	}
	return ""
}

func Create(ctx context.Context, event *eventsv1.Event, c kubernetes.Interface) error {
	_, err := c.EventsV1().Events(event.Namespace).Create(ctx, event, metav1.CreateOptions{})
	return err
}
