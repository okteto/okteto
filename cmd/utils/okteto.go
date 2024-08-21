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

package utils

import (
	"context"
	"errors"
	"fmt"

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/okteto/okteto/pkg/vars"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// HasAccessToK8sClusterNamespace checks if the user has access to a namespace
func HasAccessToK8sClusterNamespace(ctx context.Context, namespace string, k8sClient kubernetes.Interface) (bool, error) {
	_, err := k8sClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	return true, nil
}

// errFallback returns true when err is not found or namespace not found
// meaning we need to fallback into previews get query
func errFallback(err error) bool {
	return err != nil &&
		(errors.Is(err, oktetoErrors.ErrNamespaceNotFound) ||
			errors.Is(err, oktetoErrors.ErrNotFound))
}

// HasAccessToOktetoClusterNamespace checks if the user has access to a namespace/preview
func HasAccessToOktetoClusterNamespace(ctx context.Context, namespace string, oktetoClient types.OktetoInterface) (bool, error) {
	_, err := oktetoClient.Namespaces().Get(ctx, namespace)
	if errFallback(err) {
		// added possibility to point a context to a preview environment (namespace)
		// https://github.com/okteto/okteto/pull/2018
		_, err := oktetoClient.Previews().Get(ctx, namespace)
		if err != nil && errors.Is(err, oktetoErrors.ErrNamespaceNotFound) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		return true, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// ShouldCreateNamespace checks if the user has access to the namespace.
// If not, ask the user if he wants to create it
func ShouldCreateNamespace(ctx context.Context, ns string, varManager *vars.Manager) (bool, error) {
	c, err := okteto.NewOktetoClient()
	if err != nil {
		return false, err
	}

	return ShouldCreateNamespaceStateless(ctx, ns, c, varManager)
}

// ShouldCreateNamespaceStateless checks if the user has access to the namespace.
// If not, ask the user if he wants to create it
func ShouldCreateNamespaceStateless(ctx context.Context, ns string, c *okteto.Client, varManager *vars.Manager) (bool, error) {
	hasAccess, err := HasAccessToOktetoClusterNamespace(ctx, ns, c)
	if err != nil {
		return false, err
	}
	if !hasAccess {
		if varManager.LoadBoolean(constants.OktetoWithinDeployCommandContextEnvVar) {
			return false, fmt.Errorf("cannot deploy on a namespace that doesn't exist. Please create %s and try again", ns)
		}
		create, err := AskYesNo(fmt.Sprintf("The namespace %s doesn't exist. Do you want to create it?", ns), YesNoDefault_Yes)
		if err != nil {
			return false, err
		}
		if !create {
			return false, fmt.Errorf("cannot deploy on a namespace that doesn't exist. Please create %s and try again", ns)
		}
		return true, nil
	}
	return false, nil
}
