package model

import (
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type K8sObject struct {
	Name       string
	Namespace  string
	ObjectType ObjectType

	Image           string
	ObjectMeta      metav1.ObjectMeta
	Replicas        *int32
	Selector        *metav1.LabelSelector
	PodTemplateSpec *apiv1.PodTemplateSpec

	Deployment  *appsv1.Deployment
	StatefulSet *appsv1.StatefulSet
}

type K8sObjectStrategy struct {
	DeploymentStrategy  appsv1.DeploymentStrategy
	StatefulsetStrategy appsv1.StatefulSetUpdateStrategy
}

func NewResource(dev *Dev) *K8sObject {
	r := &K8sObject{}

	r.ObjectType = dev.ObjectType
	r.ObjectMeta = metav1.ObjectMeta{
		Name:      dev.Name,
		Namespace: dev.Namespace,
		Annotations: map[string]string{
			OktetoAutoCreateAnnotation: OktetoUpCmd,
		},
	}

	image := dev.Image.Name
	if image == "" {
		image = DefaultImage
	}
	r.Image = image
	r.Replicas = &DevReplicas
	r.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": dev.Name,
		},
	}

	r.PodTemplateSpec = &apiv1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": dev.Name,
			},
		},
		Spec: apiv1.PodSpec{
			ServiceAccountName: dev.ServiceAccount,
			Containers: []apiv1.Container{
				{
					Name:            "dev",
					Image:           image,
					ImagePullPolicy: apiv1.PullAlways,
				},
			},
		},
	}
	return r
}

func (r *K8sObject) GetSandbox() {
	if r.ObjectType == "" {
		r.ObjectType = DeploymentObjectType
	}
	switch r.ObjectType {
	case DeploymentObjectType:
		r.Deployment = &appsv1.Deployment{ObjectMeta: r.ObjectMeta,
			Spec: appsv1.DeploymentSpec{Replicas: r.Replicas,
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RecreateDeploymentStrategyType,
				},
				Selector: r.Selector,
				Template: *r.PodTemplateSpec,
			},
		}
		r.PodTemplateSpec = &r.Deployment.Spec.Template
	case StatefulsetObjectType:
		r.StatefulSet = &appsv1.StatefulSet{ObjectMeta: r.ObjectMeta,
			Spec: appsv1.StatefulSetSpec{Replicas: r.Replicas,
				UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
					Type: appsv1.RollingUpdateStatefulSetStrategyType,
				},
				Selector: r.Selector,
				Template: *r.PodTemplateSpec,
			},
		}
		r.PodTemplateSpec = &r.StatefulSet.Spec.Template
	}
}

func (r *K8sObject) GetObjectMeta() metav1.Object {
	switch r.ObjectType {
	case DeploymentObjectType:
		return r.Deployment.GetObjectMeta()
	case StatefulsetObjectType:
		return r.StatefulSet.GetObjectMeta()
	}
	return nil
}

func (r *K8sObject) GetPodTemplate() *apiv1.PodTemplateSpec {
	switch r.ObjectType {
	case DeploymentObjectType:
		return &r.Deployment.Spec.Template
	case StatefulsetObjectType:
		return &r.StatefulSet.Spec.Template
	}
	return nil
}

func (r *K8sObject) UpdateObjectMeta() {
	switch r.ObjectType {
	case DeploymentObjectType:
		r.Deployment.ObjectMeta = r.ObjectMeta
		r.Deployment.Spec.Template = *r.PodTemplateSpec
	case StatefulsetObjectType:
		r.StatefulSet.ObjectMeta = r.ObjectMeta
		r.StatefulSet.Spec.Template = *r.PodTemplateSpec
	}
}

func (r *K8sObject) SetAnnotations(annotations map[string]string) {
	switch r.ObjectType {
	case DeploymentObjectType:
		r.Deployment.GetObjectMeta().SetAnnotations(annotations)
	case StatefulsetObjectType:
		r.StatefulSet.GetObjectMeta().SetAnnotations(annotations)
	}
	r.ObjectMeta.SetAnnotations(annotations)
}

func (r *K8sObject) SetPodTemplateAnnotations(annotations map[string]string) {
	switch r.ObjectType {
	case DeploymentObjectType:
		r.Deployment.Spec.Template.GetObjectMeta().SetAnnotations(annotations)
	case StatefulsetObjectType:
		r.StatefulSet.Spec.Template.GetObjectMeta().SetAnnotations(annotations)
	}
	r.PodTemplateSpec.GetObjectMeta().SetAnnotations(annotations)
}

