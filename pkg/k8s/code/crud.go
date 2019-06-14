package code

import (
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var devTerminationGracePeriodSeconds int64

//Deploy creates or updates a database
func Deploy(dev *model.Dev, c *kubernetes.Clientset) error {
	ss := translate(dev)

	if exists(ss, c) {
		if err := update(ss, c); err != nil {
			return err
		}
	} else {
		if err := create(ss, c); err != nil {
			return err
		}
	}
	return nil
}

func exists(ss *appsv1.StatefulSet, c *kubernetes.Clientset) bool {
	ss, err := c.AppsV1().StatefulSets(ss.Namespace).Get(ss.Name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return ss.Name != ""
}

func create(ss *appsv1.StatefulSet, c *kubernetes.Clientset) error {
	log.Infof("creating code statefulset '%s", ss.Name)
	_, err := c.AppsV1().StatefulSets(ss.Namespace).Create(ss)
	if err != nil {
		return fmt.Errorf("error creating kubernetes code statefulset: %s", err)
	}
	log.Infof("code statefulset '%s' created", ss.Name)
	return nil
}

func update(ss *appsv1.StatefulSet, c *kubernetes.Clientset) error {
	log.Infof("updating code statefulset '%s'", ss.Name)
	if _, err := c.AppsV1().StatefulSets(ss.Namespace).Update(ss); err != nil {
		return fmt.Errorf("error updating kubernetes code statefulset: %s", err)
	}
	log.Infof("code statefulset '%s' updated", ss.Name)
	return nil
}

// Destroy destroys a database
func Destroy(dev *model.Dev, c *kubernetes.Clientset) error {
	log.Infof("destroying code statefulset '%s' ...", dev.Name)
	sfsClient := c.AppsV1().StatefulSets(dev.Namespace)
	if err := sfsClient.Delete(dev.Name, &metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("couldn't destroy code statefulset: %s", err)
		}
	}
	log.Infof("code statefulset '%s' destroyed", dev.Name)
	return nil
}
