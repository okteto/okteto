package k8s

import (
	"context"
	"time"

	"github.com/okteto/okteto/pkg/constants"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DivertManager is the manager for Divert resources in Kubernetes.
type DivertManager struct {
	Client DivertV1Interface
}

func NewDivertManager(client DivertV1Interface) *DivertManager {
	return &DivertManager{
		Client: client,
	}
}

// CreateOrUpdate creates or updates a Divert resource in Kubernetes.
// If the resource already exists, it updates it; otherwise, it creates a new one.
// It returns an error if the operation fails.
func (dm *DivertManager) CreateOrUpdate(ctx context.Context, d *Divert) error {
	old, err := dm.Client.Diverts(d.Namespace).Get(ctx, "foo", metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return dm.create(ctx, d)
		}

		return err
	}

	old.Spec = d.Spec
	return dm.update(ctx, old)
}

func (dm *DivertManager) create(ctx context.Context, d *Divert) error {
	if d.Annotations == nil {
		d.Annotations = make(map[string]string)
	}
	d.Annotations[constants.LastUpdatedAnnotation] = time.Now().UTC().Format(constants.TimeFormat)
	_, err := dm.Client.Diverts(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
	return err
}

func (dm *DivertManager) update(ctx context.Context, d *Divert) error {
	if d.Annotations == nil {
		d.Annotations = make(map[string]string)
	}
	d.Annotations[constants.LastUpdatedAnnotation] = time.Now().UTC().Format(constants.TimeFormat)
	_, err := dm.Client.Diverts(d.Namespace).Update(ctx, d)
	return err
}
