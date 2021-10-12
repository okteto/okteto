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

package utils

import (
	"context"

	"github.com/okteto/okteto/pkg/okteto"
)

func HasAccessToNamespace(ctx context.Context, namespace string) (bool, error) {
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return false, nil
	}
	nList, err := oktetoClient.ListNamespaces(ctx)
	if err != nil {
		return false, err
	}

	for i := range nList {
		if nList[i].ID == namespace {
			return true, nil
		}
	}
	return false, nil
}
