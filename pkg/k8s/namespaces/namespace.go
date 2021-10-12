package namespaces

import (
	"context"

	"github.com/ibuildthecloud/finalizers/pkg/world"
	"github.com/okteto/okteto/pkg/log"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

const (
	parallelism int64 = 10
	volumeKind        = "PersistentVolumeClaim"
)

// Namespaces struct to interact with namespaces in k8s
type Namespaces struct {
	dynClient  dynamic.Interface
	discClient discovery.DiscoveryInterface
	restConfig *rest.Config
}

// NewNamespace allows to create a new Namespace object
func NewNamespace(dynClient dynamic.Interface, discClient discovery.DiscoveryInterface, restConfig *rest.Config) *Namespaces {
	return &Namespaces{
		dynClient:  dynClient,
		discClient: discClient,
		restConfig: restConfig,
	}
}

// DestroyWithLabel deletes all resources within a namespace
func (n *Namespaces) DestroyWithLabel(ctx context.Context, ns, labelSelector string, includeVolumes bool) error {
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	trip, err := world.NewTrip(n.restConfig, &world.Options{
		Namespace:   ns,
		Parallelism: parallelism,
		List:        listOptions,
	})
	if err != nil {
		return err
	}

	groupResources, err := restmapper.GetAPIGroupResources(n.discClient)
	if err != nil {
		return err
	}

	rm := restmapper.NewDiscoveryRESTMapper(groupResources)

	// This is done because the wander function will try to list all k8s resources and most of them cannot be listed by
	// Okteto user's service accounts, so it prints tons of warnings in the standard output. Setting the logrus level to err avoid those warnings
	prevLevel := logrus.GetLevel()
	logrus.SetLevel(logrus.ErrorLevel)
	defer func() {
		logrus.SetLevel(prevLevel)
	}()

	return trip.Wander(ctx, world.TravelerFunc(func(obj runtime.Object) error {
		m, err := meta.Accessor(obj)
		if err != nil {
			return err
		}
		gvk := obj.GetObjectKind().GroupVersionKind()
		if !includeVolumes && gvk.Kind == volumeKind {
			log.Debugf("skipping deletion of pvc '%s'", m.GetName())
			return nil
		}
		mapping, err := rm.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
		if err != nil {
			return err
		}
		deleteOpts := metav1.DeleteOptions{}
		return n.dynClient.
			Resource(mapping.Resource).
			Namespace(ns).
			Delete(ctx, m.GetName(), deleteOpts)
	}))
}
