package rolebindings

import (
	"fmt"
	"strings"

	"github.com/okteto/app/api/pkg/log"
	"github.com/okteto/app/api/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create creates the role binding for a given space
func Create(s *model.Space, c *kubernetes.Clientset) error {
	if err := deleteOldRoleBindings(s, c); err != nil {
		return err
	}
	for _, m := range s.Members {
		log.Debugf("Creating role binding '%s'...", m.ID)
		old, err := c.RbacV1().RoleBindings(s.ID).Get(m.ID, metav1.GetOptions{})
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("Error getting kubernetes role binding: %s", err)
		}
		rb := translate(&m, s)
		if old.Name == "" {
			_, err = c.RbacV1().RoleBindings(s.ID).Create(rb)
			if err != nil {
				return fmt.Errorf("Error creating kubernetes role binding: %s", err)
			}
			log.Debugf("Created role binding '%s'.", m.ID)
		} else {
			_, err = c.RbacV1().RoleBindings(s.ID).Update(rb)
			if err != nil {
				return fmt.Errorf("Error updating kubernetes role binding: %s", err)
			}
			log.Debugf("Updated role binding '%s'.", m.ID)
		}
	}
	return nil
}

func deleteOldRoleBindings(s *model.Space, c *kubernetes.Clientset) error {
	newMembers := map[string]bool{}
	for _, m := range s.Members {
		newMembers[m.ID] = true
	}
	rbs, err := c.RbacV1().RoleBindings(s.ID).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, rb := range rbs.Items {
		if _, ok := newMembers[rb.Name]; !ok {
			err := c.RbacV1().RoleBindings(s.ID).Delete(rb.Name, &metav1.DeleteOptions{})
			if err != nil && !strings.Contains(err.Error(), "not found") {
				return err
			}
		}
	}
	return nil
}
