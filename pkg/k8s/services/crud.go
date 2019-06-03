package services

import (
	"fmt"
	"strings"

	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create deploys a k8s service
func Create(dev *model.Dev, c *kubernetes.Clientset) error {
	s := translate(dev)
	sClient := c.CoreV1().Services(dev.Namespace)
	old, err := sClient.Get(dev.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error getting kubernetes service: %s", err)
	}
	if old.Name == "" {
		log.Infof("creating service '%s'...", s.Name)
		_, err = sClient.Create(s)
		if err != nil {
			return fmt.Errorf("error creating kubernetes service: %s", err)
		}
		log.Infof("created service '%s'.", s.Name)
	} else {
		log.Infof("updating service '%s'...", s.Name)
		old.Spec.Ports = s.Spec.Ports
		_, err = sClient.Update(old)
		if err != nil {
			return fmt.Errorf("error updating kubernetes service: %s", err)
		}
		log.Infof("updated service '%s'.", s.Name)
	}
	return nil
}

//Destroy destroys a k8s service
func Destroy(dev *model.Dev, c *kubernetes.Clientset) error {
	log.Infof("deleting service '%s'...", dev.Name)
	sClient := c.CoreV1().Services(dev.Namespace)
	err := sClient.Delete(dev.Name, &metav1.DeleteOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Infof("service '%s' was already deleted.", dev.Name)
			return nil
		}
		return fmt.Errorf("error deleting kubernetes service: %s", err)
	}
	log.Infof("service '%s' deleted", dev.Name)
	return nil
}
