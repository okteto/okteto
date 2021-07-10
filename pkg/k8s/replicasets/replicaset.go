// Copyright 2021 The Okteto Authors
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

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"
)

// GetReplicaSetByDeployment given a deployment, returns its current replica set or an error
func GetReplicaSetByDeployment(ctx context.Context, d *appsv1.Deployment, labels string, c *kubernetes.Clientset) (*appsv1.ReplicaSet, error) {
	rsList, err := c.AppsV1().ReplicaSets(d.Namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get replicaset using %s: %s", labels, err)
	}

	for i := range rsList.Items {
		for _, or := range rsList.Items[i].OwnerReferences {
			if or.UID == d.UID {
				if v, ok := rsList.Items[i].Annotations[deploymentRevisionAnnotation]; ok && v == d.Annotations[deploymentRevisionAnnotation] {
					return &rsList.Items[i], nil
				}
			}
		}
	}
	return nil, nil
}

// GetReplicaSetByStatefulset given a statefulset, returns its current replica set or an error
func GetReplicaSetByStatefulset(ctx context.Context, sfs *appsv1.StatefulSet, labels string, c *kubernetes.Clientset) (*appsv1.ReplicaSet, error) {
	rsList, err := c.AppsV1().ReplicaSets(sfs.Namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get replicaset using %s: %s", labels, err)
	}

	for i := range rsList.Items {
		for _, or := range rsList.Items[i].OwnerReferences {
			if or.UID == sfs.UID {
				return &rsList.Items[i], nil
				// if v, ok := rsList.Items[i].Annotations[deploymentRevisionAnnotation]; ok && v == sfs.Annotations[deploymentRevisionAnnotation] {
				// 	return &rsList.Items[i], nil
				// }
			}
		}
	}
	return nil, nil
}
