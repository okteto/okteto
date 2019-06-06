package deployments

import (
	"encoding/json"
	"fmt"

	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Get returns a deployment object given its name and namespace
func Get(name, namespace string, c *kubernetes.Clientset) (*appsv1.Deployment, error) {
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

//DevModeOn activates dev mode
func DevModeOn(d *appsv1.Deployment, dev *model.Dev, forceCreate bool, client *kubernetes.Clientset) (*apiv1.Container, error) {
	d, container, err := translate(d, dev)
	if err != nil {
		return nil, err
	}

	if forceCreate {
		if err := create(d, client); err != nil {
			return nil, err
		}
	} else {
		if err := update(d, client); err != nil {
			return nil, err
		}
	}
	return container, nil
}

//IsDevModeOn returns if a deployment is in devmode
func IsDevModeOn(d *appsv1.Deployment) bool {
	labels := d.GetObjectMeta().GetLabels()
	if labels == nil {
		return false
	}
	_, ok := labels[oktetoLabel]
	return ok
}

//IsAutoCreate returns if the deplloyment is created from scratch
func IsAutoCreate(d *appsv1.Deployment) bool {
	annotations := d.GetObjectMeta().GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[oktetoAutoCreateAnnotation]
	return ok
}

// DevModeOff deactivates dev mode for d
func DevModeOff(d *appsv1.Deployment, dev *model.Dev, image string, c *kubernetes.Clientset) error {
	dManifest := getAnnotation(d.GetObjectMeta(), oktetoDeploymentAnnotation)
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

	if err := secrets.Destroy(dev, c); err != nil {
		return err
	}

	return nil
}

func create(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	log.Debugf("creating deployment %s/%s", d.Namespace, d.Name)
	_, err := c.AppsV1().Deployments(d.Namespace).Create(d)
	if err != nil {
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
