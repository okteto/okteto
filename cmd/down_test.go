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

package cmd

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_waitForDevPodTermination(t *testing.T) {
	pod := &v1.Pod{}
	pod.SetName("test")
	pod.SetNamespace("ns")
	pod.ObjectMeta.SetDeletionTimestamp(&metav1.Time{})
	client := fake.NewSimpleClientset(pod)
	waitForDevPodTermination(client, pod)

	pod.SetName("not-found")
	waitForDevPodTermination(client, pod)
}
