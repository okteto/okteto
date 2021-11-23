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

package endpoints

import (
	"context"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// List returns the list of endpoints
func List(ctx context.Context, namespace, labels string, c kubernetes.Interface) ([]apiv1.Endpoints, error) {
	svcList, err := c.CoreV1().Endpoints(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels,
		},
	)
	if err != nil {
		return nil, err
	}
	return svcList.Items, nil
}

// GetEndpointsByPodName returns the list of endpoints containing a pod
func GetEndpointsByPodName(ctx context.Context, podNames []string, namespace string, c *kubernetes.Clientset) ([]apiv1.Endpoints, error) {
	endpointsList, err := List(ctx, namespace, "", c)
	if err != nil {
		return nil, err
	}

	result := []apiv1.Endpoints{}
	for _, endpoint := range endpointsList {
		isAdded := false
		for _, subset := range endpoint.Subsets {
			for _, address := range subset.Addresses {
				if address.TargetRef == nil {
					continue
				}
				if isOnPodList(podNames, address.TargetRef.Name) && !isAlreadyAdded(result, endpoint) {
					result = append(result, endpoint)
					isAdded = true
				}
				if isAdded {
					break
				}
			}
			if isAdded {
				break
			}
		}
	}
	return result, nil
}

func isOnPodList(podNames []string, refName string) bool {
	for _, pName := range podNames {
		if pName == refName {
			return true
		}
	}
	return false
}

func isAlreadyAdded(endpointList []apiv1.Endpoints, endpoint apiv1.Endpoints) bool {
	for _, e := range endpointList {
		if e.UID == endpoint.UID {
			return true
		}
	}
	return false
}
