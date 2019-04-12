package networkpolicies

import (
	"fmt"
	"strings"

	"github.com/okteto/app/api/log"

	"github.com/okteto/app/api/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

// Create creates the network policies of a given space
func Create(s *model.Space, c *kubernetes.Clientset) error {
	log.Debugf("Creating network policy '%s'...", s.ID)
	n, err := c.NetworkingV1().NetworkPolicies(s.ID).Get(s.ID, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes network policy: %s", err)
	}
	if n.Name == "" {
		n := translate(s)
		_, err := c.NetworkingV1().NetworkPolicies(s.ID).Create(n)
		if err != nil {
			return fmt.Errorf("Error creating kubernetes network policy: %s", err)
		}
		log.Debugf("Created network policy '%s'.", s.ID)
	} else {
		log.Debugf("Network policy '%s' was already created.", s.ID)
	}
	return nil
}
