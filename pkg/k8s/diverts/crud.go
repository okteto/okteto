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

package diverts

import (
	"context"
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/ingressesv1"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Create(ctx context.Context, dev *model.Dev, isOktetoNamespace bool, c kubernetes.Interface) error {
	if !isOktetoNamespace {
		return errors.ErrDivertNotSupported
	}

	username := okteto.GetSanitizedUsername()

	r, err := divertResource(ctx, dev, username, c)
	if err != nil {
		return err
	}

	s, err := divertService(ctx, dev, r, username, c)
	if err != nil {
		return err
	}

	i, err := divertIngress(ctx, dev, username, c)
	if err != nil {
		return err
	}

	if err := createDivertCRD(ctx, dev, username, i, s, c); err != nil {
		return err
	}

	translateDev(username, dev, r)
	return nil
}

func divertResource(ctx context.Context, dev *model.Dev, username string, c kubernetes.Interface) (*model.K8sObject, error) {
	r := model.NewResource(dev)
	if r.ObjectType == model.DeploymentObjectType {
		d, err := deployments.Get(ctx, dev, dev.Namespace, c)
		if err != nil {
			return nil, fmt.Errorf("error diverting deployment: %s", err.Error())
		}

		divertDeployment := translateDeployment(username, d)
		if err := deployments.Deploy(ctx, divertDeployment, c); err != nil {
			return nil, fmt.Errorf("error creating diver deployment '%s': %s", divertDeployment.Name, err.Error())
		}
		r.Deployment = divertDeployment
		return r, nil
	} else {
		sfs, err := statefulsets.Get(ctx, dev, dev.Namespace, c)
		if err != nil {
			return nil, fmt.Errorf("error diverting deployment: %s", err.Error())
		}

		divertStatefulset := translateStatefulset(username, sfs)
		if err := statefulsets.Deploy(ctx, divertStatefulset, c); err != nil {
			return nil, fmt.Errorf("error creating diver deployment '%s': %s", divertStatefulset.Name, err.Error())
		}
		r.StatefulSet = divertStatefulset
		return r, nil
	}
}

func divertService(ctx context.Context, dev *model.Dev, r *model.K8sObject, username string, c kubernetes.Interface) (*apiv1.Service, error) {
	s, err := services.Get(ctx, dev.Divert.Service, dev.Namespace, c)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("the divert service '%s' doesn't exist", dev.Divert.Service)
		}
		return nil, fmt.Errorf("error getting divert service '%s': %s", dev.Divert.Service, err.Error())
	}

	divertService, err := translateService(username, r, s)
	if err != nil {
		return nil, err
	}
	if err := services.Deploy(ctx, divertService, c); err != nil {
		return nil, fmt.Errorf("error creating divert service '%s': %s", divertService.Name, err.Error())
	}
	return divertService, nil
}

func divertIngress(ctx context.Context, dev *model.Dev, username string, c kubernetes.Interface) (*networkingv1.Ingress, error) {
	i, err := ingressesv1.Get(ctx, dev.Divert.Ingress, dev.Namespace, c)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("the divert ingress '%s' doesn't exist", dev.Divert.Ingress)
		}
		return nil, fmt.Errorf("error getting divert ingress '%s': %s", dev.Divert.Ingress, err.Error())
	}

	divertIngress := translateIngress(username, i)
	if err := ingressesv1.Deploy(ctx, divertIngress, c); err != nil {
		return nil, fmt.Errorf("error creating divert ingress '%s': %s", divertIngress.Name, err.Error())
	}
	return divertIngress, nil
}

func createDivertCRD(ctx context.Context, dev *model.Dev, username string, i *networkingv1.Ingress, s *apiv1.Service, c kubernetes.Interface) error {
	dClient, err := GetClient(dev.Context)
	if err != nil {
		return fmt.Errorf("error creating divert CRD client: %s", err.Error())
	}

	divertCRD := translateDivertCRD(username, dev, s, i)

	old, err := dClient.Diverts(divertCRD.Namespace).Get(ctx, divertCRD.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting divert CRD '%s'': %s", divertCRD.Name, err)
	}

	if old.Name == "" {
		log.Infof("creating  divert CRD '%s'", divertCRD.Name)
		_, err = dClient.Diverts(divertCRD.Namespace).Create(ctx, divertCRD)
		if err != nil {
			return fmt.Errorf("error creating divert CRD '%s': %s", divertCRD.Name, err)
		}
		log.Infof("created divert CRD '%s'", divertCRD.Name)
	} else {
		log.Infof("updating divert CRD '%s'", divertCRD.Name)
		old.TypeMeta = divertCRD.TypeMeta
		old.Annotations = divertCRD.Annotations
		old.Labels = divertCRD.Labels
		old.Spec = divertCRD.Spec
		old.Status = DivertStatus{}
		_, err = dClient.Diverts(divertCRD.Namespace).Update(ctx, old)
		if err != nil {
			return fmt.Errorf("error updating divert CRD '%s': %s", divertCRD.Name, err)
		}
		log.Infof("updated divert CRD '%s'.", divertCRD.Name)
	}

	return nil
}

func Delete(ctx context.Context, dev *model.Dev, c kubernetes.Interface) error {
	username := okteto.GetSanitizedUsername()

	k8sObject, err := apps.GetResource(ctx, dev, dev.Namespace, c)
	if err != nil {
		k8sObject.ObjectMeta = metav1.ObjectMeta{
			Name:      dev.Name,
			Namespace: dev.Name,
		}
	}

	dClient, err := GetClient(dev.Context)
	if err != nil {
		return fmt.Errorf("error creating divert CRD client: %s", err.Error())
	}
	divertCRDName := DivertName(username, dev.Divert.Service)
	if err := dClient.Diverts(dev.Namespace).Delete(ctx, divertCRDName, metav1.DeleteOptions{}); err != nil {
		if strings.Contains(err.Error(), "the server could not find the requested resource") {
			return errors.ErrDivertNotSupported
		}
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error deleting divert CRD '%s': %s", divertCRDName, err.Error())
		}
	}

	iName := DivertName(username, dev.Divert.Ingress)
	if err := ingressesv1.Destroy(ctx, iName, dev.Namespace, c); err != nil {
		return fmt.Errorf("error deleting divert ingress '%s': %s", iName, err.Error())
	}

	sName := DivertName(username, dev.Divert.Service)
	if err := services.Destroy(ctx, sName, dev.Namespace, c); err != nil {
		return fmt.Errorf("error deleting divert service '%s': %s", sName, err.Error())
	}

	if k8sObject.ObjectType == model.StatefulsetObjectType {
		divertStatefulset := translateStatefulset(username, k8sObject.StatefulSet)
		k8sObject.UpdateStatefulset(divertStatefulset)
		translateDev(username, dev, k8sObject)
	} else {
		divertDeployment := translateDeployment(username, k8sObject.Deployment)
		k8sObject.UpdateDeployment(divertDeployment)
		translateDev(username, dev, k8sObject)
	}
	return nil
}
