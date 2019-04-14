package volumes

import (
	"fmt"
	"strings"

	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"

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
	return nil
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
	return nil
}
