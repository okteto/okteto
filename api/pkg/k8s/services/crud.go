package services

import (
	"fmt"
	"strings"

	"github.com/okteto/app/api/pkg/log"
	"github.com/okteto/app/api/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Deploy deploys a k8s service
func Deploy(new *apiv1.Service, s *model.Space, c *kubernetes.Clientset) error {
	sClient := c.CoreV1().Services(s.ID)
	old, err := sClient.Get(new.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error getting kubernetes service: %s", err)
	}
	if old.Name == "" {
		log.Infof("creating service '%s'...", new.Name)
		_, err = sClient.Create(new)
		if err != nil {
			return fmt.Errorf("error creating kubernetes service: %s", err)
		}
		log.Infof("created service '%s'.", new.Name)
	} else {
		log.Infof("updating service '%s'...", new.Name)
		old.Spec.Ports = new.Spec.Ports
		_, err = sClient.Update(old)
		if err != nil {
			return fmt.Errorf("error updating kubernetes service: %s", err)
		}
		log.Infof("updated service '%s'.", new.Name)
	}
	return nil
}

//Destroy destroys a k8s service
func Destroy(name string, s *model.Space, c *kubernetes.Clientset) error {
	log.Infof("deleting service '%s'...", name)
	sClient := c.CoreV1().Services(s.ID)
	err := sClient.Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Infof("service '%s' was already deleted.", name)
			return nil
		}
		return fmt.Errorf("error deleting kubernetes service: %s", err)
	}
	log.Infof("service '%s' deleted", name)
	return nil
}
