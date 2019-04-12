package volumes

import (
	"fmt"
	"strings"
	"time"

	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create deploys the volume claim for a given dev environment
func Create(dev *model.Dev, s *model.Space, c *kubernetes.Clientset) error {
	vClient := c.CoreV1().PersistentVolumeClaims(s.ID)
	pvc := translate(dev)
	k8Volume, err := vClient.Get(pvc.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error getting kubernetes volume claim: %s", err)
	}
	if k8Volume.Name != "" {
		return nil
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
			return nil
		}
	}
	return fmt.Errorf("kubernetes volume claim '%s' not ready after 5 minutes", pvc.Name)
}

//Destroy destroys the volume claim for a given dev environment
func Destroy(dev *model.Dev, s *model.Space, c *kubernetes.Clientset) error {
	pvc := translate(dev)
	vClient := c.CoreV1().PersistentVolumeClaims(s.ID)
	log.Infof("destroying volume claim '%s'...", pvc.Name)
	err := vClient.Delete(pvc.Name, &metav1.DeleteOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("error getting kubernetes volume claim: %s", err)
	}

	log.Infof("waiting for the volume claim '%s' to be destroyed...", pvc.Name)
	tries := 0
	for tries < 90 {
		tries++
		time.Sleep(2 * time.Second)
		_, err := vClient.Get(pvc.Name, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Infof("volume claim '%s' successfully destroyed.", pvc.Name)
				return nil
			}
			return fmt.Errorf("error getting kubernetes volume claim: %s", err)
		}
	}
	return fmt.Errorf("kubernetes volume claim '%s' not destroyed after 3 minutes", pvc.Name)
}
