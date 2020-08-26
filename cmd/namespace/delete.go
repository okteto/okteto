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

package namespace

import (
	"context"
	"errors"
	"fmt"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

//Delete deletes a namespace
func Delete(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "namespace <name>",
		Short: fmt.Sprintf("Deletes a namespace"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			err := executeDeleteNamespace(ctx, args[0])
			analytics.TrackDeleteNamespace(err == nil)
			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("delete namespace requires one argument")
			}
			return nil
		},
	}
}

func executeDeleteNamespace(ctx context.Context, namespace string) error {
	if err := okteto.DeleteNamespace(ctx, namespace); err != nil {
		return fmt.Errorf("failed to delete namespace: %s", err)
	}

	if err := CleanNamespace(ctx, namespace); err != nil {
		return fmt.Errorf("failed to clean up namespace: %s", err)
	}

	log.Success("Namespace '%s' deleted", namespace)
	return nil
}
