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
	"strings"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Create creates a namespace
func Create(ctx context.Context) *cobra.Command {
	var members *[]string

	cmd := &cobra.Command{
		Use:   "namespace <name>",
		Short: "Creates a namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			err := executeCreateNamespace(ctx, args[0], members)
			analytics.TrackCreateNamespace(err == nil)
			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("create namespace requires one argument")
			}
			return nil
		},
	}

	members = cmd.Flags().StringArrayP("members", "m", []string{}, "members of the namespace, it can the username or email")
	return cmd
}

func executeCreateNamespace(ctx context.Context, namespace string, members *[]string) error {
	oktetoNS, err := okteto.CreateNamespace(ctx, namespace)
	if err != nil {
		return err
	}

	log.Success("Namespace '%s' created", oktetoNS)

	if members != nil && len(*members) > 0 {
		if err := okteto.AddNamespaceMembers(ctx, namespace, *members); err != nil {
			return fmt.Errorf("failed to invite %s to the namespace: %s", strings.Join(*members, ", "), err)
		}
	}

	if err := RunNamespace(ctx, oktetoNS); err != nil {
		return fmt.Errorf("failed to activate your new namespace: %s", err)
	}

	return nil
}
