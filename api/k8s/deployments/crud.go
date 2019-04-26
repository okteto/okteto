package deployments

import (
	"fmt"
	"strings"

	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const maxDevEnvironments = 5

//DevOn activates dev mode
func DevOn(dev *model.Dev, s *model.Space, c *kubernetes.Clientset) error {
	deploys, err := c.AppsV1().Deployments(s.ID).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	numDeploys := len(deploys.Items)
	dPrev := get(dev, deploys.Items)
	d, err := translate(dev, dPrev, s)
	if err != nil {
		return err
	}

	if dPrev != nil {
		if dev.Attach {
			if _, ok := dPrev.GetObjectMeta().GetLabels()[oktetoLabel]; !ok {
				return fmt.Errorf("Your Okteto Environment has been deactivated")
			}
		}
		if err := update(d, c); err != nil {
			return err
		}
	} else {
		if dev.Attach {
			return fmt.Errorf("Your Okteto Environment has been removed")
		}
		if numDeploys >= maxDevEnvironments {
			return fmt.Errorf("You cannot create more than %d environments in your space", maxDevEnvironments)
		}
		if err := create(d, c); err != nil {
			return err
		}
	}
	return nil
}

//Run replaces a dev environment by a deployment running a docker image
func Run(dev *model.Dev, s *model.Space, c *kubernetes.Clientset) error {
	d := devSandbox(dev, s)
	translateResources(&d.Spec.Template.Spec.Containers[0])

	deploys, err := c.AppsV1().Deployments(s.ID).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	numDeploys := len(deploys.Items)
	dPrev := get(dev, deploys.Items)

	if dPrev != nil {
		if err := update(d, c); err != nil {
			return err
		}
	} else {
		if numDeploys >= maxDevEnvironments {
			return fmt.Errorf("You cannot create more than %d environments in your space", maxDevEnvironments)
		}
		if err := create(d, c); err != nil {
			return err
		}
	}
	return nil
}

func get(dev *model.Dev, l []appsv1.Deployment) *appsv1.Deployment {
	for _, i := range l {
		if i.Name == dev.Name {
			return &i
		}
	}
	return nil
}

func create(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	log.Infof("creating deployment '%s' in '%s'...", d.Name, d.Namespace)
	dClient := c.AppsV1().Deployments(d.Namespace)
	_, err := dClient.Create(d)
	if err != nil {
		return fmt.Errorf("error creating kubernetes deployment: %s", err)
	}
	log.Infof("deployment '%s' created", d.Name)
	return nil
}

func update(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	log.Infof("updating deployment '%s' in '%s' ...", d.Name, d.Namespace)
	dClient := c.AppsV1().Deployments(d.Namespace)
	if _, err := dClient.Update(d); err != nil {
		return fmt.Errorf("error updating kubernetes deployment: %s", err)
	}
	log.Infof("deployment '%s' updated", d.Name)
	return nil
}

//List lists the deployments in a space
func List(s *model.Space, c *kubernetes.Clientset) ([]appsv1.Deployment, error) {
	deploys, err := c.AppsV1().Deployments(s.ID).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return deploys.Items, nil
}

// Destroy destroys a deployment
func Destroy(dev *model.Dev, s *model.Space, c *kubernetes.Clientset) error {
	log.Infof("destroying deployment '%s' in '%s' ...", dev.Name, s.ID)
	dClient := c.AppsV1().Deployments(s.ID)
	if err := dClient.Delete(dev.Name, &metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("couldn't destroy deployment: %s", err)
		}
	}
	log.Infof("deployment '%s' destroyed", dev.Name)
	return nil
}
