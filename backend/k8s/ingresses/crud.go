package ingresses

import (
	"fmt"
	"strings"

	"github.com/okteto/app/backend/log"
	"github.com/okteto/app/backend/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Deploy deploys a k8s ingress
func Deploy(dev *model.Dev, s *model.Space, c *kubernetes.Clientset) error {
	iClient := c.ExtensionsV1beta1().Ingresses(s.Name)
	newIngress := translate(dev, s)
	currentIngress, err := iClient.Get(dev.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error getting kubernetes ingress: %s", err)
	}
	if currentIngress.Name == "" {
		log.Infof("creating ingress '%s'...", dev.Name)
		_, err = iClient.Create(newIngress)
		if err != nil {
			return fmt.Errorf("error creating kubernetes ingress: %s", err)
		}
		log.Infof("created ingress '%s'.", dev.Name)
	} else {
		log.Infof("updating ingress '%s'...", dev.Name)
		_, err = iClient.Update(newIngress)
		if err != nil {
			return fmt.Errorf("error updating kubernetes ingress: %s", err)
		}
		log.Infof("updated ingress '%s'.", dev.Name)
	}
	return nil
}

//Destroy destroys the k8s ingress
func Destroy(dev *model.Dev, s *model.Space, c *kubernetes.Clientset) error {
	log.Infof("deleting ingress '%s'...", dev.Name)
	iClient := c.ExtensionsV1beta1().Ingresses(s.Name)
	err := iClient.Delete(dev.Name, &metav1.DeleteOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Infof("ingress '%s' was already deleted.", dev.Name)
			return nil
		}
		return fmt.Errorf("irror getting kubernetes ingress: %s", err)
	}
	log.Infof("ingress '%s' deleted", dev.Name)
	return nil
}
