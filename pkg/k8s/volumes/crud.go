// Copyright 2020 The Okteto Authors
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

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

//Create deploys the volume claim for a given development container
func Create(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) error {
	vClient := c.CoreV1().PersistentVolumeClaims(dev.Namespace)
	pvc := translate(dev)
	k8Volume, err := vClient.Get(pvc.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error getting kubernetes volume claim: %s", err)
	}
	if k8Volume.Name != "" {
		return checkPVCValues(k8Volume, dev)
	}
	log.Infof("creating volume claim '%s'", pvc.Name)
	_, err = vClient.Create(pvc)
	if err != nil {
		return fmt.Errorf("error creating kubernetes volume claim: %s", err)
	}
	return nil
}

func checkPVCValues(pvc *apiv1.PersistentVolumeClaim, dev *model.Dev) error {
	currentSize, ok := pvc.Spec.Resources.Requests["storage"]
	if !ok {
		return fmt.Errorf("current okteto volume size is wrong. Run 'okteto down -v' and try again")
	}
	if currentSize.Cmp(resource.MustParse(dev.PersistentVolumeSize())) != 0 {
		if currentSize.Cmp(resource.MustParse("10Gi")) != 0 || dev.PersistentVolumeSize() != model.OktetoDefaultPVSize {
			return fmt.Errorf(
				"current okteto volume size is '%s' instead of '%s'. Run 'okteto down -v' and try again",
				currentSize.String(),
				dev.PersistentVolumeSize(),
			)
		}
	}
	if dev.PersistentVolumeStorageClass() != "" {
		if pvc.Spec.StorageClassName == nil {
			return fmt.Errorf(
				"current okteto volume storageclass is '' instead of '%s'. Run 'okteto down -v' and try again",
				dev.PersistentVolumeStorageClass(),
			)
		} else if dev.PersistentVolumeStorageClass() != *pvc.Spec.StorageClassName {
			return fmt.Errorf(
				"current okteto volume storageclass is '%s' instead of '%s'. Run 'okteto down -v' and try again",
				*pvc.Spec.StorageClassName,
				dev.PersistentVolumeStorageClass(),
			)
		}
	}
	return nil

}

//Destroy destroys the volume claim for a given development container
func Destroy(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) error {
	vClient := c.CoreV1().PersistentVolumeClaims(dev.Namespace)
	log.Infof("destroying volume claim '%s'", dev.GetVolumeName())

	ticker := time.NewTicker(1 * time.Second)
	to := 3 * config.GetTimeout() // 90 seconds
	timeout := time.Now().Add(to)

	for i := 0; ; i++ {
		err := vClient.Delete(dev.GetVolumeName(), &metav1.DeleteOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Infof("volume claim '%s' successfully destroyed", dev.GetVolumeName())
				return nil
			}

			return fmt.Errorf("error getting kubernetes volume claim: %s", err)
		}

		if time.Now().After(timeout) {
			if err := checkIfAttached(dev, c); err != nil {
				return err
			}

			return fmt.Errorf("volume claim '%s' wasn't destroyed after %s", dev.GetVolumeName(), to.String())
		}

		if i%10 == 5 {
			log.Infof("waiting for volume claim '%s' to be destroyed", dev.GetVolumeName())
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			log.Debug("cancelling call to update okteto revision")
			return ctx.Err()
		}
	}

}

func checkIfAttached(dev *model.Dev, c *kubernetes.Clientset) error {
	pods, err := c.CoreV1().Pods(dev.Namespace).List(metav1.ListOptions{})
	if err != nil {
		log.Infof("failed to get available pods: %s", err)
		return nil
	}

	for i := range pods.Items {
		for j := range pods.Items[i].Spec.Volumes {
			if pods.Items[i].Spec.Volumes[j].PersistentVolumeClaim != nil {
				if pods.Items[i].Spec.Volumes[j].PersistentVolumeClaim.ClaimName == dev.GetVolumeName() {
					log.Infof("pvc/%s is still attached to pod/%s", dev.GetVolumeName(), pods.Items[i].Name)
					return fmt.Errorf("can't delete your volume claim since it's still attached to 'pod/%s'", pods.Items[i].Name)
				}
			}
		}
	}

	return nil
}
