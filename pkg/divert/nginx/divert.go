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

package nginx

import (
	"context"
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/divert/k8s"
	"github.com/okteto/okteto/pkg/format"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	divertCRDMaxLength = 253
)

// DivertManager interface defines the methods for managing divert resources
type DivertManager interface {
	CreateOrUpdate(ctx context.Context, d *k8s.Divert) error
	List(ctx context.Context, namespace string) ([]*k8s.Divert, error)
	Delete(ctx context.Context, name, namespace string) error
}

// Driver nginx struct for the divert driver
type Driver struct {
	client        kubernetes.Interface
	cache         *cache
	name          string
	namespace     string
	divert        model.DivertDeploy
	divertManager DivertManager
}

func New(divert *model.DivertDeploy, name, namespace string, c kubernetes.Interface, divertManager DivertManager) *Driver {
	return &Driver{
		name:          name,
		namespace:     namespace,
		divert:        *divert,
		client:        c,
		divertManager: divertManager,
	}
}

func (d *Driver) Deploy(ctx context.Context) error {
	oktetoLog.Spinner(fmt.Sprintf("Diverting namespace %s...", d.divert.Namespace))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()
	if err := d.initCache(ctx); err != nil {
		return err
	}

	if err := d.deployDivertResources(ctx); err != nil {
		return err
	}

	if err := d.deleteOrphanDiverts(ctx); err != nil {
		return fmt.Errorf("error deleting orphan diverts resources: %w", err)
	}

	for name, in := range d.cache.divertIngresses {
		select {
		case <-ctx.Done():
			oktetoLog.Infof("deployDivert context cancelled")
			return ctx.Err()
		default:
			oktetoLog.Spinner(fmt.Sprintf("Diverting ingress %s/%s...", in.Namespace, in.Name))
			oktetoLog.StartSpinner()
			defer oktetoLog.StopSpinner()
			if err := d.divertIngress(ctx, name); err != nil {
				return err
			}
			oktetoLog.StopSpinner()
			oktetoLog.Success("Ingress '%s/%s' successfully diverted", in.Namespace, in.Name)
		}
	}
	return nil
}

// Destroy implements from the interface diver.Driver
// nolint:unparam
func (d *Driver) Destroy(_ context.Context) error {
	oktetoLog.Success("Divert from '%s' successfully destroyed", d.divert.Namespace)
	return nil
}

func (d *Driver) UpdatePod(pod apiv1.PodSpec) apiv1.PodSpec {
	if pod.DNSConfig == nil {
		pod.DNSConfig = &apiv1.PodDNSConfig{}
	}
	if pod.DNSConfig.Searches == nil {
		pod.DNSConfig.Searches = []string{}
	}
	searches := []string{fmt.Sprintf("%s.svc.cluster.local", d.divert.Namespace)}
	searches = append(searches, pod.DNSConfig.Searches...)
	pod.DNSConfig.Searches = searches

	// Add or update environment variables for all containers
	for i := range pod.InitContainers {
		updateEnvVar(&pod.InitContainers[i].Env, constants.OktetoSharedEnvironmentEnvVar, d.divert.Namespace)
		updateEnvVar(&pod.InitContainers[i].Env, constants.OktetoDivertedEnvironmentEnvVar, d.namespace)
	}

	for i := range pod.Containers {
		updateEnvVar(&pod.Containers[i].Env, constants.OktetoSharedEnvironmentEnvVar, d.divert.Namespace)
		updateEnvVar(&pod.Containers[i].Env, constants.OktetoDivertedEnvironmentEnvVar, d.namespace)
	}

	return pod
}

func (d *Driver) UpdateVirtualService(_ *istioNetworkingV1beta1.VirtualService) {}

// updateEnvVar adds or updates an environment variable in the given env var slice
func updateEnvVar(envVars *[]apiv1.EnvVar, name, value string) {
	for i := range *envVars {
		if (*envVars)[i].Name == name {
			(*envVars)[i].Value = value
			return
		}
	}
	*envVars = append(*envVars, apiv1.EnvVar{
		Name:  name,
		Value: value,
	})
}

func (d *Driver) deployDivertResources(ctx context.Context) error {
	for _, svc := range d.cache.developerServices {
		select {
		case <-ctx.Done():
			oktetoLog.Infof("deployDivert context cancelled")
			return ctx.Err()
		default:
			// If the service doesn't have the divert namespace annotation, we don't have to create or update
			// the divert resource as they are just a copy from the base namespace
			if svc.Annotations[model.OktetoDivertedNamespaceAnnotation] != "" {
				continue
			}

			name := fmt.Sprintf("%s-%s", format.ResourceK8sMetaString(d.name), svc.Name)
			if len(name) > divertCRDMaxLength {
				name = name[:divertCRDMaxLength]
				name = strings.TrimSuffix(name, "-")
			}
			div := &k8s.Divert{
				TypeMeta: metav1.TypeMeta{
					Kind:       k8s.DivertKind,
					APIVersion: fmt.Sprintf("%s/%s", k8s.GroupName, k8s.GroupVersion),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: d.namespace,
					Labels: map[string]string{
						model.DeployedByLabel: format.ResourceK8sMetaString(d.name),
					},
				},
				Spec: k8s.DivertSpec{
					Service:         svc.Name,
					SharedNamespace: d.divert.Namespace,
					DivertKey:       d.namespace,
				},
			}

			// As divert resources are created or updated only here, and they are scoped to the developer namespace
			// we don't handle a conflict error to retry the change. If that situation changes, we should add
			// a retry mechanism to handle conflicts.
			oktetoLog.Debugf("creating or updating divert resource for service '%s/%s'", svc.Namespace, svc.Name)
			if err := d.divertManager.CreateOrUpdate(ctx, div); err != nil {
				oktetoLog.Infof("error creating or updating divert resource '%s/%s': %s", div.Namespace, div.Name, err)
				return fmt.Errorf("error diverting service %s/%s: %w", div.Namespace, div.Name, err)
			}
			oktetoLog.Debugf("Divert resource '%s/%s' created or updated", div.Namespace, div.Name)
		}
	}

	return nil
}

// deleteOrphanDiverts checks for divert resources that are not associated with any developer service, and deletes them.
func (d *Driver) deleteOrphanDiverts(ctx context.Context) error {
	divertToDelete := make([]string, 0)
	for _, div := range d.cache.divertResources {
		if _, ok := d.cache.developerServices[div.Spec.Service]; ok {
			continue
		}

		divertToDelete = append(divertToDelete, div.Name)
	}

	for _, name := range divertToDelete {
		if err := d.divertManager.Delete(ctx, name, d.namespace); err != nil {
			oktetoLog.Infof("failed to delete divert '%s/%s': %s", d.namespace, name, err.Error())
			return err
		}

		oktetoLog.Debugf("deleted divert '%s/%s'", d.namespace, name)
	}

	return nil
}
