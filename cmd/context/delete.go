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

package context

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// DeleteCMD removes a cluster from okteto context
func DeleteCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Args:  utils.MinimumNArgsAccepted(1, "https://okteto.com/docs/reference/okteto-cli/#delete"),
		Short: "Delete one or more contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			for idx, arg := range args {
				args[idx] = okteto.AddSchema(arg)
				args[idx] = strings.TrimSuffix(arg, "/")
			}
			errs := Delete(args)
			var errLen int
			var totalContextsDeleted int
			if errs != nil {
				if merr, ok := errs.(*multierror.Error); ok {
					errLen = len(merr.Errors)
				}
			}
			success := len(args) == errLen
			analytics.TrackContextDelete(totalContextsDeleted, success)
			return errs
		},
	}
	return cmd
}

func Delete(okCtxs []string) error {
	ctxStore := okteto.GetContextStore()
	var errs error
	validOptions := make([]string, 0)
	for _, okCtx := range okCtxs {
		if okCtx == ctxStore.CurrentContext {
			ctxStore.CurrentContext = ""
		}

		if _, ok := ctxStore.Contexts[okCtx]; ok {
			delete(ctxStore.Contexts, okCtx)
			if err := okteto.NewContextConfigWriter().Write(); err != nil {
				return err
			}
			oktetoLog.Success("'%s' deleted successfully", okCtx)
		} else {
			for k, v := range ctxStore.Contexts {
				if v.IsOkteto {
					validOptions = append(validOptions, k)
				}
			}
			errs = multierror.Append(errs, fmt.Errorf("'%s' context doesn't exist.", okCtx))
		}
	}

	if errs != nil {
		if err, ok := errs.(*multierror.Error); ok {
			err.ErrorFormat = func(es []error) string {
				points := make([]string, len(es))
				for i, err := range es {
					points[i] = fmt.Sprintf("\t- %s", err)
				}

				hint := fmt.Sprintf("Valid options are: [%s]. To delete a Kubernetes context run 'kubectl config delete-context <k8s-context-name>'", strings.Join(validOptions, ", "))
				return fmt.Sprintf("Following contexts couldn't be deleted:\n%s\n%s\n\n", strings.Join(points, "\n"), hint)
			}
		}
	}

	return errs
}
