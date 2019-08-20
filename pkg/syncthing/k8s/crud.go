package k8s

import (
	"fmt"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var devTerminationGracePeriodSeconds int64

//Deploy creates or updates a syncthing stateful set
func Deploy(dev *model.Dev, d *appsv1.Deployment, c *apiv1.Container, client *kubernetes.Clientset) error {
	ss := translate(dev, d, c)

	if old := exists(ss, client); old != nil {
		ss.Spec.Template.Spec.NodeName = pods.GetSyncNode(dev, client)
		ss.Spec.PodManagementPolicy = old.Spec.PodManagementPolicy
		if err := update(ss, client); err != nil {
			if strings.Contains(err.Error(), "updates to statefulset spec for fields other than") {
				return fmt.Errorf("You have done an incompatible change with your previous okteto configuration. Run 'okteto down -v' and execute 'okteto up' again")
			}
			return err
		}
	} else {
		if err := create(ss, client); err != nil {
			return err
		}
	}
	return nil
}

func exists(ss *appsv1.StatefulSet, c *kubernetes.Clientset) *appsv1.StatefulSet {
	ss, err := c.AppsV1().StatefulSets(ss.Namespace).Get(ss.Name, metav1.GetOptions{})
	if err != nil || ss.Name == "" {
		return nil
	}
	return ss
}

func create(ss *appsv1.StatefulSet, c *kubernetes.Clientset) error {
	log.Infof("creating syncthing statefulset '%s", ss.Name)
	_, err := c.AppsV1().StatefulSets(ss.Namespace).Create(ss)
	if err != nil {
		return fmt.Errorf("error creating kubernetes syncthing statefulset: %s", err)
	}
	log.Infof("syncthing statefulset '%s' created", ss.Name)
	return nil
}

func update(ss *appsv1.StatefulSet, c *kubernetes.Clientset) error {
	log.Infof("updating syncthing statefulset '%s'", ss.Name)
	if _, err := c.AppsV1().StatefulSets(ss.Namespace).Update(ss); err != nil {
		return fmt.Errorf("error updating kubernetes syncthing statefulset: %s", err)
	}
	log.Infof("syncthing statefulset '%s' updated", ss.Name)
	return nil
}

// Destroy destroys a database
func Destroy(dev *model.Dev, c *kubernetes.Clientset) error {
	log.Infof("destroying syncthing statefulset '%s' ...", dev.Name)
	sfsClient := c.AppsV1().StatefulSets(dev.Namespace)
	if err := sfsClient.Delete(dev.GetStatefulSetName(), &metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("couldn't destroy syncthing statefulset: %s", err)
		}
	}

	ticker := time.NewTicker(1 * time.Second)
	for i := 0; i < 30; i++ {
		_, err := sfsClient.Get(dev.GetStatefulSetName(), metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}

			return err
		}
		<-ticker.C
	}

	log.Infof("syncthing statefulset '%s' destroyed", dev.Name)
	return nil
}
