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

package stack

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

func ListEndpoints(ctx context.Context, stack *model.Stack, output string) error {
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}
	endpointList, err := oktetoClient.ListStackEndpoints(ctx, stack)
	if err != nil {
		return fmt.Errorf("failed to get preview environments: %s", err)
	}

	switch output {
	case "json":
		bytes, err := json.MarshalIndent(endpointList, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(bytes))
	default:
		if len(endpointList) == 0 {
			fmt.Printf("There are no available endpoints for stack '%s'\n", stack.Name)
		} else {
			endpoints := make([]string, 0)
			for _, endpoint := range endpointList {
				endpoints = append(endpoints, endpoint.URL)
			}
			sort.Slice(endpoints, func(i, j int) bool {
				return len(endpoints[i]) < len(endpoints[j])
			})
			fmt.Printf("Available endpoints for stack '%s'\n  - %s\n", stack.Name, strings.Join(endpoints, "\n  - "))
		}
	}
	return nil
}
