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

package context

import (
	"context"
	"fmt"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Create adds a new cluster to okteto context
func DeleteCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Args:  utils.ExactArgsAccepted(1, "https://okteto.com/docs/reference/cli/#delete"),
		Short: "Delete a context",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			args[0] = okteto.AddSchema(args[0])
			args[0] = strings.TrimSuffix(args[0], "/")
			if err := Delete(ctx, args[0]); err != nil {
				return err
			}
			return nil
		},
	}
	return cmd
}

func Delete(ctx context.Context, okCtx string) error {
	ctxStore := okteto.ContextStore()
	if okCtx == ctxStore.CurrentContext {
		ctxStore.CurrentContext = ""
	}

	if _, ok := ctxStore.Contexts[okCtx]; ok {
		delete(ctxStore.Contexts, okCtx)
		if err := okteto.WriteOktetoContextConfig(); err != nil {
			return err
		}
		log.Success("'%s' deleted successfully", okCtx)
	} else {
		validOptions := make([]string, 0)
		for k, v := range ctxStore.Contexts {
			if v.IsOkteto {
				validOptions = append(validOptions, k)
			}
		}
		return fmt.Errorf("'%s' context doesn't exist. Valid options are: [%s]", okCtx, strings.Join(validOptions, ", "))
	}
	return nil
}
