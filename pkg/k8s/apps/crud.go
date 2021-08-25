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

package apps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/annotations"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/model"
	"k8s.io/client-go/kubernetes"
)

func GetResource(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) (*model.K8sObject, error) {
	var err error
	k8sObject := model.NewResource(dev)
	if k8sObject.ObjectType == "" {
		k8sObject.ObjectType = GetResourceFromServiceName(ctx, dev.Name, dev.Namespace, c)
	}
	switch k8sObject.ObjectType {
	case model.DeploymentObjectType:
		k8sObject.Deployment, err = deployments.Get(ctx, dev, namespace, c)
		if err != nil {
			return k8sObject, err
		}
		k8sObject.Name = k8sObject.Deployment.Name
		k8sObject.ObjectMeta = k8sObject.Deployment.ObjectMeta
		k8sObject.Replicas = k8sObject.Deployment.Spec.Replicas
		k8sObject.Selector = k8sObject.Deployment.Spec.Selector
		k8sObject.PodTemplateSpec = &k8sObject.Deployment.Spec.Template
		return k8sObject, nil
	case model.StatefulsetObjectType:
		k8sObject.StatefulSet, err = statefulsets.Get(ctx, dev, namespace, c)
		if err != nil {
			return k8sObject, err
		}
		k8sObject.Name = k8sObject.StatefulSet.Name
		k8sObject.ObjectMeta = k8sObject.StatefulSet.ObjectMeta
		k8sObject.Replicas = k8sObject.StatefulSet.Spec.Replicas
		k8sObject.Selector = k8sObject.StatefulSet.Spec.Selector
		k8sObject.PodTemplateSpec = &k8sObject.StatefulSet.Spec.Template
		return k8sObject, nil
	}

	return nil, fmt.Errorf("Could not retrieve '%s' resource.", dev.Name)
}

func IsDevModeOn(r *model.K8sObject) bool {
	switch r.ObjectType {
	case model.DeploymentObjectType:
		return deployments.IsDevModeOn(r.Deployment)
	case model.StatefulsetObjectType:
		return statefulsets.IsDevModeOn(r.StatefulSet)
	default:
		return false
	}
}

func HasBeenChanged(k8sObject *model.K8sObject) bool {
	switch k8sObject.ObjectType {
	case model.DeploymentObjectType:
		return deployments.HasBeenChanged(k8sObject.Deployment)
	case model.StatefulsetObjectType:
		return statefulsets.HasBeenChanged(k8sObject.StatefulSet)
	default:
		return false
	}
}

func ValidateMountPaths(k8sObject *model.K8sObject, dev *model.Dev) error {
	if !dev.PersistentVolumeInfo.Enabled {
		return nil
	}
	devContainer := GetDevContainer(&k8sObject.PodTemplateSpec.Spec, dev.Container)

	for _, vm := range devContainer.VolumeMounts {
		if dev.GetVolumeName() == vm.Name {
			continue
		}
		for _, syncVolume := range dev.Sync.Folders {
			if vm.MountPath == syncVolume.RemotePath {
				return errors.UserError{
					E:    fmt.Errorf("'%s' is already defined as volume in %v %s", vm.MountPath, k8sObject.ObjectType, k8sObject.Name),
					Hint: `Disable the okteto persistent volume (https://okteto.com/docs/reference/manifest/#persistentvolume-object-optional) and try again`}
			}
		}
	}
	return nil
}

//GetTranslations fills all the deployments pointed by a development container
func GetTranslations(ctx context.Context, dev *model.Dev, k8sObject *model.K8sObject, reset bool, c kubernetes.Interface) (map[string]*model.Translation, error) {
	result := map[string]*model.Translation{}
	if k8sObject.Deployment != nil || k8sObject.StatefulSet != nil {
		var replicas int32
		var k8sObjectStrategy model.K8sObjectStrategy
		trRulesJSON := annotations.Get(k8sObject.PodTemplateSpec.GetObjectMeta(), model.TranslationAnnotation)
		if trRulesJSON != "" {
			trRules := &model.Translation{}
			if err := json.Unmarshal([]byte(trRulesJSON), trRules); err != nil {
				return nil, fmt.Errorf("malformed tr rules: %s", err)
			}
			replicas = trRules.Replicas
			k8sObjectStrategy.SetStrategy(trRules.Strategy)

		} else {
			replicas = getPreviousK8sObjectReplicas(k8sObject)
			k8sObjectStrategy.SetStrategyFromResource(k8sObject)
		}

		rule := dev.ToTranslationRule(dev, reset)
		result[k8sObject.Name] = &model.Translation{
			Interactive: true,
			Name:        dev.Name,
			Version:     model.TranslationVersion,
			K8sObject:   k8sObject,
			Annotations: dev.Annotations,
			Tolerations: dev.Tolerations,
			Replicas:    replicas,
			Strategy:    k8sObjectStrategy,
			Rules:       []*model.TranslationRule{rule},
		}
	}

	if err := loadServiceTranslations(ctx, dev, reset, result, c); err != nil {
		return nil, err
	}

	return result, nil
}

