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

package volumes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/okteto/okteto/cmd/utils"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// List returns the list of volumes
func List(ctx context.Context, namespace, labels string, c kubernetes.Interface) ([]apiv1.PersistentVolumeClaim, error) {
	vList, err := c.CoreV1().PersistentVolumeClaims(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels,
		},
	)
	if err != nil {
		return nil, err
	}
	return vList.Items, nil
}

// CreateForDev deploys the volume claim for a given development container
func CreateForDev(ctx context.Context, dev *model.Dev, c kubernetes.Interface, devPath string) error {
	vClient := c.CoreV1().PersistentVolumeClaims(dev.Namespace)
	pvcForDev := translate(dev)
	k8Volume, err := vClient.Get(ctx, pvcForDev.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error getting kubernetes volume claim: %w", err)
	}
	if k8Volume.Name == "" {
		oktetoLog.Infof("creating volume claim '%s'", pvcForDev.Name)
		_, err = vClient.Create(ctx, pvcForDev, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating kubernetes volume claim: %w", err)
		}
	} else {
		if err := checkPVCValues(k8Volume, dev, devPath); err != nil {
			return err
		}
		oktetoLog.Infof("updating volume claim '%s'", pvcForDev.Name)
		if pvcForDev.Spec.StorageClassName == nil {
			pvcForDev.Spec.StorageClassName = k8Volume.Spec.StorageClassName
		}
		pvcForDev.Spec.VolumeName = k8Volume.Spec.VolumeName
		_, err = vClient.Update(ctx, pvcForDev, metav1.UpdateOptions{})
		if err != nil {
			if !isDynamicallyProvisionedPVCError(err, pvcForDev.Name) {
				return fmt.Errorf("error updating kubernetes volume claim: %w", err)
			}
			oktetoLog.Debug("could not update pvc in namespace %s: %w", dev.Namespace, err)
			oktetoLog.Warning(`Could not increase the size of the dev volume from %s to %s:
try running 'okteto down -v' and 'okteto up', or talk to your administrator
(the PVC's storage class must support 'allowVolumeExpansion' to be able to upscale dev volumes).`,
				k8Volume.Spec.Resources.Requests[apiv1.ResourceStorage], pvcForDev.Spec.Resources.Requests[apiv1.ResourceStorage])
		}
	}
	return nil
}

func isDynamicallyProvisionedPVCError(err error, pvcName string) bool {
	errorString := fmt.Sprintf("persistentvolumeclaims \"%s\" is forbidden: only dynamically provisioned pvc can be resized and the storageclass that provisions the pvc must support resize", pvcName)
	return strings.Contains(err.Error(), errorString)
}

