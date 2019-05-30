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

const (
	devAnnotation            = "dev.okteto.com/deployment"
	oktetoVolumeTemplate     = "okteto-%s"
	oktetoVolumeDataTemplate = "okteto-%s-%d"
	oktetoSecretTemplate     = "okteto-%s"
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
func DevModeOff(d *appsv1.Deployment, dev *model.Dev, c *kubernetes.Clientset) error {
	manifest := getAnnotation(d.GetObjectMeta(), devAnnotation)
	if len(manifest) == 0 {
		log.Infof("%s/%s is not an okteto environment", d.Namespace, d.Name)
		return nil
	}

	dOrig := &appsv1.Deployment{}
	if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
		return fmt.Errorf("malformed manifest: %s", err)
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

// DestroyVolumes deletes the volumes attached to the Okteto environment
func DestroyVolumes(dev *model.Dev, c *kubernetes.Clientset) error {
	if err := destroyVolume(fmt.Sprintf(oktetoVolumeTemplate, dev.Name), dev.Space, c); err != nil {
		return err
	}

	for i := range dev.Volumes {
		if err := destroyVolume(fmt.Sprintf(oktetoVolumeDataTemplate, dev.Name, i), dev.Space, c); err != nil {
			return err
		}
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

func destroyVolume(namespace, name string, c *kubernetes.Clientset) error {
	log.Debugf("destroying volume %s/%s", namespace, name)
	err := c.Core().PersistentVolumeClaims(namespace).Delete(name, &metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}

	return err
}

func getAnnotation(o metav1.Object, key string) string {
	annotations := o.GetAnnotations()
	if annotations != nil {
		return annotations[key]
	}
	return ""
}
