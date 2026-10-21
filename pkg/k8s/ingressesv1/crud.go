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

package ingressesv1

import (
	"context"
	"fmt"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Deploy(ctx context.Context, i *networkingv1.Ingress, c kubernetes.Interface) error {
	old, err := Get(ctx, i.Name, i.Namespace, c)
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return fmt.Errorf("error getting kubernetes ingress: %w", err)
	}

	if old == nil || old.Name == "" {
		oktetoLog.Infof("creating ingress '%s'", i.Name)
		_, err = c.NetworkingV1().Ingresses(i.Namespace).Create(ctx, i, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating kubernetes ingress: %w", err)
		}
		oktetoLog.Infof("created ingress '%s'", i.Name)
	} else {
		oktetoLog.Infof("updating ingress '%s'", i.Name)
		old.Annotations = i.Annotations
		old.Labels = i.Labels
		old.Spec = i.Spec
		_, err = c.NetworkingV1().Ingresses(i.Namespace).Update(ctx, old, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating kubernetes ingress: %w", err)
		}
		oktetoLog.Infof("updated ingress '%s'.", i.Name)
	}
	return nil
}

func Get(ctx context.Context, name, namespace string, c kubernetes.Interface) (*networkingv1.Ingress, error) {
	return c.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
}

// List returns the list of deployments
func List(ctx context.Context, namespace, labels string, c kubernetes.Interface) ([]networkingv1.Ingress, error) {
	iList, err := c.NetworkingV1().Ingresses(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels,
		},
	)
	if err != nil {
		return nil, err
	}
	return iList.Items, nil
}

// Destroy destroys a k8s deployment
func Destroy(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	oktetoLog.Infof("deleting ingress '%s'", name)
	err := c.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if oktetoErrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error deleting kubernetes ingress: %w", err)
	}
	oktetoLog.Infof("Ingress '%s' deleted", name)
	return nil
}
