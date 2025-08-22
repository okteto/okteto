// Copyright 2025 The Okteto Authors
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
	k8sdivert "github.com/okteto/okteto/pkg/divert/k8s"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/testing"
)

type V1 struct {
	possibleErrs PossibleDivertErrors
	tracker      testing.ObjectTracker
	testing.Fake
}

type PossibleDivertErrors struct {
	GetErr, UpdateErr, CreateErr, DeleteErr, ListErr error
}

func NewFakeDivertV1(errs PossibleDivertErrors, objects ...runtime.Object) *V1 {
	scheme := runtime.NewScheme()
	err := k8sdivert.AddToScheme(scheme)
	if err != nil {
		oktetoLog.Infof("error adding divert scheme: %s", err)
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

func (c *V1) Diverts(namespace string) k8sdivert.DivertInterface {
	return &Divert{
		Fake:      c,
		ns:        namespace,
		getErr:    c.possibleErrs.GetErr,
		createErr: c.possibleErrs.CreateErr,
		updateErr: c.possibleErrs.UpdateErr,
		deleteErr: c.possibleErrs.DeleteErr,
		listErr:   c.possibleErrs.ListErr,
	}
}
