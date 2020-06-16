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

package services

import (
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//CreateDev deploys a default k8s service for a development container
func CreateDev(dev *model.Dev, c *kubernetes.Clientset) error {
	old, err := Get(dev.Namespace, dev.Name, c)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error getting kubernetes service: %s", err)
	}

	s := translate(dev)
	sClient := c.CoreV1().Services(dev.Namespace)

	if old.Name == "" {
		log.Infof("creating service '%s'", s.Name)
		_, err = sClient.Create(s)
		if err != nil {
			return fmt.Errorf("error creating kubernetes service: %s", err)
		}
		log.Infof("created service '%s'", s.Name)
	} else {
		log.Infof("updating service '%s'", s.Name)
		old.Spec.Ports = s.Spec.Ports
		_, err = sClient.Update(old)
		if err != nil {
			return fmt.Errorf("error updating kubernetes service: %s", err)
		}
		log.Infof("updated service '%s'.", s.Name)
	}
	return nil
}

//DestroyDev destroys the default service for a development container
func DestroyDev(dev *model.Dev, c *kubernetes.Clientset) error {
	log.Infof("deleting service '%s'", dev.Name)
	sClient := c.CoreV1().Services(dev.Namespace)
	err := sClient.Delete(dev.Name, &metav1.DeleteOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Infof("service '%s' was already deleted.", dev.Name)
			return nil
		}
		return fmt.Errorf("error deleting kubernetes service: %s", err)
	}
	log.Infof("service '%s' deleted", dev.Name)
	return nil
}

// Get returns a kubernetes service by the name, or an error if it doesn't exist
func Get(namespace, name string, c kubernetes.Interface) (*apiv1.Service, error) {
	return c.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
}

//GetPortsByPod returns the ports exposed via endpoint of a given pod
func GetPortsByPod(p *apiv1.Pod, c *kubernetes.Clientset) ([]int, error) {
	eList, err := c.CoreV1().Endpoints(p.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := []int{}
	for _, e := range eList.Items {
		for _, s := range e.Subsets {
			for _, a := range append(s.Addresses, s.NotReadyAddresses...) {
				if a.TargetRef.UID == p.UID {
					for _, p := range s.Ports {
						result = append(result, int(p.Port))
					}
					break
				}
			}
		}
	}
	return result, nil
}
