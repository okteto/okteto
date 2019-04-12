package roles

import (
	"fmt"
	"strings"

	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create creates the role for a given space
func Create(s *model.Space, c *kubernetes.Clientset) error {
	log.Debugf("Creating role '%s'...", s.Name)
	r, err := c.RbacV1().Roles(s.Name).Get(s.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes role: %s", err)
	}
	if r.Name != "" {
		log.Debugf("Role '%s' was already created", s.Name)
		return nil
	}
	r = translate(s)
	_, err = c.RbacV1().Roles(s.Name).Create(r)
	if err != nil {
		return fmt.Errorf("Error creating kubernetes role: %s", err)
	}
	log.Debugf("Created role '%s'.", s.Name)
	return nil
}
