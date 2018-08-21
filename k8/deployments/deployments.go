package deployments

import (
	"fmt"
	"log"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Deploy deploys a k8 deployment
func Deploy(d *appsv1.Deployment, namespace string, c *kubernetes.Clientset) error {
	dClient := c.AppsV1().Deployments(namespace)
	dk8, err := dClient.Get(d.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes deployment: %s", err)
	}

	if dk8.Name == "" {
		log.Printf("Creating deployment '%s'...", d.Name)
		_, err = dClient.Create(d)
		if err != nil {
			return fmt.Errorf("Error creating kubernetes deployment: %s", err)
		}
		log.Printf("Created deployment %s.", d.Name)
	} else {
		log.Printf("Updating deployment '%s'...", d.Name)
		_, err = dClient.Update(d)
		if err != nil {
			return fmt.Errorf("Error updating kubernetes deployment: %s", err)
		}
	}

	log.Printf("Waiting for the deployment '%s' to be ready...", d.Name)
	tries := 0
	for tries < 60 {
		tries++
		time.Sleep(5 * time.Second)
		d, err = dClient.Get(d.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Error getting kubernetes deployment: %s", err)
		}
		if d.Status.ReadyReplicas == 1 && d.Status.UpdatedReplicas == 1 {
			log.Printf("Kubernetes deployment '%s' is ready.", d.Name)
			return nil
		}
	}
	return fmt.Errorf("Kubernetes deployment not ready after 300s")
}

//Destroy destroysa k8 deployment
func Destroy(d *appsv1.Deployment, namespace string, c *kubernetes.Clientset) error {
	log.Printf("Deleting deployment '%s'...", d.Name)
	dClient := c.AppsV1beta1().Deployments(namespace)
	deletePolicy := metav1.DeletePropagationForeground
	err := dClient.Delete(d.Name, &metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("Error getting kubernetes deployment: %s", err)
	}

	log.Printf("Waiting for the deployment '%s' to be deleted...", d.Name)
	tries := 0
	for tries < 10 {
		tries++
		time.Sleep(5 * time.Second)
		_, err := dClient.Get(d.Name, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Printf("Deployment '%s' successfully deleted.", d.Name)
				return nil
			}
			return fmt.Errorf("Error getting kubernetes deployment: %s", err)
		}
	}
	return fmt.Errorf("Kubernetes deployment not deleted after 50s")
}
