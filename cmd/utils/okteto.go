// Copyright 2022 The Okteto Authors
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
	"fmt"
	"os"
	"strconv"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
)

// HasAccessToNamespace checks if the user has access to a namespace/preview
func HasAccessToNamespace(ctx context.Context, namespace string, oktetoClient types.OktetoInterface) (bool, error) {

	nList, err := oktetoClient.Namespaces().List(ctx)
	if err != nil {
		return false, err
	}

	for i := range nList {
		if nList[i].ID == namespace {
			return true, nil
		}
	}

	previewList, err := oktetoClient.Previews().List(ctx)
	if err != nil {
		return false, err
	}

	for i := range previewList {
		if previewList[i].ID == namespace {
			return true, nil
		}
	}

	return false, nil
}

// LoadBoolean loads a boolean environment variable and returns it value
func LoadBoolean(k string) bool {
	v := os.Getenv(k)
	if v == "" {
		v = "false"
	}

	h, err := strconv.ParseBool(v)
	if err != nil {
		oktetoLog.Yellow("'%s' is not a valid value for environment variable %s", v, k)
	}

	return h
}

// ShouldCreateNamespace checks if the namespace exists and if not ask the user if he wants to create it
func ShouldCreateNamespace(ctx context.Context, ns string) (bool, error) {
	c, err := okteto.NewOktetoClient()
	if err != nil {
		return false, err
	}
	hasAccess, err := HasAccessToNamespace(ctx, ns, c)
	if err != nil {
		return false, err
	}
	if !hasAccess {
		if LoadBoolean(model.OktetoWithinDeployCommandContextEnvVar) {
			return false, fmt.Errorf("cannot deploy on a namespace that doesn't exist. Please create %s and try again", ns)
		}
		create, err := AskYesNo(fmt.Sprintf("The namespace %s doesn't exist. Do you want to create it? [y/n] ", ns))
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
