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

package services

import (
	"context"
	"fmt"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateDev deploys a default k8s service for a development container
func CreateDev(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) error {
	s := translate(dev, namespace)
	return Deploy(ctx, s, c)
}

// Deploy creates/updates a k8s service
func Deploy(ctx context.Context, s *apiv1.Service, c kubernetes.Interface) error {
	old, err := Get(ctx, s.Name, s.Namespace, c)
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return fmt.Errorf("error getting kubernetes service: %w", err)
	}

	if old == nil || old.Name == "" {
		oktetoLog.Infof("creating service '%s'", s.Name)
		_, err = c.CoreV1().Services(s.Namespace).Create(ctx, s, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating kubernetes service: %w", err)
		}
		oktetoLog.Infof("created service '%s'", s.Name)
	} else {
		oktetoLog.Infof("updating service '%s'", s.Name)
		op := "update"

		isDivertDeploy := old.Spec.Type == apiv1.ServiceTypeExternalName && old.Annotations[model.OktetoAutoCreateAnnotation] == "true"

		old.Annotations = s.Annotations
		old.Labels = s.Labels
		old.Spec.Ports = s.Spec.Ports
		old.Spec.Selector = s.Spec.Selector

		if (old.Spec.ClusterIP != "None" && s.Spec.ClusterIP == "") || s.Spec.ClusterIP == old.Spec.ClusterIP {
			// do nothing, keep the same clusterIP if the new one doesn't specify it
			// or if both are the same value
			oktetoLog.Debugf("keeping existing service ClusterIP %q", old.Spec.ClusterIP)
		} else {
			oktetoLog.Debugf("replacing service ClusterIP %q with %q", old.Spec.ClusterIP, s.Spec.ClusterIP)
			op = "replace"
		}

		if s.Spec.Type == apiv1.ServiceTypeClusterIP {
			if isDivertDeploy {
				old.Spec.Type = s.Spec.Type
				old.Spec.ExternalName = s.Spec.ExternalName
			}
		}

		switch op {
		case "replace":
			s.ResourceVersion = ""
			oktetoLog.Debugf("replacing service '%s'", s.Name)
			if err := c.CoreV1().Services(s.Namespace).Delete(ctx, s.Name, metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("error deleting kubernetes service: %w", err)
			}
			if _, err := c.CoreV1().Services(s.Namespace).Create(ctx, s, metav1.CreateOptions{}); err != nil {
				return fmt.Errorf("error creating kubernetes service: %w", err)
			}
		case "update":
			if _, err := c.CoreV1().Services(s.Namespace).Update(ctx, old, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("error updating kubernetes service: %w", err)
			}
		}
		oktetoLog.Infof("updated service '%s'.", s.Name)
	}
	return nil
}

// Get returns a kubernetes service by the name, or an error if it doesn't exist
func Get(ctx context.Context, name, namespace string, c kubernetes.Interface) (*apiv1.Service, error) {
	return c.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
}

// Update updates a k8s service
func Update(ctx context.Context, namespace string, svc *apiv1.Service, c kubernetes.Interface) (*apiv1.Service, error) {
	return c.CoreV1().Services(namespace).Update(ctx, svc, metav1.UpdateOptions{})
}

// List returns the list of services
func List(ctx context.Context, namespace, labels string, c kubernetes.Interface) ([]apiv1.Service, error) {
	svcList, err := c.CoreV1().Services(namespace).List(
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

// DestroyDev destroys the default service for a development container
func DestroyDev(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) error {
	return Destroy(ctx, dev.Name, namespace, c)
}

// Destroy destroys a k8s service
func Destroy(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	oktetoLog.Infof("deleting service '%s'", name)
	err := c.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if oktetoErrors.IsNotFound(err) {
			oktetoLog.Infof("service '%s' was already deleted.", name)
			return nil
		}
		return fmt.Errorf("error deleting kubernetes service: %w", err)
	}
	oktetoLog.Infof("service '%s' deleted", name)
	return nil
}

// GetServiceNameByLabel returns the name of the service with certain labels
func GetServiceNameByLabel(ctx context.Context, namespace string, c kubernetes.Interface, labels string) (string, error) {
	serviceList, err := c.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return "", err
	}
	foundServices := serviceList.Items
	if len(foundServices) == 0 {
		return "", fmt.Errorf("could not find any service with the following labels: '%s'", labels)
	} else if len(foundServices) == 1 {
		serviceInfo := foundServices[0].ObjectMeta
		return serviceInfo.Name, nil
	}
	servicesNames := GetServicesNamesFromList(serviceList)
	return "", fmt.Errorf("services [%s] have the following labels: '%s'.\nPlease specify the one you want to forward by name or use more specific labels", servicesNames, labels)
}

func GetServicesNamesFromList(serviceList *apiv1.ServiceList) string {
	names := make([]string, 0)

	for _, service := range serviceList.Items {
		names = append(names, service.ObjectMeta.Name)
	}
	return strings.Join(names, ", ")
}
