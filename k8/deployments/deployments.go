package deployments

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Deploy deploys a k8 deployment
func Deploy(d *appsv1.Deployment, namespace string, c *kubernetes.Clientset) error {
	deploymentName := GetFullName(namespace, d.Name)
	dClient := c.AppsV1().Deployments(namespace)
	dk8, err := dClient.Get(d.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes deployment: %s", err)
	}

	if dk8.Name == "" {
		log.Debugf("Creating deployment '%s'...", deploymentName)
		_, err = dClient.Create(d)
		if err != nil {
			return fmt.Errorf("Error creating kubernetes deployment: %s", err)
		}
		log.Debugf("Created deployment %s.", deploymentName)
	} else {
		log.Debugf("Updating deployment '%s'...", deploymentName)
		_, err = dClient.Update(d)
		if err != nil {
			return fmt.Errorf("Error updating kubernetes deployment: %s", err)
		}
	}

	log.Debugf("Waiting for the deployment '%s' to be ready...", deploymentName)
	tries := 0
	for tries < 60 {
		tries++
		time.Sleep(5 * time.Second)
		d, err = dClient.Get(d.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Error getting kubernetes deployment: %s", err)
		}
		if d.Status.ReadyReplicas == 1 && d.Status.UpdatedReplicas == 1 {
			log.Debugf("Kubernetes deployment '%s' is ready.", deploymentName)
			return nil
		}
	}
	return fmt.Errorf("Kubernetes deployment  %s not ready after 300s", deploymentName)
}

//Destroy destroysa k8 deployment
func Destroy(d *appsv1.Deployment, namespace string, c *kubernetes.Clientset) error {
	deploymentName := GetFullName(namespace, d.Name)
	log.Debugf("Deleting deployment '%s'...", deploymentName)
	dClient := c.AppsV1beta1().Deployments(namespace)
	deletePolicy := metav1.DeletePropagationForeground
	err := dClient.Delete(d.Name, &metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("Error getting kubernetes deployment: %s", err)
	}

	log.Debugf("Waiting for the deployment '%s' to be deleted...", deploymentName)
	tries := 0
	for tries < 10 {
		tries++
		time.Sleep(5 * time.Second)
		_, err := dClient.Get(d.Name, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Infof("Deployment '%s' successfully deleted.", deploymentName)
				return nil
			}
			return fmt.Errorf("Error getting %s: %s", deploymentName, err)
		}
	}
	return fmt.Errorf("%s not deleted after 50s", deploymentName)
}

// GetFullName returns the full name of the deployment. This is mostly used for logs and labels
func GetFullName(deploymentName, namespace string) string {
	return fmt.Sprintf("%s/%s", namespace, deploymentName)
}
