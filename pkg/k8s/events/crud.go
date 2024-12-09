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

	"github.com/okteto/okteto/pkg/errors"
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

const (
	unhealthyReason             = "Unhealthy"
	readinessProbeFailedMessage = "Readiness probe failed:"
	livenessProbeFailedMessage  = "Liveness probe failed:"
)

func readinessProbeFailed(event apiv1.Event) bool {
	return event.Reason == unhealthyReason && strings.HasPrefix(event.Message, readinessProbeFailedMessage)
}
func livenessProbeFailed(event apiv1.Event) bool {
	return event.Reason == unhealthyReason && strings.HasPrefix(event.Message, livenessProbeFailedMessage)
}

// GetUnhealthyEventFailure returns the message of the last event that caused the pod to be unhealthy
// this could be a readiness or liveness probe failure
func GetUnhealthyEventFailure(ctx context.Context, namespace, podName string, c kubernetes.Interface) error {
	events, err := List(ctx, namespace, podName, c)
	if err != nil {
		return nil
	}
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if readinessProbeFailed(event) {
			return errors.ErrReadinessProbeFailed
		}
		if livenessProbeFailed(event) {
			return errors.ErrLivenessProbeFailed
		}
	}
	return nil
}

func Create(ctx context.Context, event *eventsv1.Event, c kubernetes.Interface) error {
	_, err := c.EventsV1().Events(event.Namespace).Create(ctx, event, metav1.CreateOptions{})
	return err
}
