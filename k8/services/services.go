package services

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Deploy deploys a k8 service
func Deploy(s *apiv1.Service, namespace string, c *kubernetes.Clientset) error {
	serviceName := fmt.Sprintf("%s/%s", namespace, s.Name)
	sClient := c.CoreV1().Services(namespace)
	sk8, err := sClient.Get(s.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting k8 service: %s", err)
	}

	if sk8.Name == "" {
		log.Infof("Creating service '%s'...", serviceName)
		_, err = sClient.Create(s)
		if err != nil {
			return fmt.Errorf("Error creating kubernetes service: %s", err)
		}
		log.Infof("Created service '%s'.", serviceName)
	} else {
		log.Infof("Updating service '%s'...", serviceName)
		s.Spec.ClusterIP = sk8.Spec.ClusterIP
		s.GetObjectMeta().SetResourceVersion(sk8.GetObjectMeta().GetResourceVersion())
		_, err = sClient.Update(s)
		if err != nil {
			return fmt.Errorf("Error updating kubernetes service: %s", err)
		}
		log.Infof("Updated service '%s'.", serviceName)
	}
	return nil
}
