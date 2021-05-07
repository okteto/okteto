package ingress

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"k8s.io/client-go/kubernetes"

	networkingv1 "k8s.io/api/networking/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Create(ctx context.Context, i *networkingv1.Ingress, c kubernetes.Interface) error {
	_, err := c.NetworkingV1().Ingresses(i.Namespace).Create(ctx, i, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

//List returns the list of deployments
func List(ctx context.Context, namespace, labels string, c kubernetes.Interface) ([]networkingv1.Ingress, error) {
	iList, err := c.NetworkingV1().Ingresses(namespace).List(
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
	log.Infof("deleting ingress '%s'", name)
	err := c.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error deleting kubernetes ingress: %s", err)
	}
	log.Infof("Ingress '%s' deleted", name)
	return nil
}

//Update updates a statefulset
func Update(ctx context.Context, i *networkingv1.Ingress, c kubernetes.Interface) error {
	_, err := c.NetworkingV1().Ingresses(i.Namespace).Update(ctx, i, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
