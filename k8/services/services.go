package services

import (
	"fmt"
	"log"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Deploy deploys a k8 service
func Deploy(s *apiv1.Service, namespace string, c *kubernetes.Clientset) error {
	sClient := c.CoreV1().Services(namespace)
	sk8, err := sClient.Get(s.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting k8 service: %s", err)
	}
	if sk8.Name != "" {
		log.Printf("Updating service '%s'...", s.Name)
		_, err = sClient.Update(s)
		if err != nil {
			return fmt.Errorf("Error updating kubernetes service: %s", err)
		}
		log.Printf("Updated service '%s'.", s.Name)
	}
	return nil
}
