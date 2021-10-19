// Copyright 2021 The Okteto Authors
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

// DeleteAllOptions options for delete all operation
type DeleteAllOptions struct {
	// LabelSelector selector for resources to be deleted
	LabelSelector string
	// IncludeVolumes flag to indicate if volumes have to be deleted or not
	IncludeVolumes bool
}

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
func (n *Namespaces) DestroyWithLabel(ctx context.Context, ns string, opts DeleteAllOptions) error {
	listOptions := metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
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
		if !opts.IncludeVolumes && gvk.Kind == volumeKind {
			log.Debugf("skipping deletion of pvc '%s'", m.GetName())
			return nil
		}
		mapping, err := rm.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
		if err != nil {
			return err
		}
		deleteOpts := metav1.DeleteOptions{}
		err = n.dynClient.
			Resource(mapping.Resource).
			Namespace(ns).
			Delete(ctx, m.GetName(), deleteOpts)

		if err != nil {
			log.Debugf("error deleting '%s' '%s': %s", gvk.Kind, m.GetName(), err)
			return err
		}

		log.Debugf("successfully deleted '%s' '%s'", gvk.Kind, m.GetName())
		return nil
	}))
}
