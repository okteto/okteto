package rolebindings

import (
	"fmt"
	"strings"

	"github.com/okteto/app/backend/log"
	"github.com/okteto/app/backend/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create creates the role binding for a given space
func Create(s *model.Space, c *kubernetes.Clientset) error {
	log.Debugf("Creating role binding '%s'...", s.Name)
	rb, err := c.RbacV1().RoleBindings(s.Name).Get(s.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes role binding: %s", err)
	}
	if rb.Name != "" {
		log.Debugf("Role binding '%s' was already created", s.Name)
		return nil
	}
	rb = translate(s)
	_, err = c.RbacV1().RoleBindings(s.Name).Create(rb)
	if err != nil {
		return fmt.Errorf("Error creating kubernetes role binding: %s", err)
	}
	log.Debugf("Created role binding '%s'.", s.Name)
	return nil
}
