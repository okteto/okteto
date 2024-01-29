// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fake

import (
	k8sexternalresource "github.com/okteto/okteto/pkg/externalresource/k8s"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/testing"
)

type V1 struct {
	possibleErrs PossibleERErrors
	tracker      testing.ObjectTracker
	testing.Fake
}

type PossibleERErrors struct {
	GetErr, UpdateErr, CreateErr, ListErr error
}

func NewFakeExternalResourceV1(errs PossibleERErrors, objects ...runtime.Object) *V1 {
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

	cs := &V1{tracker: o, possibleErrs: errs}
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

func (c *V1) ExternalResources(namespace string) k8sexternalresource.ExternalResourceInterface {
	return &ExternalResource{
		Fake:      c,
		ns:        namespace,
		getErr:    c.possibleErrs.GetErr,
		createErr: c.possibleErrs.CreateErr,
		updateErr: c.possibleErrs.UpdateErr,
		listErr:   c.possibleErrs.ListErr,
	}
}
