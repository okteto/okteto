package quotas

import (
	"fmt"
	"strings"

	"github.com/okteto/app/api/pkg/log"
	"github.com/okteto/app/api/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create creates quotas for a given space
func Create(s *model.Space, c *kubernetes.Clientset) error {
	log.Debugf("Creating quotas '%s'...", s.ID)
	old, err := c.CoreV1().ResourceQuotas(s.ID).Get(s.ID, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes quota: %s", err)
	}
	q := translate(s)
	if old.Name == "" {
		_, err = c.CoreV1().ResourceQuotas(s.ID).Create(q)
		if err != nil {
			return fmt.Errorf("Error creating kubernetes quota: %s", err)
		}
		log.Debugf("Created quota '%s'.", s.ID)
	} else {
		log.Debugf("Quota '%s' was already created", s.ID)
		_, err = c.CoreV1().ResourceQuotas(s.ID).Update(q)
		if err != nil {
			return fmt.Errorf("Error updating kubernetes quota: %s", err)
		}
	}
	return nil
}
