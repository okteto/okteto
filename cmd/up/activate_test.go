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

package up

import (
	"context"
	"testing"
	"time"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestWaitUntilAppAwaken(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Cfg: &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	tt := []struct {
		expectedErr          error
		oktetoClientProvider *test.FakeK8sProvider
		name                 string
		autocreate           bool
	}{
		{
			name:        "dev is autocreate",
			autocreate:  true,
			expectedErr: nil,
		},
		{
			name:       "failed to provide k8s client",
			autocreate: false,
			oktetoClientProvider: &test.FakeK8sProvider{
				ErrProvide: assert.AnError,
			},
			expectedErr: assert.AnError,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			up := &upContext{
				Dev: &model.Dev{
					Autocreate: tc.autocreate,
				},
				K8sClientProvider: tc.oktetoClientProvider,
			}
			err := up.waitUntilAppIsAwaken(context.Background(), nil)
			assert.ErrorIs(t, tc.expectedErr, err)
		})
	}
}

func TestWaitUntilDevelopmentContainerIsRunning(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Cfg: &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	tt := []struct {
		expectedErr          error
		oktetoClientProvider *test.FakeK8sProvider
		name                 string
	}{
		{
			name: "failed to provide k8s client",
			oktetoClientProvider: &test.FakeK8sProvider{
				ErrProvide: assert.AnError,
			},
			expectedErr: assert.AnError,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			up := &upContext{
				Dev:               &model.Dev{},
				K8sClientProvider: tc.oktetoClientProvider,
			}
			err := up.waitUntilDevelopmentContainerIsRunning(context.Background(), nil)
			assert.ErrorIs(t, tc.expectedErr, err)
		})
	}
}

func TestWaitUntilDevelopmentContainerIsRunningInitContainer(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{Contexts: map[string]*okteto.Context{"test": {Cfg: &api.Config{}}}, CurrentContext: "test"}
	cases := []struct {
		name    string
		status  apiv1.ContainerState
		phase   apiv1.PodPhase
		wantErr string
	}{
		{name: "failed init container", status: apiv1.ContainerState{Terminated: &apiv1.ContainerStateTerminated{ExitCode: 1, Reason: "Error", Message: "setup failed"}}, phase: apiv1.PodPending, wantErr: "init container setup failed: Error (exit code 1): setup failed"},
		{name: "crashing init container", status: apiv1.ContainerState{Waiting: &apiv1.ContainerStateWaiting{Reason: "CrashLoopBackOff", Message: "back-off restarting failed container"}}, phase: apiv1.PodPending, wantErr: "init container setup failed: CrashLoopBackOff: back-off restarting failed container"},
		{name: "still running init container", status: apiv1.ContainerState{Running: &apiv1.ContainerStateRunning{}}, phase: apiv1.PodPending},
		{name: "initializing container", status: apiv1.ContainerState{Waiting: &apiv1.ContainerStateWaiting{Reason: "PodInitializing"}}, phase: apiv1.PodPending},
		{name: "restarting native sidecar", status: apiv1.ContainerState{Waiting: &apiv1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}, phase: apiv1.PodRunning},
		{name: "successful init container", status: apiv1.ContainerState{Terminated: &apiv1.ContainerStateTerminated{ExitCode: 0}}, phase: apiv1.PodRunning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := test.NewFakeK8sProvider()
			iface, _, err := provider.Provide(&api.Config{})
			assert.NoError(t, err)
			client := iface.(*fake.Clientset)
			podWatch := watch.NewRaceFreeFake()
			eventWatch := watch.NewRaceFreeFake()
			defer podWatch.Stop()
			defer eventWatch.Stop()
			client.PrependWatchReactor("pods", ktesting.DefaultWatchReactor(podWatch, nil))
			client.PrependWatchReactor("events", ktesting.DefaultWatchReactor(eventWatch, nil))
			pod := &apiv1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "dev-pod", UID: "test-pod"}, Status: apiv1.PodStatus{Phase: tc.phase, InitContainerStatuses: []apiv1.ContainerStatus{{Name: "setup", State: tc.status}}}}
			unrelated := pod.DeepCopy()
			unrelated.UID = "another-pod"
			unrelated.Status.InitContainerStatuses[0].State = apiv1.ContainerState{Terminated: &apiv1.ContainerStateTerminated{ExitCode: 1}}
			podWatch.Add(unrelated)
			podWatch.Add(pod)
			if tc.wantErr == "" && tc.phase != apiv1.PodRunning {
				running := pod.DeepCopy()
				running.Status.Phase = apiv1.PodRunning
				podWatch.Add(running)
			}
			up := &upContext{Dev: &model.Dev{Name: "test"}, Pod: pod, Namespace: "test", K8sClientProvider: provider}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err = up.waitUntilDevelopmentContainerIsRunning(ctx, nil)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}
