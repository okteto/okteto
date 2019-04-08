package volume

import (
	"fmt"
	"strings"
	"time"

	"cli/cnd/pkg/log"
	"cli/cnd/pkg/model"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Deploy deploys a list of volume claims
func Deploy(d *appsv1.Deployment, devList []*model.Dev, c *kubernetes.Clientset) error {
	vClient := c.CoreV1().PersistentVolumeClaims(d.Namespace)
	for _, pvc := range translate(devList) {
		k8Volume, err := vClient.Get(pvc.Name, metav1.GetOptions{})
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("error getting kubernetes volume claim: %s", err)
		}
		if k8Volume.Name != "" {
			continue
		}

		log.Infof("creating volume claim '%s'...", pvc.Name)
		_, err = vClient.Create(pvc)
		if err != nil {
			return fmt.Errorf("error creating kubernetes volume claim: %s", err)
		}

		log.Infof("waiting for the volume claim '%s' to be ready...", pvc.Name)
		tries := 0
		for tries < 150 {
			tries++
			time.Sleep(2 * time.Second)
			k8Volume, err = vClient.Get(pvc.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error getting kubernetes volume claim: %s", err)
			}
			if k8Volume.Status.Phase == apiv1.ClaimBound {
				log.Infof("kubernetes volume claim '%s' is bound.", pvc.Name)
				break
			}
		}
		if tries < 150 {
			continue
		}
		return fmt.Errorf("kubernetes volume claim '%s' not ready after 5 minutes", pvc.Name)
	}
	return nil
}

//Destroy destroys a list of volume claims
func Destroy(pvcName, namespace string, c *kubernetes.Clientset) error {
	vClient := c.CoreV1().PersistentVolumeClaims(namespace)
	log.Infof("destroying volume claim '%s'...", pvcName)
	err := vClient.Delete(pvcName, &metav1.DeleteOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("error getting kubernetes volume claim: %s", err)
	}

	log.Infof("waiting for the volume claim '%s' to be destroyed...", pvcName)
	tries := 0
	for tries < 90 {
		tries++
		time.Sleep(2 * time.Second)
		_, err := vClient.Get(pvcName, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Infof("volume claim '%s' successfully destroyed.", pvcName)
				return nil
			}
			return fmt.Errorf("error getting kubernetes volume claim: %s", err)
		}
	}
	return fmt.Errorf("kubernetes volume claim '%s' not destroyed after 3 minutes", pvcName)
}
