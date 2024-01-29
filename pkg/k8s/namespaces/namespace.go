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

package namespaces

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/pager"
)

const (
	parallelism              int64 = 10
	volumeKind                     = "PersistentVolumeClaim"
	volumeSnapshotKind             = "VolumeSnapshot"
	jobKind                        = "Job"
	resourcePolicyAnnotation       = "dev.okteto.com/policy"
	keepPolicy                     = "keep"
)

type Traveler interface {
	See(obj runtime.Object) error
}

type TravelerFunc func(obj runtime.Object) error

func (t TravelerFunc) See(obj runtime.Object) error {
	return t(obj)
}

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
	k8sClient  kubernetes.Interface
}

type Options struct {
	List        metav1.ListOptions
	Namespace   string
	Parallelism int64
}

type Trip struct {
	k8s        kubernetes.Interface
	dynamic    dynamic.Interface
	restConfig *rest.Config
	sem        *semaphore.Weighted
	list       metav1.ListOptions
	namespace  string
	writeLock  sync.Mutex
}

// noneRateLimiter noop rate limiter to avoid rate limiting errors on k8s client.
// We created this to avoid to depend on github.com/rancher/rancher dependency as this is the only place where it was used.
type noneRateLimiter struct{}

func (*noneRateLimiter) TryAccept() bool              { return true }
func (*noneRateLimiter) Stop()                        {}
func (*noneRateLimiter) Accept()                      {}
func (*noneRateLimiter) QPS() float32                 { return 1 }
func (*noneRateLimiter) Wait(_ context.Context) error { return nil }

// NewNamespace allows to create a new Namespace object
func NewNamespace(dynClient dynamic.Interface, discClient discovery.DiscoveryInterface, restConfig *rest.Config, k8s kubernetes.Interface) *Namespaces {
	return &Namespaces{
		dynClient:  dynClient,
		discClient: discClient,
		restConfig: restConfig,
		k8sClient:  k8s,
	}
}

