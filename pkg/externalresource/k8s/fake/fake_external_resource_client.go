package fake

import (
	k8sexternalresource "github.com/okteto/okteto/pkg/externalresource/k8s"
	oktetoLog "github.com/okteto/okteto/pkg/log"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/testing"
)

type FakeExternalResourceV1 struct {
	possibleErrs PossibleERErrors
	tracker      testing.ObjectTracker
	testing.Fake
}

type PossibleERErrors struct {
	GetErr, UpdateErr, CreateErr, ListErr error
}

func NewFakeExternalResourceV1(errs PossibleERErrors, objects ...runtime.Object) *FakeExternalResourceV1 {
	scheme := runtime.NewScheme()
	err := k8sexternalresource.AddToScheme(scheme)
	if err != nil {
		oktetoLog.Infof("error adding externalresource scheme: %s", err)

	}
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
	return &FakeExternalResource{
		Fake:      c,
		ns:        namespace,
		getErr:    c.possibleErrs.GetErr,
		createErr: c.possibleErrs.CreateErr,
		updateErr: c.possibleErrs.UpdateErr,
		listErr:   c.possibleErrs.ListErr,
	}
}
