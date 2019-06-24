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

//Destroy destroys the volume claim for a given dev environment
func Destroy(name string, dev *model.Dev, c *kubernetes.Clientset) error {
	vClient := c.CoreV1().PersistentVolumeClaims(dev.Namespace)
	log.Infof("destroying volume claim '%s'...", name)
	for {
		err := vClient.Delete(name, &metav1.DeleteOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil
			}
			return fmt.Errorf("error getting kubernetes volume claim: %s", err)
		}
		time.Sleep(1 * time.Second)
	}
}
