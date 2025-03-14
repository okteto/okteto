package endpoints

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetByName(ctx context.Context, name, namespace string, c kubernetes.Interface) (*corev1.Endpoints, error) {
	e, err := c.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting kubernetes endpoint: %s", err)
	}
	return e, nil
}
