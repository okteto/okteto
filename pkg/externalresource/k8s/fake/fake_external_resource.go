package fake

import (
	"context"

	k8sexternalresource "github.com/okteto/okteto/pkg/externalresource/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/testing"
)

// FakeExternalResource implements ExternalResourceInterface
type FakeExternalResource struct {
	Fake                         *FakeExternalResourceV1
	ns                           string
	getErr, createErr, updateErr error
}

var externalResourceResource = schema.GroupVersionResource{Group: k8sexternalresource.GroupName, Version: k8sexternalresource.GroupVersion, Resource: k8sexternalresource.ExternalResourceResource}

var externalResourceKind = schema.GroupVersionKind{Group: k8sexternalresource.GroupName, Version: k8sexternalresource.GroupVersion, Kind: k8sexternalresource.ExternalResourceKind}

func (c *FakeExternalResource) Create(_ context.Context, external *k8sexternalresource.External) (*k8sexternalresource.External, error) {
	if c.createErr != nil {
		return nil, c.createErr
	}

	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(externalResourceResource, c.ns, external), &k8sexternalresource.External{})

	if obj == nil {
		return nil, err
	}
	return obj.(*k8sexternalresource.External), err
}

func (c *FakeExternalResource) Update(_ context.Context, external *k8sexternalresource.External) (*k8sexternalresource.External, error) {
	if c.updateErr != nil {
		return nil, c.updateErr
	}

	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(externalResourceResource, c.ns, external), &k8sexternalresource.External{})

	if obj == nil {
		return nil, err
	}
	return obj.(*k8sexternalresource.External), err
}

func (c *FakeExternalResource) Get(_ context.Context, name string, _ metav1.GetOptions) (*k8sexternalresource.External, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}

	obj, err := c.Fake.
		Invokes(testing.NewGetAction(externalResourceResource, c.ns, name), &k8sexternalresource.External{})

	if obj == nil {
		return &k8sexternalresource.External{}, err
	}
	return obj.(*k8sexternalresource.External), err
}