func Create(ctx context.Context, pvc *apiv1.PersistentVolumeClaim, c kubernetes.Interface) error {
	_, err := c.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func Update(ctx context.Context, pvc *apiv1.PersistentVolumeClaim, c kubernetes.Interface) error {
	_, err := c.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(ctx, pvc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func checkPVCValues(pvc *apiv1.PersistentVolumeClaim, dev *model.Dev, devPath string) error {
	currentSize, ok := pvc.Spec.Resources.Requests["storage"]
	if !ok {
		return fmt.Errorf("current okteto volume size is wrong. Run '%s' and try again", utils.GetDownCommand(devPath))
	}
	devPVSize, err := resource.ParseQuantity(dev.PersistentVolumeSize())
	if err != nil {
		return fmt.Errorf("error parsing dev volume size %q: %w", dev.PersistentVolumeSize(), err)
	}
	if currentSize.Cmp(devPVSize) > 0 {
		if currentSize.Cmp(resource.MustParse("10Gi")) != 0 || dev.HasDefaultPersistentVolumeSize() {
			return fmt.Errorf(
				"okteto volume size '%s' cannot be less than previous value '%s'. Run '%s' and try again",
				dev.PersistentVolumeSize(),
				currentSize.String(),
				utils.GetDownCommand(devPath),
			)
		}
	}
	if currentSize.Cmp(devPVSize) < 0 {
		restartUUID := uuid.New().String()
		if dev.Metadata == nil {
			dev.Metadata = &model.Metadata{}
		}
		if dev.Metadata.Annotations == nil {
			dev.Metadata.Annotations = map[string]string{}
		}
		dev.Metadata.Annotations[model.OktetoRestartAnnotation] = restartUUID
		for _, s := range dev.Services {
			if s.Annotations == nil {
				s.Annotations = map[string]string{}
			}
			s.Annotations[model.OktetoRestartAnnotation] = restartUUID
		}
	}

	if dev.PersistentVolumeStorageClass() != "" {
		if pvc.Spec.StorageClassName == nil {
			return fmt.Errorf(
				"okteto volume storageclass is '' instead of '%s'. Run '%s' and try again",
				dev.PersistentVolumeStorageClass(),
				utils.GetDownCommand(devPath),
			)
		} else if dev.PersistentVolumeStorageClass() != *pvc.Spec.StorageClassName {
			return fmt.Errorf(
				"okteto volume storageclass cannot be updated from '%s' to '%s'. Run '%s' and try again",
				*pvc.Spec.StorageClassName,
				dev.PersistentVolumeStorageClass(),
				utils.GetDownCommand(devPath),
			)
		}
	}
	return nil

}

// DestroyDev destroys the persistent volume claim for a given development container
func DestroyDev(ctx context.Context, dev *model.Dev, c kubernetes.Interface) error {
	return Destroy(ctx, dev.GetVolumeName(), dev.Namespace, c, dev.Timeout.Default)
}

// Destroy destroys a persistent volume claim
func Destroy(ctx context.Context, name, namespace string, c kubernetes.Interface, timeout time.Duration) error {
	vClient := c.CoreV1().PersistentVolumeClaims(namespace)
	oktetoLog.Infof("destroying volume '%s'", name)

	ticker := time.NewTicker(1 * time.Second)
	timeoutDuration := 3 * timeout
	to := time.Now().Add(timeoutDuration) // 90 seconds
	logDebounceInterval := 5

	for i := 0; ; i++ {
		err := vClient.Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			if oktetoErrors.IsNotFound(err) {
				oktetoLog.Infof("volume '%s' successfully destroyed", name)
				return nil
			}

			return fmt.Errorf("error deleting kubernetes volume: %s", err)
		}

		if time.Now().After(to) {
			if err := checkIfAttached(ctx, name, namespace, c); err != nil {
				return err
			}

			return fmt.Errorf("volume claim '%s' wasn't destroyed after %s", name, timeout.String())
		}

		if i%10 == logDebounceInterval {
			oktetoLog.Infof("waiting for volume '%s' to be destroyed", name)
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			oktetoLog.Info("call to volumes.Destroy cancelled")
			return ctx.Err()
		}
	}

}

// DestroyWithoutTimeout destroys a PVC without checking if it was detached or not
func DestroyWithoutTimeout(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	vClient := c.CoreV1().PersistentVolumeClaims(namespace)
	oktetoLog.Infof("destroying volume '%s'", name)

	err := vClient.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return fmt.Errorf("error deleting kubernetes volume: %w", err)
		}
	}

	return nil
}

func checkIfAttached(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	pods, err := c.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		oktetoLog.Infof("failed to get available pods: %s", err)
		return nil
	}

	for i := range pods.Items {
		for j := range pods.Items[i].Spec.Volumes {
			if pods.Items[i].Spec.Volumes[j].PersistentVolumeClaim != nil {
				if pods.Items[i].Spec.Volumes[j].PersistentVolumeClaim.ClaimName == name {
					oktetoLog.Infof("pvc/%s is still attached to pod/%s", name, pods.Items[i].Name)
					return fmt.Errorf("can't delete the volume '%s' since it's still attached to 'pod/%s'", name, pods.Items[i].Name)
				}
			}
		}
	}

	return nil
}
