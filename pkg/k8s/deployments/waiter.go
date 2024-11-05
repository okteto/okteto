// Copyright 2024 The Okteto Authors
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

package deployments

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type Waiter struct {
	Client kubernetes.Interface
}

func (w *Waiter) AllDeploymentsHealthy(ctx context.Context, namespace, labelSelector string) error {
	c := w.Client

	dList, err := c.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return err
	}
	pendingDeployments := make(map[string]bool)
	for _, d := range dList.Items {
		if d.Status.ReadyReplicas == 0 {
			pendingDeployments[d.Name] = true
		}
	}

	if len(pendingDeployments) > 0 {
		w, err := c.AppsV1().Deployments(namespace).Watch(ctx, metav1.ListOptions{
			LabelSelector:   labelSelector,
			ResourceVersion: dList.ResourceVersion,
		})
		if err != nil {
			return err
		}
		defer w.Stop()

		for {
			event, openChan := <-w.ResultChan()
			if !openChan {
				break
			}

			switch event.Type {
			case watch.Error:
				// from docs "If Type is Error: *api.Status is recommended; other types may make sense depending on context."
				errObject := apierrors.FromObject(event.Object)
				statusErr, ok := errObject.(*apierrors.StatusError)
				if !ok {
					return fmt.Errorf("error watching deployments to be ready")
				}
				return fmt.Errorf("error watching deployments to be ready: %s", statusErr.Error())

			case watch.Added, watch.Modified:
				deployment, ok := event.Object.(*appsv1.Deployment)
				if !ok {
					continue
				}
				if _, ok := pendingDeployments[deployment.Name]; ok && deployment.Status.ReadyReplicas > 0 {
					delete(pendingDeployments, deployment.Name)
				}
				if len(pendingDeployments) == 0 {
					break
				}
			case watch.Bookmark, watch.Deleted:
				continue
			}
		}
	}
	return nil
}
