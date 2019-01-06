package deployments

import (
	"fmt"

	"github.com/okteto/cnd/pkg/model"
	log "github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

// GetFullName returns the full name of the deployment. This is mostly used for logs and labels
func GetFullName(namespace, deploymentName string) string {
	return fmt.Sprintf("%s/%s", namespace, deploymentName)
}

func loadDeployment(namespace, deploymentName string, c *kubernetes.Clientset) (*appsv1.Deployment, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	d, err := c.AppsV1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Debugf("error while retrieving the deployment: %s", err)
	}

	return d, err
}

func isContainerInPod(pod *apiv1.Pod, container string) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name == container {
			return true
		}
	}

	return false
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
