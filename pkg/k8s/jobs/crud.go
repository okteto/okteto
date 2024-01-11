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

package jobs

import (
	"context"
	"fmt"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Create(ctx context.Context, job *batchv1.Job, c kubernetes.Interface) error {
	_, err := c.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func Update(ctx context.Context, job *batchv1.Job, c kubernetes.Interface) error {
	if err := Destroy(ctx, job.Name, job.Namespace, c); err != nil {
		return err
	}
	return Create(ctx, job, c)
}

func List(ctx context.Context, namespace, labels string, c kubernetes.Interface) ([]batchv1.Job, error) {
	jobList, err := c.BatchV1().Jobs(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels,
		},
	)
	if err != nil {
		return nil, err
	}
	return jobList.Items, nil
}

func Destroy(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	oktetoLog.Infof("deleting job '%s'", name)
	deletePropagation := metav1.DeletePropagationBackground
	err := c.BatchV1().Jobs(namespace).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &deletePropagation})
	if err != nil {
		if oktetoErrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error deleting kubernetes job: %w", err)
	}
	oktetoLog.Infof("job '%s' deleted", name)
	return nil
}

func IsRunning(ctx context.Context, namespace, svcName string, c kubernetes.Interface) bool {
	job, err := c.BatchV1().Jobs(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	if job.Status.Active != 0 || job.Status.Succeeded != 0 {
		return true
	}
	return false
}

func IsSuccedded(ctx context.Context, namespace, jobName string, c kubernetes.Interface) bool {
	job, err := c.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return job.Status.Succeeded == *job.Spec.Completions
}
func IsFailed(ctx context.Context, namespace, jobName string, c kubernetes.Interface) bool {
	job, err := c.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return job.Status.Failed > 0 && job.Status.Failed >= *job.Spec.BackoffLimit
}
