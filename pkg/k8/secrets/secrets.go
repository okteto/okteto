package secrets

import (
	"fmt"
	"strings"

	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create creates the syncthing config secret
func Create(d *appsv1.Deployment, devList []*model.Dev, c *kubernetes.Clientset) error {
	secretName := model.GetCNDSyncSecret(d.Name)
	s, err := c.Core().Secrets(d.Namespace).Get(secretName, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes secret: %s", err)
	}
	config, err := getConfigXML(devList)
	if err != nil {
		return fmt.Errorf("Error generating syncthing configuration: %s", err)
	}
	data := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName},
		Type:       v1.SecretTypeOpaque,
		Data: map[string][]byte{
			"config.xml": config,
			"cert.pem":   []byte(certPEM),
			"key.pem":    []byte(keyPEM),
		},
	}
	if s.Name == "" {
		_, err := c.Core().Secrets(d.Namespace).Create(data)
		if err != nil {
			return fmt.Errorf("Error creating kubernetes sync secret: %s", err)
		}
		log.Infof("Created syncthing secret '%s'.", secretName)
	} else {
		_, err := c.Core().Secrets(d.Namespace).Update(data)
		if err != nil {
			return fmt.Errorf("Error updating kubernetes sync secret: %s", err)
		}
		log.Infof("Sync secret '%s' was updated.", secretName)
	}
	return nil
}

//Delete deletes the syncthing config secret
func Delete(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	secretName := model.GetCNDSyncSecret(d.Name)
	err := c.Core().Secrets(d.Namespace).Delete(secretName, &metav1.DeleteOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("Error deleting kubernetes sync secret: %s", err)
	}
	return nil
}
