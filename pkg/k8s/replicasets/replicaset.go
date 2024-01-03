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

package replicasets

import (
	"context"
	"fmt"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetReplicaSetByDeployment given a deployment, returns its current replica set or an error
func GetReplicaSetByDeployment(ctx context.Context, d *appsv1.Deployment, c kubernetes.Interface) (*appsv1.ReplicaSet, error) {
	rsList, err := c.AppsV1().ReplicaSets(d.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get replicasets: %w", err)
	}

	for i := range rsList.Items {
		for _, or := range rsList.Items[i].OwnerReferences {
			if or.UID == d.UID {
				if v, ok := rsList.Items[i].Annotations[model.DeploymentRevisionAnnotation]; ok && v == d.Annotations[model.DeploymentRevisionAnnotation] {
					return &rsList.Items[i], nil
				}
			}
		}
	}
	return nil, oktetoErrors.ErrNotFound
}