func loadServiceTranslations(ctx context.Context, dev *model.Dev, reset bool, result map[string]*model.Translation, c kubernetes.Interface) error {
	for _, s := range dev.Services {
		k8sObject, err := GetResource(ctx, s, dev.Namespace, c)
		if err != nil {
			return err
		}

		rule := s.ToTranslationRule(dev, reset)

		if _, ok := result[k8sObject.Name]; ok {
			result[k8sObject.Name].Rules = append(result[k8sObject.Name].Rules, rule)
			continue
		}

		result[k8sObject.Name] = &model.Translation{
			Name:        dev.Name,
			Interactive: false,
			Version:     model.TranslationVersion,
			K8sObject:   k8sObject,
			Annotations: dev.Annotations,
			Tolerations: dev.Tolerations,
			Replicas:    *k8sObject.Replicas,
			Rules:       []*model.TranslationRule{rule},
		}

	}

	return nil
}

//TranslateDevMode translates the deployment manifests to put them in dev mode
func TranslateDevMode(tr map[string]*model.Translation, c *kubernetes.Clientset, isOktetoNamespace bool) error {
	for _, t := range tr {
		err := translate(t, c, isOktetoNamespace)
		if err != nil {
			return err
		}
	}
	return nil
}

func SetLastBuiltAnnotation(r *model.K8sObject) {
	switch r.ObjectType {
	case model.DeploymentObjectType:
		deployments.SetLastBuiltAnnotation(r.Deployment)
	case model.StatefulsetObjectType:
		statefulsets.SetLastBuiltAnnotation(r.StatefulSet)
	}
}

func Deploy(ctx context.Context, r *model.K8sObject, forceCreate bool, client *kubernetes.Clientset) error {
	switch r.ObjectType {
	case model.DeploymentObjectType:
		return deployments.Deploy(ctx, r.Deployment, client)
	case model.StatefulsetObjectType:
		return statefulsets.Deploy(ctx, r.StatefulSet, client)
	}
	return nil
}

func Create(ctx context.Context, r *model.K8sObject, client *kubernetes.Clientset) error {
	switch r.ObjectType {
	case model.DeploymentObjectType:
		return deployments.Create(ctx, r.Deployment, client)
	case model.StatefulsetObjectType:
		return statefulsets.Create(ctx, r.StatefulSet, client)
	}
	return nil
}

func DestroyDev(ctx context.Context, r *model.K8sObject, dev *model.Dev, c kubernetes.Interface) error {
	switch r.ObjectType {
	case model.DeploymentObjectType:
		return deployments.DestroyDev(ctx, dev, c)
	case model.StatefulsetObjectType:
		return statefulsets.DestroyDev(ctx, dev, c)
	}
	return nil
}

func Get(ctx context.Context, r *model.K8sObject, dev *model.Dev, client *kubernetes.Clientset) error {
	switch r.ObjectType {
	case model.DeploymentObjectType:
		_, err := deployments.Get(ctx, dev, dev.Namespace, client)
		return err
	case model.StatefulsetObjectType:
		_, err := statefulsets.Get(ctx, dev, dev.Namespace, client)
		return err
	}
	return nil
}

func Update(ctx context.Context, r *model.K8sObject, client *kubernetes.Clientset) error {
	switch r.ObjectType {
	case model.DeploymentObjectType:
		return deployments.Update(ctx, r.Deployment, client)
	case model.StatefulsetObjectType:
		return statefulsets.Update(ctx, r.StatefulSet, client)
	}
	return nil
}

// UpdateK8sObjects update all resources in the given translation list
func UpdateK8sObjects(ctx context.Context, trList map[string]*model.Translation, c kubernetes.Interface) error {
	for _, tr := range trList {
		switch tr.K8sObject.ObjectType {
		case model.DeploymentObjectType:
			if tr.K8sObject.Deployment == nil {
				continue
			}
			if err := deployments.Update(ctx, tr.K8sObject.Deployment, c); err != nil {
				return err
			}
		case model.StatefulsetObjectType:
			if tr.K8sObject.StatefulSet == nil {
				continue
			}
			if err := statefulsets.Update(ctx, tr.K8sObject.StatefulSet, c); err != nil {
				return err
			}
		}

	}
	return nil
}

func GetResourceFromServiceName(ctx context.Context, resourceName, namespace string, c kubernetes.Interface) model.ObjectType {
	d, err := deployments.GetDeploymentByName(ctx, resourceName, namespace, c)
	if err == nil && d != nil {
		return model.DeploymentObjectType
	}
	sfs, err := statefulsets.GetStatefulsetByName(ctx, resourceName, namespace, c)
	if err == nil && sfs != nil {
		return model.StatefulsetObjectType
	}
	return model.DeploymentObjectType
}

func GetRevisionAnnotatedK8sObjectOrFailed(ctx context.Context, dev *model.Dev, c kubernetes.Interface, waitUntilDeployed bool) (*model.K8sObject, error) {

	objectType := GetResourceFromServiceName(ctx, dev.Name, dev.Namespace, c)
	k8sObject := model.NewResource(dev)
	k8sObject.ObjectType = objectType
	switch objectType {
	case model.DeploymentObjectType:
		d, err := deployments.GetRevisionAnnotatedDeploymentOrFailed(ctx, dev, c, waitUntilDeployed)
		if d == nil {
			return nil, err
		}
		k8sObject.UpdateDeployment(d)
		return k8sObject, nil
	case model.StatefulsetObjectType:
		sfs, err := statefulsets.GetRevisionAnnotatedStatefulsetOrFailed(ctx, dev, c, waitUntilDeployed)
		if sfs == nil {
			return nil, err
		}
		k8sObject.UpdateStatefulset(sfs)
		return k8sObject, nil
	}
	return nil, fmt.Errorf("kubernetes object not found")
}
