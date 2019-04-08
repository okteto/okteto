package namespace

import (
	"fmt"
	"strings"

	"log"

	"github.com/okteto/app/model"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create creates a namespace for a given space
func Create(s *model.Space, c *kubernetes.Clientset) error {
	log.Infof("Creating namespace '%s'...", s.Name)
	n, err := c.Core().Namespaces().Get(s.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes namespace: %s", err)
	}
	if n.Name == "" {
		n := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: e.Name}}
		_, err := c.Core().Namespaces().Create(n)
		if err != nil {
			return fmt.Errorf("Error creating kubernetes namespace: %s", err)
		}
		log.Infof("Created namespace '%s'.", e.Name)
	} else {
		log.Infof("Namespace '%s' was already created.", e.Name)
	}
	return nil
}
