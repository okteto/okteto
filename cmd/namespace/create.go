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

package namespace

import (
	"context"
	"fmt"
	"strings"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// CreateOptions represents the options that namespace create has
type CreateOptions struct {
	Members      *[]string
	Namespace    string
	Show         bool
	SetCurrentNs bool
}

// Create creates a namespace
func Create(ctx context.Context, ioCtrl *io.Controller) *cobra.Command {
	options := &CreateOptions{
		Show: false,
	}
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an Okteto Namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{}); err != nil {
				return err
			}
			options.Namespace = args[0]

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			nsCmd, err := NewCommand(ioCtrl)
			if err != nil {
				return err
			}
			err = nsCmd.Create(ctx, options)
			analytics.TrackCreateNamespace(err == nil)
			return err
		},
		Args: utils.ExactArgsAccepted(1, ""),
	}

	options.Members = cmd.Flags().StringArrayP("members", "m", []string{}, "members of the Okteto Namespace, it can by username or email")
	cmd.Flags().BoolVarP(&options.SetCurrentNs, "use", "", true, "use the newly created Okteto Namespace as the current namespace")
	return cmd
}

func (nc *Command) Create(ctx context.Context, opts *CreateOptions) error {
	oktetoNS, err := nc.okClient.Namespaces().Create(ctx, opts.Namespace)
	if err != nil {
		return err
	}

	oktetoLog.Success("Namespace '%s' created", oktetoNS)

	if opts.Members != nil && len(*opts.Members) > 0 {
		if err := nc.okClient.Namespaces().AddMembers(ctx, opts.Namespace, *opts.Members); err != nil {
			return fmt.Errorf("failed to invite %s to the namespace: %w", strings.Join(*opts.Members, ", "), err)
		}
	}

	ctxOptions := &contextCMD.Options{
		IsCtxCommand: opts.Show,
		IsOkteto:     true,
		Token:        okteto.GetContext().Token,
		Namespace:    oktetoNS,
		Context:      okteto.GetContext().Name,
	}

	if opts.SetCurrentNs {
		ctxOptions.Save = true
		ctxOptions.Show = true
	} else {
		ctxOptions.Save = false
		ctxOptions.Show = false
	}

	if err := nc.ctxCmd.Run(ctx, ctxOptions); err != nil {
		return fmt.Errorf("failed to activate your new namespace %s: %w", oktetoNS, err)
	}

	return nil
}
