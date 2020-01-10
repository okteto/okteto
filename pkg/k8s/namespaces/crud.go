package namespaces

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	// OktetoNotAllowedLabel tells Okteto to not allow operations on the namespace
	OktetoNotAllowedLabel = "dev.okteto.com/not-allowed"
)

//IsOktetoNamespace checks if this is a namespace created by okteto
func IsOktetoNamespace(ns *apiv1.Namespace) bool {
	return ns.Labels[okLabels.DevLabel] == "true"
}

//IsOktetoAllowed checks if Okteto operationos are allowed in this namespace
func IsOktetoAllowed(ns *apiv1.Namespace) bool {
	if _, ok := ns.Labels[OktetoNotAllowedLabel]; ok {
		return false
	}

	return true
}

// Get returns the namespace object of ns
func Get(ns string, c *kubernetes.Clientset) (*apiv1.Namespace, error) {
	n, err := c.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return n, nil
}
