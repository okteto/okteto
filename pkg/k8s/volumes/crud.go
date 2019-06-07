package volumes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

const (
	maxRetries = 150
)

//Create deploys the volume claim for a given dev environment
func Create(ctx context.Context, name string, dev *model.Dev, c *kubernetes.Clientset) error {
	vClient := c.CoreV1().PersistentVolumeClaims(dev.Namespace)
	pvc := translate(name)
	k8Volume, err := vClient.Get(pvc.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error getting kubernetes volume claim: %s", err)
	}
	if k8Volume.Name == "" {
		log.Infof("creating volume claim '%s'...", pvc.Name)
		k8Volume, err = vClient.Create(pvc)
		if err != nil {
			return fmt.Errorf("error creating kubernetes volume claim: %s", err)
		}
	}

	tries := 0
	ticker := time.NewTicker(1 * time.Second)
	for k8Volume.Status.Phase != apiv1.ClaimBound {
		log.Debugf("PVC '%s' is '%s'...", k8Volume.Name, k8Volume.Status.Phase)
		select {
		case <-ticker.C:
			tries++
			if tries >= maxRetries {
				return fmt.Errorf("error creating PVC '%s'. It is '%s' after 150s", k8Volume.Name, k8Volume.Status.Phase)
			}
		case <-ctx.Done():
			log.Debug("cancelling call to create volume")
			return ctx.Err()
		}
		k8Volume, err = vClient.Get(pvc.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("error getting kubernetes volume claim: %s", err)
		}
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
