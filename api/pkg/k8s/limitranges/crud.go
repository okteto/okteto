package limitranges

import (
	"fmt"
	"strings"

	"github.com/okteto/app/api/pkg/log"
	"github.com/okteto/app/api/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create creates limitranges for a given space
func Create(s *model.Space, c *kubernetes.Clientset) error {
	log.Debugf("Creating limit ranges '%s'...", s.ID)
	old, err := c.CoreV1().LimitRanges(s.ID).Get(s.ID, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes limit range: %s", err)
	}
	lr := translate(s)
	if old.Name == "" {
		_, err = c.CoreV1().LimitRanges(s.ID).Create(lr)
		if err != nil {
			return fmt.Errorf("Error creating kubernetes limit range: %s", err)
		}
		log.Debugf("Created limit range '%s'.", s.ID)
	} else {
		log.Debugf("Limit range '%s' was already created", s.ID)
		_, err = c.CoreV1().LimitRanges(s.ID).Update(lr)
		if err != nil {
			return fmt.Errorf("Error updating kubernetes limit range: %s", err)
		}
	}
	return nil
}
