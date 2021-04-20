package pvcs

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//Create creates a pvc
func Create(ctx context.Context, pvc *apiv1.PersistentVolumeClaim, c kubernetes.Interface) error {
	_, err := c.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

//List returns the list of pvcs
func List(ctx context.Context, namespace, labels string, c kubernetes.Interface) ([]apiv1.PersistentVolumeClaim, error) {
	iList, err := c.CoreV1().PersistentVolumeClaims(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels,
		},
	)
	if err != nil {
		return nil, err
	}
	return iList.Items, nil
}

//Destroy destroys a k8s deployment
func Destroy(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	log.Infof("deleting persistent volume claim '%s'", name)
	err := c.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error deleting kubernetes persistentVolumeClaim: %s", err)
	}
	log.Infof("PersistentVolumeClaim '%s' deleted", name)
	return nil
}
