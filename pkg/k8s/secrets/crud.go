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

package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/syncthing"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	oktetoSecretTemplate = "okteto-%s"
)

// Secrets struct to handle secrets on k8s
type Secrets struct {
	k8sClient kubernetes.Interface
}

// NewSecrets creates a new Secrets object
func NewSecrets(k8sClient kubernetes.Interface) *Secrets {
	return &Secrets{
		k8sClient: k8sClient,
	}
}

// Get returns the value of a secret
func Get(ctx context.Context, name, namespace string, c kubernetes.Interface) (*v1.Secret, error) {
	secret, err := c.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return secret, fmt.Errorf("error getting kubernetes secret: %s", err)
	}
	return secret, nil
}

// Create creates the syncthing config secret
func Create(ctx context.Context, dev *model.Dev, c kubernetes.Interface, s *syncthing.Syncthing) error {
	secretName := GetSecretName(dev)

	sct, err := Get(ctx, secretName, dev.Namespace, c)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error getting kubernetes secret: %s", err)
	}

	config, err := getConfigXML(s)
	if err != nil {
		return fmt.Errorf("error generating syncthing configuration: %s", err)
	}
	data := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				constants.DevLabel: "true",
			},
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			"config.xml": config,
			"cert.pem":   []byte(certPEM),
			"key.pem":    []byte(keyPEM),
		},
	}

	idx := 0
	for _, s := range dev.Secrets {
		content, err := os.ReadFile(s.LocalPath)
		if err != nil {
			return fmt.Errorf("error reading secret '%s': %s", s.LocalPath, err)
		}
		if strings.Contains(s.GetKeyName(), "stignore") {
			idx++
			data.Data[fmt.Sprintf("%s-%d", s.GetKeyName(), idx)] = content
		} else {
			data.Data[s.GetKeyName()] = content
		}

	}

	if sct.Name == "" {
		_, err := c.CoreV1().Secrets(dev.Namespace).Create(ctx, data, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating kubernetes sync secret: %s", err)
		}

		oktetoLog.Infof("created okteto secret '%s'", secretName)
	} else {
		_, err := c.CoreV1().Secrets(dev.Namespace).Update(ctx, data, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating kubernetes okteto secret: %s", err)
		}
		oktetoLog.Infof("updated okteto secret '%s'", secretName)
	}
	return nil
}

// Destroy deletes the syncthing config secret
func Destroy(ctx context.Context, dev *model.Dev, c kubernetes.Interface) error {
	secretName := GetSecretName(dev)
	err := c.CoreV1().Secrets(dev.Namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("error deleting kubernetes okteto secret: %s", err)
	}
	return nil
}

// GetSecretName returns the okteto secret name for a given development container
func GetSecretName(dev *model.Dev) string {
	return fmt.Sprintf(oktetoSecretTemplate, dev.Name)
}

// List lists secrets for a namespace
func (s *Secrets) List(ctx context.Context, ns, labelSelector string) ([]v1.Secret, error) {
	sList, err := s.k8sClient.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	return sList.Items, nil
}
