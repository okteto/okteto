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

package ingresses

import (
	"context"
	"fmt"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Client struct {
	c    kubernetes.Interface
	isV1 bool
}

func NewIngressClient(c kubernetes.Interface, isV1 bool) *Client {
	return &Client{
		c:    c,
		isV1: isV1,
	}
}

type Ingress struct {
	V1      *networkingv1.Ingress
	V1Beta1 *networkingv1beta1.Ingress
}

func GetClient(c kubernetes.Interface) (*Client, error) {
	rList, err := c.Discovery().ServerResourcesForGroupVersion("networking.k8s.io/v1")
	if err != nil {
		return nil, err
	}
	for _, apiResource := range rList.APIResources {
		if apiResource.Kind == "Ingress" {
			return NewIngressClient(c, true), nil
		}
	}

	return NewIngressClient(c, false), nil
}

// Get results the ingress
func (iClient *Client) Get(ctx context.Context, name, namespace string) (metav1.Object, error) {
	if iClient.isV1 {
		i, err := iClient.c.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return i.GetObjectMeta(), nil
	}

	i, err := iClient.c.NetworkingV1beta1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return i.GetObjectMeta(), nil
}

func (iClient *Client) Create(ctx context.Context, i *Ingress) error {
	if iClient.isV1 {
		_, err := iClient.c.NetworkingV1().Ingresses(i.V1.Namespace).Create(ctx, i.V1, metav1.CreateOptions{})
		return err
	}
	_, err := iClient.c.NetworkingV1beta1().Ingresses(i.V1Beta1.Namespace).Create(ctx, i.V1Beta1, metav1.CreateOptions{})
	return err
}

// Update updates a statefulset
func (iClient *Client) Update(ctx context.Context, i *Ingress) error {
	if iClient.isV1 {
		_, err := iClient.c.NetworkingV1().Ingresses(i.V1.Namespace).Update(ctx, i.V1, metav1.UpdateOptions{})
		return err
	}
	_, err := iClient.c.NetworkingV1beta1().Ingresses(i.V1Beta1.Namespace).Update(ctx, i.V1Beta1, metav1.UpdateOptions{})
	return err
}

// List returns the list of deployments
func (iClient *Client) List(ctx context.Context, namespace, labels string) ([]metav1.Object, error) {
	result := []metav1.Object{}
	if iClient.isV1 {
		iList, err := iClient.c.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels})
		if err != nil {
			return nil, err
		}
		for i := range iList.Items {
			result = append(result, iList.Items[i].GetObjectMeta())
		}
		return result, nil
	}

	iList, err := iClient.c.NetworkingV1beta1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return nil, err
	}
	for i := range iList.Items {
		result = append(result, iList.Items[i].GetObjectMeta())
	}
	return result, nil
}

// Destroy destroys a k8s deployment
func (iClient *Client) Destroy(ctx context.Context, name, namespace string) error {
	oktetoLog.Infof("deleting ingress '%s'", name)
	if iClient.isV1 {
		err := iClient.c.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil && !oktetoErrors.IsNotFound(err) {
			return fmt.Errorf("error deleting kubernetes ingress: %w", err)
		}
		return nil
	}

	err := iClient.c.NetworkingV1beta1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return fmt.Errorf("error deleting kubernetes ingress: %w", err)
	}
	return nil
}

func (iClient *Client) GetEndpointsBySelector(ctx context.Context, namespace, labels string) ([]string, error) {
	result := make([]string, 0)
	if iClient.isV1 {
		iList, err := iClient.c.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels})
		if err != nil {
			return nil, err
		}
		for i := range iList.Items {
			for _, rule := range iList.Items[i].Spec.Rules {
				if rule.Host == "" {
					continue
				}
				for _, path := range rule.IngressRuleValue.HTTP.Paths {
					result = append(result, fmt.Sprintf("https://%s%s", rule.Host, path.Path))
				}
			}
		}
		return result, nil
	}

	iList, err := iClient.c.NetworkingV1beta1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return nil, err
	}
	for i := range iList.Items {
		for _, rule := range iList.Items[i].Spec.Rules {
			for _, path := range rule.IngressRuleValue.HTTP.Paths {
				result = append(result, fmt.Sprintf("https://%s%s", rule.Host, path.Path))
			}
		}
	}
	return result, nil
}

// GetName gets the name of the ingress
func (i Ingress) GetName() string {
	if i.V1 != nil {
		return i.V1.Name
	}
	return i.V1Beta1.Name
}

// GetNamespace gets the namespace of the ingress
func (i Ingress) GetNamespace() string {
	if i.V1 != nil {
		return i.V1.Namespace
	}
	return i.V1Beta1.Namespace
}

// GetLabels gets the labels of the ingress
func (i Ingress) GetLabels() map[string]string {
	if i.V1 != nil {
		return i.V1.Labels
	}
	return i.V1Beta1.Labels
}

// GetAnnotations gets the annotations of the ingress
func (i Ingress) GetAnnotations() map[string]string {
	if i.V1 != nil {
		return i.V1.Annotations
	}
	return i.V1Beta1.Annotations
}

// Deploy creates or updates an ingress
func (iClient *Client) Deploy(ctx context.Context, ingress *Ingress) error {
	if _, err := iClient.Get(ctx, ingress.GetName(), ingress.GetNamespace()); err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return fmt.Errorf("error getting ingress '%s': %w", ingress.GetName(), err)
		}
		if err := iClient.Create(ctx, ingress); err != nil {
			return err
		}
		oktetoLog.Success("Endpoint '%s' created", ingress.GetName())
		return nil
	}

	if err := iClient.Update(ctx, ingress); err != nil {
		return err
	}
	oktetoLog.Success("Endpoint '%s' updated", ingress.GetName())
	return nil
}
