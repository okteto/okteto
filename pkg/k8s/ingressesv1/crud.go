package ingressesv1

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"k8s.io/client-go/kubernetes"

	networkingv1 "k8s.io/api/networking/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Deploy(ctx context.Context, i *networkingv1.Ingress, c kubernetes.Interface) error {
	old, err := Get(ctx, i.Name, i.Namespace, c)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting kubernetes ingress: %s", err)
	}

	if old == nil || old.Name == "" {
		log.Infof("creating ingress '%s'", i.Name)
		_, err = c.NetworkingV1().Ingresses(i.Namespace).Create(ctx, i, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating kubernetes ingress: %s", err)
		}
		log.Infof("created ingress '%s'", i.Name)
	} else {
		log.Infof("updating ingress '%s'", i.Name)
		old.Annotations = i.Annotations
		old.Labels = i.Labels
		old.Spec = i.Spec
		_, err = c.NetworkingV1().Ingresses(i.Namespace).Update(ctx, old, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating kubernetes ingress: %s", err)
		}
		log.Infof("updated ingress '%s'.", i.Name)
	}
	return nil
}

func Get(ctx context.Context, name, namespace string, c kubernetes.Interface) (*networkingv1.Ingress, error) {
	return c.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
}

// List returns the list of deployments
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

// Destroy destroys a k8s deployment
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
