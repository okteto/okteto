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

package stack

import (
	"context"
	"strconv"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"
)

func Test_translateConfigMapToStack(t *testing.T) {
	configMap := &apiv1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "namespace",
		},
		Data: map[string]string{
			NameField:    "test",
			ComposeField: strconv.FormatBool(true),
			YamlField:    `c2VydmljZXM6CiAgcHJvZHVjZXI6CiAgICBidWlsZDogLi9wcm9kdWNlcgogICAgZW52X2ZpbGU6IC4vcHJvZHVjZXIvLmVudi5wcm9kdWNlcgogICAgZGVwZW5kc19vbjoKICAgICAgcmFiYml0bXE6CiAgICAgICAgY29uZGl0aW9uOiBzZXJ2aWNlX2hlYWx0aHkKICAgICAgbW9uZ29kYjoKICAgICAgICBjb25kaXRpb246IHNlcnZpY2VfaGVhbHRoeQogICAgICBpbml0aWFsaXplLXF1ZXVlOgogICAgICAgIGNvbmRpdGlvbjogc2VydmljZV9jb21wbGV0ZWRfc3VjY2Vzc2Z1bGx5CiAgICB2b2x1bWVzOiAKICAgICAgLSBwcm9kdWNlci1saWJzOi91c3IvbG9jYWwvbGliL3B5dGhvbjMuOC9zaXRlLXBhY2thZ2VzCiAgICBjb250YWluZXJfbmFtZTogcHJvZHVjZXIKICBjb25zdW1lcjoKICAgIGJ1aWxkOiAuL2NvbnN1bWVyCiAgICBlbnZpcm9ubWVudDogCiAgICAgIC0gUFlUSE9OVU5CVUZGRVJFRD0xCiAgICAgIC0gUkFCQklUTVFfVVNFUj1yYWJiaXRtcQogICAgICAtIFJBQkJJVE1RX1BBU1MKICAgICAgLSBSQUJCSVRNUV9QT1JUPTU2NzIKICAgICAgLSBUQVNLX1FVRVVFPXRhc2tfcXVldWUKICAgIHZvbHVtZXM6IAogICAgICAtIC91c3IvbG9jYWwvbGliL3B5dGhvbjMuOC9zaXRlLXBhY2thZ2VzCiAgICBkZXBlbmRzX29uOgogICAgICByYWJiaXRtcToKICAgICAgICBjb25kaXRpb246IHNlcnZpY2VfaGVhbHRoeQogICAgICBtb25nb2RiOgogICAgICAgIGNvbmRpdGlvbjogc2VydmljZV9oZWFsdGh5CiAgICAgIHByb2R1Y2VyOgogICAgICAgIGNvbmRpdGlvbjogc2VydmljZV9zdGFydGVkCiAgaW5pdGlhbGl6ZS1xdWV1ZToKICAgIGJ1aWxkOiAuL2pvYgogICAgcmVzdGFydDogb24tZmFpbHVyZQogICAgZGVwZW5kc19vbjoKICAgICAgcmFiYml0bXE6CiAgICAgICAgY29uZGl0aW9uOiBzZXJ2aWNlX2hlYWx0aHkKICAgIGVudl9maWxlOiAuL2pvYi8uZW52LmpvYiAKICAgIHZvbHVtZXM6IAogICAgICAtIGpvYi1saWJzOi91c3IvbG9jYWwvbGliL3B5dGhvbjMvc2l0ZS1wYWNrYWdlcwogICAgICAtIC91c3IvbG9jYWwvbGliL3B5dGhvbjMuOC9zaXRlLXBhY2thZ2VzCiAgYXBpOgogICAgYnVpbGQ6IC4vYXBpCiAgICBkZXBlbmRzX29uOgogICAgICBtb25nb2RiOgogICAgICAgIGNvbmRpdGlvbjogc2VydmljZV9oZWFsdGh5CiAgbmdpbng6CiAgICBpbWFnZTogbmdpbngKICAgIHZvbHVtZXM6CiAgICAgIC0gLi9uZ2lueC9uZ2lueC5jb25mOi90bXAvbmdpbnguY29uZgogICAgY29tbWFuZDogL2Jpbi9iYXNoIC1jICJlbnZzdWJzdCA8IC90bXAvbmdpbnguY29uZiA+IC9ldGMvbmdpbngvY29uZi5kL2RlZmF1bHQuY29uZiAmJiBuZ2lueCAtZyAnZGFlbW9uIG9mZjsnIgogICAgZW52aXJvbm1lbnQ6CiAgICAgIC0gRkxBU0tfU0VSVkVSX0FERFI9YXBpOjUwMDAKICAgIHBvcnRzOgogICAgICAtIDgyOjgwCiAgICAgIC0gODM6ODAKICAgIGRlcGVuZHNfb246CiAgICAgIGFwaToKICAgICAgICBjb25kaXRpb246IHNlcnZpY2Vfc3RhcnRlZAogICAgY29udGFpbmVyX25hbWU6IHdlYi1zdmMKICByYWJiaXRtcToKICAgIGltYWdlOiByYWJiaXRtcTptYW5hZ2VtZW50CiAgICBlbnZpcm9ubWVudDoKICAgICAgLSBSQUJCSVRNUV9ERUZBVUxUX1VTRVI9cmFiYml0bXEKICAgICAgLSBSQUJCSVRNUV9ERUZBVUxUX1BBU1M9cmFiYml0bXEKICAgIHZvbHVtZXM6CiAgICAgIC0gL3Zhci9sb2cvcmFiYml0bXEvCiAgICAgIC0gL3Zhci91c3IvcmFiYml0bXEvCiAgICAgIC0gcmFiYml0bXEtZGF0YTovdmFyL2xpYi9yYWJiaXRtcQogIG1vbmdvZGI6CiAgICBpbWFnZTogbW9uZ286bGF0ZXN0CiAgICB2b2x1bWVzOgogICAgICAtIG1vbmdvZGItZGF0YTovZGF0YS9kYgp2b2x1bWVzOgogIHJhYmJpdG1xLWRhdGE6CiAgICBzaXplOiAwLjVHaQogIG1vbmdvZGItZGF0YToKICAgIHNpemU6IDAuNUdpCiAgcHJvZHVjZXItbGliczoKICAgIHNpemU6IDAuNUdpCiAgam9iLWxpYnM6CiAgICBzaXplOiAwLjVHaQo=`,
		},
	}
	stack, err := translateConfigMapToStack(configMap)
	if err != nil {
		t.Fatalf("Wrong translation: %s", err.Error())
	}
	if stack.Name != "test" {
		t.Fatal("Wrong translation of Name field")
	}
	if stack.Namespace != "namespace" {
		t.Fatal("Wrong translation of Nsamespace field")
	}
	if !stack.IsCompose {
		t.Fatal("Wrong translation of IsCompose field")
	}
	if len(stack.Volumes) != 5 {
		t.Fatal("Wrong translation of Volumes field")
	}
	if len(stack.Services) != 7 {
		t.Fatal("Wrong translation of Services field")
	}
	if len(stack.Endpoints) != 2 {
		t.Fatal("Wrong translation of Endpoints field")
	}
}

