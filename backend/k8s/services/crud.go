package services

import (
	"fmt"
	"strings"

	"github.com/okteto/app/backend/log"
	"github.com/okteto/app/backend/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Deploy deploys a k8s service
func Deploy(dev *model.Dev, s *model.Space, c *kubernetes.Clientset) error {
	sClient := c.CoreV1().Services(s.Name)
	newService := translate(dev, s)

	currentService, err := sClient.Get(dev.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error getting kubernetes service: %s", err)
	}

	if currentService.Name == "" {
		log.Infof("creating service '%s'...", dev.Name)
		_, err = sClient.Create(newService)
		if err != nil {
			return fmt.Errorf("error creating kubernetes service: %s", err)
		}
		log.Infof("created service '%s'.", dev.Name)
	} else {
		log.Infof("updating service '%s'...", dev.Name)
		currentService.Spec.Ports = newService.Spec.Ports
		_, err = sClient.Update(currentService)
		if err != nil {
			return fmt.Errorf("error updating kubernetes service: %s", err)
		}
		log.Infof("updated service '%s'.", dev.Name)
	}
	return nil
}

//Destroy destroys a k8s service
func Destroy(dev *model.Dev, s *model.Space, c *kubernetes.Clientset) error {
	log.Infof("deleting service '%s'...", dev.Name)
	sClient := c.CoreV1().Services(s.Name)
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
