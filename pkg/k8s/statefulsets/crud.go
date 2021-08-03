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

package statefulsets

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//List returns the list of statefulsets
func List(ctx context.Context, namespace, labels string, c kubernetes.Interface) ([]appsv1.StatefulSet, error) {
	sfsList, err := c.AppsV1().StatefulSets(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels,
		},
	)
	if err != nil {
		return nil, err
	}
	return sfsList.Items, nil
}

//Get returns a statefulset object given its name and namespace
func Get(ctx context.Context, name, namespace string, c kubernetes.Interface) (*appsv1.StatefulSet, error) {
	sfs, err := c.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get statefulset %s/%s: %w", namespace, name, err)
	}

	return sfs, nil
}

//Create creates a statefulset
func Create(ctx context.Context, sfs *appsv1.StatefulSet, c kubernetes.Interface) error {
	_, err := c.AppsV1().StatefulSets(sfs.Namespace).Create(ctx, sfs, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

//Update updates a statefulset
func Update(ctx context.Context, sfs *appsv1.StatefulSet, c kubernetes.Interface) error {
	sfs.ResourceVersion = ""
	sfs.Status = appsv1.StatefulSetStatus{}
	_, err := c.AppsV1().StatefulSets(sfs.Namespace).Update(ctx, sfs, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

//Destroy removes a statefulset object given its name and namespace
func Destroy(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	if err := c.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error deleting kubernetes job: %s", err)
	}
	log.Infof("statefulset '%s' deleted", name)
	return nil
}

func IsRunning(ctx context.Context, namespace, svcName string, c kubernetes.Interface) bool {
	sfs, err := c.AppsV1().StatefulSets(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return sfs.Status.ReadyReplicas > 0
}
