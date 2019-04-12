package serviceaccounts

import (
	"fmt"
	"strings"

	"github.com/okteto/app/api/k8s/client"
	"github.com/okteto/app/api/k8s/secrets"
	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create creates a service account for a given space
func Create(s *model.Space, c *kubernetes.Clientset) error {
	log.Debugf("Creating service account '%s'...", s.Name)
	sa, err := c.CoreV1().ServiceAccounts(s.Name).Get(s.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes service account: %s", err)
	}
	if sa.Name != "" {
		log.Debugf("Service account '%s' was already created", s.Name)
		return nil
	}
	sa = translate(s)
	_, err = c.CoreV1().ServiceAccounts(s.Name).Create(sa)
	if err != nil {
		return fmt.Errorf("Error creating kubernetes service account: %s", err)
	}
	log.Debugf("Created service account '%s'.", s.Name)
	return nil
}

//GetCredentialConfig returns the credential for accessing the dev mode container
func GetCredentialConfig(s *model.Space) (string, error) {
	log.Debug("Get service account credential")
	c, err := client.Get()
	if err != nil {
		return "", err
	}
	sa, err := c.CoreV1().ServiceAccounts(s.Name).Get(s.Name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Error getting kubernetes service account: %s", err)
	}
	secret, err := secrets.Get(sa.Secrets[0].Name, s, c)
	if err != nil {
		return "", err
	}
	return getConfigB64(s, string(secret.Data["ca.crt"]), string(secret.Data["token"])), nil
}