// DestroyWithLabel deletes all resources within a namespace
func (n *Namespaces) DestroyWithLabel(ctx context.Context, ns string, opts DeleteAllOptions) error {
	listOptions := metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
	}

	trip, err := newTrip(n.restConfig, &Options{
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

	return trip.wander(ctx, TravelerFunc(func(obj runtime.Object) error {
		m, err := meta.Accessor(obj)
		if err != nil {
			return err
		}
		gvk := obj.GetObjectKind().GroupVersionKind()
		if isStorage(gvk.Kind) && !opts.IncludeVolumes {
			oktetoLog.Debugf("skipping deletion of '%s' '%s' because of volume flag", gvk.Kind, m.GetName())
			return nil
		}

		if m.GetAnnotations()[resourcePolicyAnnotation] == keepPolicy {
			oktetoLog.Debugf("skipping deletion of %s '%s' because of policy annotation", gvk.Kind, m.GetName())
			return nil
		}

		mapping, err := rm.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
		if err != nil {
			return err
		}
		deleteOpts := metav1.DeleteOptions{}

		// It seems that by default, client-go doesn't delete pods scheduled by jobs, so we need to set the propagation policy
		if gvk.Kind == jobKind {
			deletePropagation := metav1.DeletePropagationBackground
			deleteOpts.PropagationPolicy = &deletePropagation
		}

		err = n.dynClient.
			Resource(mapping.Resource).
			Namespace(ns).
			Delete(ctx, m.GetName(), deleteOpts)

		if err != nil {
			oktetoLog.Debugf("error deleting '%s' '%s': %s", gvk.Kind, m.GetName(), err)
			return err
		}

		oktetoLog.Debugf("successfully deleted '%s' '%s'", gvk.Kind, m.GetName())
		return nil
	}))
}

// DestroySFSVolumes This function deletes volumes for any statefulset that matches with opts.LabelSelector but it doesn't have any
// dev.okteto.com/deployed-by label. This is to avoid to left PVCs behind when everything deployed with okteto deploy
// command is deleted
func (n *Namespaces) DestroySFSVolumes(ctx context.Context, ns string, opts DeleteAllOptions) error {
	if !opts.IncludeVolumes {
		return nil
	}
	var pvcNames []string

	ssList, err := statefulsets.List(ctx, ns, opts.LabelSelector, n.k8sClient)
	if err != nil {
		return fmt.Errorf("error getting statefulsets: %w", err)
	}
	for _, ss := range ssList {
		for _, pvcTemplate := range ss.Spec.VolumeClaimTemplates {
			pvcNames = append(pvcNames, fmt.Sprintf("%s-%s-", pvcTemplate.Name, ss.Name))
		}
	}

	if len(pvcNames) == 0 {
		return nil
	}

	// We only need to delete all the volumes without deployed-by label. The ones with the label will be deleted by
	// DestroyWithLabel function
	deployedByNotExist, err := labels.NewRequirement(
		model.DeployedByLabel,
		selection.DoesNotExist,
		nil,
	)
	if err != nil {
		return err
	}
	deployedByNotExistSelector := labels.NewSelector().Add(*deployedByNotExist).String()
	vList, err := volumes.List(ctx, ns, deployedByNotExistSelector, n.k8sClient)
	if err != nil {
		return fmt.Errorf("error getting volumes: %w", err)
	}
	for _, v := range vList {
		if v.Annotations[resourcePolicyAnnotation] == keepPolicy {
			oktetoLog.Debugf("skipping deletion of pvc '%s' because of policy annotation", v.GetName())
			continue
		}
		for _, pvcName := range pvcNames {
			if strings.HasPrefix(v.Name, pvcName) {
				if err := volumes.DestroyWithoutTimeout(ctx, v.Name, ns, n.k8sClient); err != nil {
					return err
				}
				break
			}
		}
	}

	return nil
}

// Below functions were added to remove "github.com/ibuildthecloud/finalizers" as a dependency
func newTrip(restConfig *rest.Config, opts *Options) (*Trip, error) {
	if opts == nil {
		opts = &Options{}
	}

	if restConfig.RateLimiter == nil {
		restConfig.RateLimiter = &noneRateLimiter{}
	}

	k8s, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	dynamic, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	defaultMaxWeight := int64(10)

	var sem *semaphore.Weighted
	if opts.Parallelism > 0 {
		sem = semaphore.NewWeighted(opts.Parallelism)
	} else {
		sem = semaphore.NewWeighted(defaultMaxWeight)
	}

	return &Trip{
		namespace:  opts.Namespace,
		list:       opts.List,
		restConfig: restConfig,
		k8s:        k8s,
		dynamic:    dynamic,
		sem:        sem,
	}, nil
}

func (t *Trip) wander(ctx context.Context, traveler Traveler) error {
	_, apis, err := t.k8s.Discovery().ServerGroupsAndResources()
	if err != nil {
		return err
	}

	eg, _ := errgroup.WithContext(ctx)
	for _, api := range apis {
		for _, api2 := range api.APIResources {
			listable := false
			for _, verb := range api2.Verbs {
				if verb == "list" {
					listable = true
					break
				}
			}
			if !listable {
				continue
			}

			if t.namespace != "" && !api2.Namespaced {
				continue
			}

			gvr := schema.FromAPIVersionAndKind(api.GroupVersion, api2.Kind).
				GroupVersion().
				WithResource(api2.Name)

			var client dynamic.ResourceInterface
			if t.namespace == "" {
				client = t.dynamic.Resource(gvr)
			} else {
				client = t.dynamic.Resource(gvr).Namespace(t.namespace)
			}

			// capture values for goroutine
			api := api
			api2 := api2
			eg.Go(func() error {
				if err := t.listAll(ctx, traveler, client, api, api2); err != nil {
					logrus.Warn("Failed to list", api.GroupVersion, api2.Kind, err)
				}
				return nil
			})
		}
	}

	return eg.Wait()
}

func (t *Trip) listAll(ctx context.Context, traveler Traveler, client dynamic.ResourceInterface, api *metav1.APIResourceList, api2 metav1.APIResource) error {
	if err := t.sem.Acquire(ctx, 1); err != nil {
		return err
	}
	defer t.sem.Release(1)

	pager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
		objs, err := client.List(ctx, t.list)
		return objs, err
	}))

	return pager.EachListItem(ctx, t.list, func(obj runtime.Object) error {
		t.writeLock.Lock()
		defer t.writeLock.Unlock()
		if err := traveler.See(obj); err != nil {
			m, err2 := meta.Accessor(obj)
			if err2 == nil {
				logrus.Warn("Failed to process", api.GroupVersion, api2.Kind, m.GetNamespace(), m.GetName(), err)
			}
		}
		return nil
	})
}

// isStorage returns if the kind is some of storage kind
func isStorage(kind string) bool {
	switch kind {
	case volumeKind, volumeSnapshotKind:
		return true
	}
	return false
}
