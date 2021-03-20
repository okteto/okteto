package configmaps

import (
	"context"
	"strings"

	"github.com/okteto/okteto/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Get returns a configmap
func Get(ctx context.Context, name, namespace string, c kubernetes.Interface) (*apiv1.ConfigMap, error) {
	cf, err := c.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cf, nil
}

// List returns a list of configmap that match labelselector
func List(ctx context.Context, namespace, labelSelector string, c *kubernetes.Clientset) ([]apiv1.ConfigMap, error) {
	cm, err := c.CoreV1().ConfigMaps(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labelSelector,
		},
	)

	if err != nil {
		return nil, err
	}

	return cm.Items, nil
}

// Deploy creates or updates a configmap
func Deploy(ctx context.Context, cf *apiv1.ConfigMap, namespace string, c *kubernetes.Clientset) error {
	_, err := Get(ctx, cf.Name, namespace, c)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return create(ctx, cf, namespace, c)
		}
		return err
	}
	return update(ctx, cf, namespace, c)
}

// Destroy deletes a configmap in a space
func Destroy(ctx context.Context, name, namespace string, c *kubernetes.Clientset) error {
	err := c.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func create(ctx context.Context, cf *apiv1.ConfigMap, namespace string, c *kubernetes.Clientset) error {
	_, err := c.CoreV1().ConfigMaps(namespace).Create(ctx, cf, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func update(ctx context.Context, cf *apiv1.ConfigMap, namespace string, c *kubernetes.Clientset) error {
	_, err := c.CoreV1().ConfigMaps(namespace).Update(ctx, cf, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