func (r *K8sObject) Unmarshal(manifest []byte) error {
	switch r.ObjectType {
	case DeploymentObjectType:
		dOrig := &appsv1.Deployment{}
		if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
			return err
		}
		r.Deployment = dOrig
	case StatefulsetObjectType:
		dOrig := &appsv1.StatefulSet{}
		if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
			return err
		}
		r.StatefulSet = dOrig
	}
	return nil
}

func (r *K8sObject) SetStatus(resource *K8sObject) ([]byte, error) {
	switch r.ObjectType {
	case DeploymentObjectType:
		r.Deployment.Status = appsv1.DeploymentStatus{}
		manifestBytes, err := json.Marshal(r.Deployment)
		if err != nil {
			return nil, err
		}
		return manifestBytes, nil
	case StatefulsetObjectType:
		r.StatefulSet.Status = appsv1.StatefulSetStatus{}
		manifestBytes, err := json.Marshal(r.StatefulSet)
		if err != nil {
			return nil, err
		}
		return manifestBytes, nil
	}
	return nil, nil
}

func (r *K8sObject) SetReplicas(replicas *int32) {
	r.Replicas = replicas
	switch r.ObjectType {
	case DeploymentObjectType:
		r.Deployment.Spec.Replicas = replicas
	case StatefulsetObjectType:
		r.StatefulSet.Spec.Replicas = replicas
	}
}

func (r *K8sObject) GetAnnotation(key string) string {
	switch r.ObjectType {
	case DeploymentObjectType:
		return r.Deployment.Annotations[key]
	case StatefulsetObjectType:
		return r.StatefulSet.Annotations[key]
	}
	return ""
}

func (r *K8sObject) GetLabel(key string) string {
	switch r.ObjectType {
	case DeploymentObjectType:
		return r.Deployment.Annotations[key]
	case StatefulsetObjectType:
		return r.StatefulSet.Annotations[key]
	}
	return ""
}
func (r *K8sObject) SetAnnotation(key, value string) {
	switch r.ObjectType {
	case DeploymentObjectType:
		r.Deployment.Annotations[key] = value
	case StatefulsetObjectType:
		r.StatefulSet.Annotations[key] = value
	}
}

func (r *K8sObject) GetReplicas() *int32 {
	switch r.ObjectType {
	case DeploymentObjectType:
		return r.Deployment.Spec.Replicas
	case StatefulsetObjectType:
		return r.StatefulSet.Spec.Replicas
	}
	return nil
}

func (r *K8sObject) UpdateDeployment(d *appsv1.Deployment) {
	r.Deployment = d
	r.Name = d.Name
	r.Namespace = d.Namespace
	r.ObjectMeta = d.ObjectMeta
	r.PodTemplateSpec = &d.Spec.Template
	r.Replicas = d.Spec.Replicas
	r.Selector = d.Spec.Selector
}

func (r *K8sObject) UpdateStrategy(strategy K8sObjectStrategy) {
	switch r.ObjectType {
	case DeploymentObjectType:
		r.Deployment.Spec.Strategy = strategy.DeploymentStrategy
	case StatefulsetObjectType:
		r.StatefulSet.Spec.UpdateStrategy = strategy.StatefulsetStrategy
	}
}

func (r *K8sObject) UpdateStatefulset(sfs *appsv1.StatefulSet) {
	r.StatefulSet = sfs
	r.Name = sfs.Name
	r.Namespace = sfs.Namespace
	r.ObjectMeta = sfs.ObjectMeta
	r.PodTemplateSpec = &sfs.Spec.Template
	r.Replicas = sfs.Spec.Replicas
	r.Selector = sfs.Spec.Selector
}

func (s K8sObjectStrategy) SetStrategy(otherStrategy K8sObjectStrategy) {
	s = otherStrategy
}

func (s K8sObjectStrategy) SetStrategyFromResource(resource *K8sObject) {
	switch resource.ObjectType {
	case DeploymentObjectType:
		s.DeploymentStrategy = resource.Deployment.Spec.Strategy
	case StatefulsetObjectType:
		s.StatefulsetStrategy = resource.StatefulSet.Spec.UpdateStrategy
	}
}
