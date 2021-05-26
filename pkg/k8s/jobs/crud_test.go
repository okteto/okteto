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

package jobs

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreate(t *testing.T) {
	ctx := context.Background()
	job := &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}

	clientset := fake.NewSimpleClientset()

	err := Create(ctx, job, clientset)
	if err != nil {
		t.Fatal(err)
	}
	retrieved, err := clientset.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retrieved, job) {
		t.Fatalf("Didn't created correctly")
	}
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	labels := map[string]string{"key": "value"}
	job := &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			Labels:    labels,
		},
	}

	clientset := fake.NewSimpleClientset(job)

	updatedLabels := map[string]string{"key": "value", "key2": "value2"}
	updatedJob := &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			Labels:    updatedLabels,
		},
	}
	err := Update(ctx, updatedJob, clientset)
	if err != nil {
		t.Fatal(err)
	}

	retrieved, err := clientset.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retrieved, updatedJob) {
		t.Fatalf("Didn't updated correctly")
	}
}

func TestDestroy(t *testing.T) {
	var tests = []struct {
		name      string
		jobName   string
		namespace string
		job       *batchv1.Job
	}{
		{
			name:      "existent-job",
			jobName:   "job-test",
			namespace: "test",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job-test",
					Namespace: "test",
				},
			},
		},
		{
			name:      "job-not-found",
			jobName:   "job-test",
			namespace: "test",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-existent-job",
					Namespace: "another-space",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			clientset := fake.NewSimpleClientset(tt.job)

			err := Destroy(ctx, tt.jobName, tt.namespace, clientset)

			if err != nil {
				t.Fatalf("unexpected error '%s'", err)
			}
		})
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()

	job := &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}
	clientset := fake.NewSimpleClientset(job)

	jobList, err := List(ctx, job.Namespace, "", clientset)
	if err != nil {
		t.Fatal(err)
	}

	if len(jobList) != 1 {
		t.Fatal(fmt.Errorf("Expected 1 job, found %d", len(jobList)))
	}

}
