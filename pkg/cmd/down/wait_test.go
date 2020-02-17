// Copyright 2020 The Okteto Authors
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

package down

import (
	"testing"

	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/model"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_waitForDevPodsTermination(t *testing.T) {
	var tests = []struct {
		name string
		dev  *model.Dev
	}{
		{
			name: "dev",
			dev: &model.Dev{
				Name: "dev",
			},
		},
		{
			name: "services",
			dev: &model.Dev{
				Name: "dev",
				Services: []*model.Dev{
					{
						Name: "service",
					},
				},
			},
		},
		{
			name: "not-found",
			dev: &model.Dev{
				Name: "not-found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &v1.Pod{}
			pod.SetName("dev-123")
			pod.SetNamespace("ns")
			pod.Labels = map[string]string{labels.InteractiveDevLabel: "dev"}
			pod.ObjectMeta.SetDeletionTimestamp(&metav1.Time{})

			dPod := &v1.Pod{}
			dPod.SetName("service-123")
			dPod.SetNamespace("ns")
			dPod.Labels = map[string]string{labels.DetachedDevLabel: "service"}
			dPod.ObjectMeta.SetDeletionTimestamp(&metav1.Time{})

			client := fake.NewSimpleClientset(pod, dPod)
			waitForDevPodsTermination(client, tt.dev, 5)
		})
	}

}
