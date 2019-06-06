package volumes

import (
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create deploys the volume claim for a given dev environment
func Create(name string, dev *model.Dev, c *kubernetes.Clientset) error {
	vClient := c.CoreV1().PersistentVolumeClaims(dev.Namespace)
	pvc := translate(name)
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
func Destroy(name string, dev *model.Dev, c *kubernetes.Clientset) error {
	pvc := translate(name)
	vClient := c.CoreV1().PersistentVolumeClaims(dev.Namespace)
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
