package namespaces

import (
	"context"

	"github.com/ibuildthecloud/finalizers/pkg/world"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

const parallelism int64 = 10

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
func (n *Namespaces) DestroyWithLabel(ctx context.Context, ns, labelSelector string) error {
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

	return trip.Wander(ctx, world.TravelerFunc(func(obj runtime.Object) error {
		m, err := meta.Accessor(obj)
		if err != nil {
			return err
		}
		gvk := obj.GetObjectKind().GroupVersionKind()
		mapping, err := rm.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
		if err != nil {
			return err
		}
		/*log.Information("resource mapped '%s' from %s/%s", mapping.Resource.Resource, obj.GetObjectKind().GroupVersionKind().Group, obj.GetObjectKind().GroupVersionKind().Kind)
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Namespace: ns,
					Verb:      "destroy",
					Group:     obj.GetObjectKind().GroupVersionKind().Group,
					Version:   obj.GetObjectKind().GroupVersionKind().Version,
					Resource:  mapping.Resource.Resource,
				},
			},
		}
		resp, err := n.k8sClient.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			log.Information("something was wrong creating the subject access review: %s", err)
			return err
		}

		b, err := json.Marshal(resp)
		if err != nil {
			log.Information("could not marshal th response")
			return err
		}

		log.Information("Resource: %s/%s/%s", gvk.Group, gvk.Version, gvk.Kind)
		log.Information("response #####: %s", b)*/
		deleteOpts := metav1.DeleteOptions{}
		return n.dynClient.
			Resource(mapping.Resource).
			Namespace(ns).
			Delete(ctx, m.GetName(), deleteOpts)
	}))
}
