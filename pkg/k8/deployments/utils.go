package deployments

import (
	"encoding/json"
	"fmt"

	"github.com/okteto/cnd/pkg/model"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetFullName returns the full name of the deployment. This is mostly used for logs and labels
func GetFullName(namespace, deploymentName string) string {
	return fmt.Sprintf("%s/%s", namespace, deploymentName)
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

func GetDevFromAnnotation(d *appsv1.Deployment) (*model.Dev, error) {
	dev := &model.Dev{}
	annotations := d.GetObjectMeta().GetAnnotations()
	if annotations == nil {
		return nil, fmt.Errorf("the deployment '%s' is not a cloud native environment", d.Name)
	}
	for k, v := range annotations {
		if k == model.CNDDevAnnotation {
			if err := json.Unmarshal([]byte(v), dev); err != nil {
				return nil, err
			}
			return dev, nil
		}
	}
	return nil, fmt.Errorf("the deployment '%s' is not a cloud native environment", d.Name)
}

func setDevAsAnnotation(d *appsv1.Deployment, dev *model.Dev) error {
	devBytes, err := json.Marshal(dev)
	if err != nil {
		return err
	}
	setAnnotation(d.GetObjectMeta(), model.CNDDevAnnotation, string(devBytes))
	return nil
}

func getDevContainerOrFirst(container string, containers []apiv1.Container) string {
	if container == "" {
		for _, c := range containers {
			if c.Name != model.CNDSyncContainerName {
				container = c.Name
			}
		}
	}

	return container
}
