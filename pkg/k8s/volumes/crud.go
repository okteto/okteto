package volumes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

const (
	maxRetries = 90
)

//Create deploys the volume claim for a given dev environment
func Create(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) error {
	vClient := c.CoreV1().PersistentVolumeClaims(dev.Namespace)
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
func Destroy(dev *model.Dev, c *kubernetes.Clientset) error {
	vClient := c.CoreV1().PersistentVolumeClaims(dev.Namespace)
	log.Infof("destroying volume claim '%s'...", dev.GetVolumeName())

	ticker := time.NewTicker(1 * time.Second)
	for i := 0; i < maxRetries; i++ {
		err := vClient.Delete(dev.GetVolumeName(), &metav1.DeleteOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Infof("volume claim '%s' successfully destroyed", dev.GetVolumeName())
				return nil
			}

			return fmt.Errorf("error getting kubernetes volume claim: %s", err)
		}

		<-ticker.C
		if i%10 == 5 {
			log.Infof("waiting for volume claim '%s' to be destroyed...", dev.GetVolumeName())
		}
	}
	if err := checkIfAttached(dev, c); err != nil {
		return err
	}
	return fmt.Errorf("volume claim '%s' wasn't destroyed after 120s", dev.GetVolumeName())
}

func checkIfAttached(dev *model.Dev, c *kubernetes.Clientset) error {
	pods, err := c.CoreV1().Pods(dev.Namespace).List(metav1.ListOptions{})
	if err != nil {
		log.Infof("failed to get available pods: %s", err)
		return nil
	}

	for _, p := range pods.Items {
		for _, v := range p.Spec.Volumes {
			if v.PersistentVolumeClaim != nil {
				if v.PersistentVolumeClaim.ClaimName == dev.GetVolumeName() {
					log.Infof("pvc/%s is still attached to pod/%s", dev.GetVolumeName(), p.Name)
					return fmt.Errorf("can't delete your volume claim since it's still attached to 'pod/%s'", p.Name)
				}
			}
		}
	}

	return nil
}
