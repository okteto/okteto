package deployments

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	deploymentAnnotation = "dev.okteto.com/deployment"
	oktetoSecretTemplate = "okteto-%s"
)

func getAnnotation(o metav1.Object, key string) string {
	annotations := o.GetAnnotations()
	if annotations != nil {
		return annotations[key]
	}
	return ""
}
