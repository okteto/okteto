package volumes

import (
	"fmt"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

const maxRetries = 300

//Destroy destroys the volume claim for a given dev environment
func Destroy(name string, dev *model.Dev, c *kubernetes.Clientset) error {
	vClient := c.CoreV1().PersistentVolumeClaims(dev.Namespace)
	log.Infof("destroying volume claim '%s'...", name)

	if err := checkIfAttached(name, dev, c); err != nil {
		return err
	}

	ticker := time.NewTicker(1 * time.Second)
	for i := 0; i < maxRetries; i++ {
		err := vClient.Delete(name, &metav1.DeleteOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Infof("volume claim '%s' successfully deleted", name)
				return nil
			}

			return fmt.Errorf("error getting kubernetes volume claim: %s", err)
		}

		<-ticker.C
	}

	return fmt.Errorf("volume claim '%s' wasn't deleted after 300s", name)
}

func checkIfAttached(pvc string, dev *model.Dev, c *kubernetes.Clientset) error {
	pods, err := c.CoreV1().Pods(dev.Namespace).List(metav1.ListOptions{})
	if err != nil {
		log.Infof("failed to get available pods: %s", err)
		return nil
	}

	for _, p := range pods.Items {
		if strings.HasPrefix(p.Name, dev.Name) {
			continue
		}

		for _, v := range p.Spec.Volumes {
			if v.PersistentVolumeClaim != nil {
				if v.PersistentVolumeClaim.ClaimName == pvc {
					log.Infof("pvc/%s is still attached to pod/%s", pvc, p.Name)
					return fmt.Errorf("can't delete your persistent volume since it's still attached to 'pod/%s'", p.Name)
				}
			}
		}
	}

	return nil

}
