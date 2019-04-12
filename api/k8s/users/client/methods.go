package client

import (
	"github.com/okteto/app/api/k8s/users/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// UserInterface TBD
type UserInterface interface {
	List(opts metav1.ListOptions) (*v1alpha1.UserList, error)
	Get(name string, options metav1.GetOptions) (*v1alpha1.User, error)
	Create(*v1alpha1.User) (*v1alpha1.User, error)
	Update(*v1alpha1.User) (*v1alpha1.User, error)
	Delete(name string, options *metav1.DeleteOptions) error
}

type userClient struct {
	restClient rest.Interface
	ns         string
}

func (c *userClient) List(opts metav1.ListOptions) (*v1alpha1.UserList, error) {
	result := v1alpha1.UserList{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource("users").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)

	return &result, err
}

func (c *userClient) Get(name string, opts metav1.GetOptions) (*v1alpha1.User, error) {
	result := v1alpha1.User{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource("users").
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)

	return &result, err
}

func (c *userClient) Create(user *v1alpha1.User) (*v1alpha1.User, error) {
	result := v1alpha1.User{}
	err := c.restClient.
		Post().
		Namespace(c.ns).
		Resource("users").
		Body(user).
		Do().
		Into(&result)

	return &result, err
}

func (c *userClient) Update(user *v1alpha1.User) (*v1alpha1.User, error) {
	result := &v1alpha1.User{}
	err := c.restClient.
		Put().
		Namespace(c.ns).
		Resource("users").
		Name(user.Name).
		Body(user).
		Do().
		Into(result)
	return result, err
}

func (c *userClient) Delete(name string, options *metav1.DeleteOptions) error {
	return c.restClient.
		Delete().
		Namespace(c.ns).
		Resource("users").
		Name(name).
		Body(options).
		Do().
		Error()
}
