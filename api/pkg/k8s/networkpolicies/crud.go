package networkpolicies

import (
	"fmt"
	"strings"

	"github.com/okteto/app/api/pkg/log"

	"github.com/okteto/app/api/pkg/model"

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

// Create creates the network policies of a given space
func Create(s *model.Space, c *kubernetes.Clientset) error {
	log.Debugf("Creating network policies '%s'...", s.ID)
	new := translate(s)
	if err := create(new, s, c); err != nil {
		return err
	}
	dns := translateDNS(s)
	if err := create(dns, s, c); err != nil {
		return err
	}
	return nil
}

func create(new *netv1.NetworkPolicy, s *model.Space, c *kubernetes.Clientset) error {
	old, err := c.NetworkingV1().NetworkPolicies(s.ID).Get(new.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error getting kubernetes network policy: %s", err)
	}
	if old.Name == "" {
		_, err := c.NetworkingV1().NetworkPolicies(s.ID).Create(new)
		if err != nil {
			return fmt.Errorf("error creating kubernetes network policy: %s", err)
		}
		log.Debugf("created network policy '%s'.", new.Name)
	} else {
		_, err := c.NetworkingV1().NetworkPolicies(s.ID).Update(new)
		if err != nil {
			return fmt.Errorf("error updating kubernetes network policy: %s", err)
		}
		log.Debugf("updated network policy '%s'.", new.Name)
	}
	return nil
}
