package fake

import (
	k8sexternalresource "github.com/okteto/okteto/pkg/externalresource/k8s"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"
)

type FakeExternalResourceV1 struct {
	testing.Fake
	tracker      testing.ObjectTracker
	possibleErrs PossibleERErrors
}

type PossibleERErrors struct {
	GetErr, UpdateErr, CreateErr error
}

func NewFakeExternalResourceV1(errs PossibleERErrors, objects ...runtime.Object) *FakeExternalResourceV1 {
	scheme := runtime.NewScheme()
	_ = k8sexternalresource.AddToScheme(scheme)
	codecs := serializer.NewCodecFactory(scheme)

	o := testing.NewObjectTracker(scheme, codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &FakeExternalResourceV1{tracker: o, possibleErrs: errs}
	cs.AddReactor("*", "*", testing.ObjectReaction(o))
	cs.AddWatchReactor("*", func(action testing.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := o.Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})

	return cs
}

func (c *FakeExternalResourceV1) ExternalResources(namespace string) k8sexternalresource.ExternalResourceInterface {
	return &FakeExternalResource{c, namespace, c.possibleErrs.GetErr, c.possibleErrs.CreateErr, c.possibleErrs.UpdateErr}
}

func (c *FakeExternalResourceV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
