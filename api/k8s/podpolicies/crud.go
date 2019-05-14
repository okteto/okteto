package podpolicies

import (
	"fmt"
	"strings"

	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create creates the pod security policy for a given space
func Create(s *model.Space, c *kubernetes.Clientset) error {
	log.Debugf("Creating pod security policy '%s'...", s.ID)
	old, err := c.PolicyV1beta1().PodSecurityPolicies().Get(s.ID, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes pod security policy: %s", err)
	}
	psp := translate(s)
	if old.Name == "" {
		_, err = c.PolicyV1beta1().PodSecurityPolicies().Create(psp)
		if err != nil {
			return fmt.Errorf("Error creating kubernetes pod security policy: %s", err)
		}
		log.Debugf("Created pod security policy '%s'.", s.ID)
	} else {
		log.Debugf("Pod security policy '%s' was already created", s.ID)
		_, err = c.PolicyV1beta1().PodSecurityPolicies().Update(psp)
		if err != nil {
			return fmt.Errorf("Error updating kubernetes pod security policy: %s", err)
		}
		log.Debugf("Updated pod security policy '%s'.", s.ID)
	}
	return nil
}
