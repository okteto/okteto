package deployments

import (
	"encoding/json"
	"fmt"

	"github.com/okteto/app/cli/pkg/errors"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Get returns a deployment object given its name and namespace
func Get(namespace, name string, c *kubernetes.Clientset) (*appsv1.Deployment, error) {
	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	d, err := c.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		log.Debugf("error while retrieving deployment %s/%s: %s", namespace, name, err)
		return nil, err
	}

	return d, nil
}

// DevModeOff deactivates dev mode for d
func DevModeOff(d *appsv1.Deployment, dev *model.Dev, image string, c *kubernetes.Clientset) error {
	dManifest := getAnnotation(d.GetObjectMeta(), deploymentAnnotation)
	if len(dManifest) == 0 {
		log.Infof("%s/%s is not an okteto environment", d.Namespace, d.Name)
		return nil
	}

	dOrig := &appsv1.Deployment{}
	if err := json.Unmarshal([]byte(dManifest), dOrig); err != nil {
		return fmt.Errorf("malformed manifest: %s", err)
	}

	if image != "" {
		dOrig.Spec.Template.Spec.Containers[0].Image = image
	}

	dOrig.ResourceVersion = ""
	if err := update(dOrig, c); err != nil {
		return err
	}

	if err := deleteSecret(d, c); err != nil {
		return err
	}

	return nil
}

func update(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	log.Debugf("updating deployment %s/%s", d.Namespace, d.Name)
	_, err := c.AppsV1().Deployments(d.Namespace).Update(d)
	if err != nil {
		return err
	}

	return nil
}

func deleteSecret(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	secretName := fmt.Sprintf(oktetoSecretTemplate, d.Name)
	log.Debugf("deleting secret %s/%s", d.Namespace, secretName)
	err := c.Core().Secrets(d.Namespace).Delete(secretName, &metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("Error deleting kubernetes sync secret: %s", err)
	}
	return nil
}
