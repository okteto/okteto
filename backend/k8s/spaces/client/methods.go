package client

import (
	"github.com/okteto/app/backend/k8s/spaces/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// SpaceInterface TBD
type SpaceInterface interface {
	List(opts metav1.ListOptions) (*v1alpha1.SpaceList, error)
	Get(name string, options metav1.GetOptions) (*v1alpha1.Space, error)
	Create(*v1alpha1.Space) (*v1alpha1.Space, error)
	Update(*v1alpha1.Space) (*v1alpha1.Space, error)
	Delete(name string, options *metav1.DeleteOptions) error
}

type spaceClient struct {
	restClient rest.Interface
	ns         string
}

func (c *spaceClient) List(opts metav1.ListOptions) (*v1alpha1.SpaceList, error) {
	result := v1alpha1.SpaceList{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource("spaces").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)

	return &result, err
}

func (c *spaceClient) Get(name string, opts metav1.GetOptions) (*v1alpha1.Space, error) {
	result := v1alpha1.Space{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource("spaces").
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)

	return &result, err
}

func (c *spaceClient) Create(space *v1alpha1.Space) (*v1alpha1.Space, error) {
	result := v1alpha1.Space{}
	err := c.restClient.
		Post().
		Namespace(c.ns).
		Resource("spaces").
		Body(space).
		Do().
		Into(&result)

	return &result, err
}

// Update takes the representation of a space and updates it
func (c *spaceClient) Update(space *v1alpha1.Space) (*v1alpha1.Space, error) {
	result := &v1alpha1.Space{}
	err := c.restClient.
		Put().
		Namespace(c.ns).
		Resource("spaces").
		Name(space.Name).
		Body(space).
		Do().
		Into(result)
	return result, err
}

// Delete takes name of the space and deletes it.
func (c *spaceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return c.restClient.
		Delete().
		Namespace(c.ns).
		Resource("spaces").
		Name(name).
		Body(options).
		Do().
		Error()
}