func Test_waitForSvcRunning(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      "svcToBeCompleted",
			Namespace: "namespace",
		},
		Spec: batchv1.JobSpec{
			Completions: pointer.Int32Ptr(2),
		},
		Status: batchv1.JobStatus{
			Succeeded: 2,
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "svcToBeCompleted",
			Namespace: "namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 2,
		},
	}
	sfs := &appsv1.StatefulSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      "svcToBeCompleted",
			Namespace: "namespace",
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: 2,
		},
	}
	fakeClient := fake.NewSimpleClientset(job, sfs, deployment)
	ctx := context.Background()
	tests := []struct {
		name    string
		stack   *model.Stack
		svcName string
	}{
		{
			name: "wait for deployment",
			stack: &model.Stack{
				Namespace: "namespace",
				Services: map[string]*model.Service{
					"svcToBeCompleted": {
						RestartPolicy: apiv1.RestartPolicyAlways,
					},
				},
			},
			svcName: "svcToBeCompleted",
		},
		{
			name: "wait for sfs",
			stack: &model.Stack{
				Namespace: "namespace",
				Services: map[string]*model.Service{
					"svcToBeCompleted": {
						RestartPolicy: apiv1.RestartPolicyAlways,
						Volumes: []model.StackVolume{
							{
								LocalPath:  "/usr",
								RemotePath: "/usr",
							},
						},
					},
				},
			},
			svcName: "svcToBeCompleted",
		},
		{
			name: "wait for job",
			stack: &model.Stack{
				Namespace: "namespace",
				Services: map[string]*model.Service{
					"svcToBeCompleted": {
						RestartPolicy: apiv1.RestartPolicyNever,
						Volumes: []model.StackVolume{
							{
								LocalPath:  "/usr",
								RemotePath: "/usr",
							},
						},
					},
				},
			},
			svcName: "svcToBeCompleted",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := waitForSvcRunning(ctx, tt.stack, tt.svcName, fakeClient)
			if err != nil {
				t.Fatal("Not waited properly")
			}
		})
	}
}

func Test_waitForSvcCompleted(t *testing.T) {
	ctx := context.Background()
	stack := &model.Stack{
		Namespace: "namespace",
		Services: map[string]*model.Service{
			"svcToBeCompleted": {},
		},
	}

	job := &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      "svcToBeCompleted",
			Namespace: "namespace",
		},
		Spec: batchv1.JobSpec{
			Completions: pointer.Int32Ptr(2),
		},
		Status: batchv1.JobStatus{
			Succeeded: 2,
		},
	}
	fakeClient := fake.NewSimpleClientset(job)
	err := waitForSvcCompleted(ctx, stack, "svcToBeCompleted", fakeClient)
	if err != nil {
		t.Fatal("Not waited properly")
	}
}
