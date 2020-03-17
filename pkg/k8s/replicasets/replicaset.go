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

package replicasets

import (
	"fmt"

	"github.com/okteto/okteto/pkg/errors"
	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"
)

// GetReplicaSetByDeployment given a deployment, returns its current replica set or an error
func GetReplicaSetByDeployment(dev *model.Dev, d *appsv1.Deployment, c *kubernetes.Clientset) (*appsv1.ReplicaSet, error) {
	ls := fmt.Sprintf("%s=%s", okLabels.InteractiveDevLabel, dev.Name)
	rsList, err := c.AppsV1().ReplicaSets(d.Namespace).List(
		metav1.ListOptions{
			LabelSelector: ls,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get replicaset using %s: %s", ls, err)
	}

	for i := range rsList.Items {
		for _, or := range rsList.Items[i].OwnerReferences {
			if or.UID == d.UID {
				if v, ok := rsList.Items[i].Annotations[deploymentRevisionAnnotation]; ok && v == d.Annotations[deploymentRevisionAnnotation] {
					log.Infof("replicaset %s with revison %s is progressing", rsList.Items[i].Name, d.Annotations[deploymentRevisionAnnotation])
					return &rsList.Items[i], nil
				}
			}
		}
	}

	return nil, errors.ErrNotFound
}
