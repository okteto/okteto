package deployments

import (
	"encoding/json"
	"fmt"

	"github.com/cloudnativedevelopment/cnd/pkg/model"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetFullName returns the full name of the deployment. This is mostly used for logs and labels
func GetFullName(namespace, deploymentName string) string {
	return fmt.Sprintf("%s/%s", namespace, deploymentName)
}

// IsDevModeEnabled returns true if the deployment has dev annotations
func IsDevModeEnabled(o metav1.Object) (bool, error) {
	l, err := getDevListFromAnnotation(o)
	if err != nil {
		return false, err
	}

	return (len(l) > 0), nil
}

// GetAndUpdateDevListFromAnnotation returns the active cloud dev environments from the deployment annotations
func GetAndUpdateDevListFromAnnotation(o metav1.Object, dev *model.Dev) ([]*model.Dev, error) {
	devList, err := getDevListFromAnnotation(o)
	if err != nil {
		return nil, err
	}
	for i, v := range devList {
		if dev.Swap.Deployment.Container == v.Swap.Deployment.Container {
			devList[i] = dev
			return devList, nil
		}
	}
	devList = append(devList, dev)
	return devList, nil
}

func getDevListFromAnnotation(o metav1.Object) ([]*model.Dev, error) {
	devList := []*model.Dev{}
	devListAnnotation := getAnnotation(o, model.CNDDevListAnnotation)
	if devListAnnotation == "" {
		return devList, nil
	}
	if err := json.Unmarshal([]byte(devListAnnotation), &devList); err != nil {
		return nil, err
	}
	return devList, nil
}

func setDevListAsAnnotation(o metav1.Object, devList []*model.Dev) error {
	devListBytes, err := json.Marshal(devList)
	if err != nil {
		return err
	}
	setAnnotation(o, model.CNDDevListAnnotation, string(devListBytes))
	return nil
}

// GetDevContainerOrFirst returns the first container if container is empty
func GetDevContainerOrFirst(container string, containers []apiv1.Container) string {
	if container == "" {
		for _, c := range containers {
			return c.Name
		}
	}
	return container
}

func getLabel(o metav1.Object, key string) string {
	labels := o.GetLabels()
	if labels != nil {
		return labels[key]
	}
	return ""
}

func setLabel(o metav1.Object, key, value string) {
	labels := o.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[key] = value
	o.SetLabels(labels)
}

func getAnnotation(o metav1.Object, key string) string {
	annotations := o.GetAnnotations()
	if annotations != nil {
		return annotations[key]
	}
	return ""
}

func setAnnotation(o metav1.Object, key, value string) {
	annotations := o.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[key] = value
	o.SetAnnotations(annotations)
}
