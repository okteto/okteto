package statefulsets

import (
	"fmt"
	"strings"

	"github.com/okteto/app/api/k8s/statefulsets/mongo"
	"github.com/okteto/app/api/k8s/statefulsets/postgres"
	"github.com/okteto/app/api/k8s/statefulsets/redis"

	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var devTerminationGracePeriodSeconds int64

//Deploy creates or updates a database
func Deploy(db *model.DB, s *model.Space, c *kubernetes.Clientset) error {
	var dbSFS *appsv1.StatefulSet
	switch db.Name {
	case model.MONGO:
		dbSFS = mongo.TranslateStatefulSet(s)
	case model.REDIS:
		dbSFS = redis.TranslateStatefulSet(s)
	case model.POSTGRES:
		dbSFS = postgres.TranslateStatefulSet(s)
	}
	if exists(dbSFS, s, c) {
		if err := update(dbSFS, s, c); err != nil {
			return err
		}
	} else {
		if err := create(dbSFS, s, c); err != nil {
			return err
		}
	}
	return nil
}

func exists(dbSFS *appsv1.StatefulSet, s *model.Space, c *kubernetes.Clientset) bool {
	dbSFS, err := c.AppsV1().StatefulSets(s.Name).Get(dbSFS.Name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return dbSFS.Name != ""
}

func create(dbSFS *appsv1.StatefulSet, s *model.Space, c *kubernetes.Clientset) error {
	log.Infof("creating statefulset '%s' in '%s'...", dbSFS.Name, s.Name)
	sfsClient := c.AppsV1().StatefulSets(s.Name)
	_, err := sfsClient.Create(dbSFS)
	if err != nil {
		return fmt.Errorf("error creating kubernetes statefulset: %s", err)
	}
	log.Infof("statefulset '%s' created", dbSFS.Name)
	return nil
}

func update(dbSFS *appsv1.StatefulSet, s *model.Space, c *kubernetes.Clientset) error {
	log.Infof("updating statefulset '%s' in '%s' ...", dbSFS.Name, s.Name)
	sfsClient := c.AppsV1().StatefulSets(s.Name)
	if _, err := sfsClient.Update(dbSFS); err != nil {
		return fmt.Errorf("error updating kubernetes statefulset: %s", err)
	}
	log.Infof("statefulset '%s' updated", dbSFS.Name)
	return nil
}

//List lists the statefulsets in a space
func List(s *model.Space, c *kubernetes.Clientset) ([]appsv1.StatefulSet, error) {
	stss, err := c.AppsV1().StatefulSets(s.Name).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return stss.Items, nil
}

// Destroy destroys a database
func Destroy(db *model.DB, s *model.Space, c *kubernetes.Clientset) error {
	log.Infof("destroying statefulset '%s' in '%s' ...", db.Name, s.Name)
	sfsClient := c.AppsV1().StatefulSets(s.Name)
	if err := sfsClient.Delete(db.Name, &metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("couldn't destroy statefulset: %s", err)
		}
	}
	log.Infof("statefulset '%s' destroyed", db.Name)
	return nil
}
