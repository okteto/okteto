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

package utils

import (
	"context"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

//RunWithRetry runs a function and it refresh the kubernetes credentilas if needed
func RunWithRetry(f func() error) error {
	err := f()

	if !errors.IsCredentialError(err) {
		return err
	}

	if okteto.GetClusterContext() != client.GetSessionContext("") {
		return err
	}

	ctx := context.Background()
	namespace := client.GetContextNamespace("")
	if _, _, err := okteto.RefreshOktetoKubeconfig(ctx, namespace); err != nil {
		return err
	}
	log.Information("Kubernetes credentials updated")

	return f()
}
