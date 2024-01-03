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

package stack

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/okteto/okteto/pkg/k8s/ingresses"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

func ListEndpoints(ctx context.Context, stack *model.Stack) error {
	c, _, err := okteto.GetK8sClient()
	if err != nil {
		return fmt.Errorf("failed to load your local Kubeconfig: %w", err)
	}
	iClient, err := ingresses.GetClient(c)
	if err != nil {
		return err
	}

	endpointList, err := iClient.GetEndpointsBySelector(ctx, stack.Namespace, stack.GetLabelSelector())
	if err != nil {
		return err
	}
	if len(endpointList) > 0 {
		sort.Slice(endpointList, func(i, j int) bool {
			return len(endpointList[i]) < len(endpointList[j])
		})
		oktetoLog.Information("Endpoints available:\n  - %s\n", strings.Join(endpointList, "\n  - "))
	}
	return nil
}
