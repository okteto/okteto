package namespaces

import (
	"github.com/okteto/okteto/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

const (
	//OktetoLabel represents the owner of the namespace
	OktetoLabel = "dev.okteto.com"
)

//IsOktetoNamespace checks if this is a namespace created by okteto
func IsOktetoNamespace(ns string, c *kubernetes.Clientset) bool {
	n, err := c.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
	if err != nil {
		log.Infof("error accessing namespace %s: %s", ns, err)
		return false
	}
	return n.Labels[OktetoLabel] == "true"
}
