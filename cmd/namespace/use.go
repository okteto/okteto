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

package namespace

import (
	"context"
	"fmt"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

const (
	newNamespaceOption = "Create new namespace"
)

//UseOptions are the options for the use command
type UseOptions struct {
	personal bool
}

// Use sets the namespace of current context
func Use(ctx context.Context) *cobra.Command {
	options := &UseOptions{}
	cmd := &cobra.Command{
		Use:     "use [namespace]",
		Short:   "Configure the current namespace of the okteto context",
		Aliases: []string{"ns"},
		Args:    utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#use-1"),
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace := ""
			if len(args) > 0 {
				namespace = args[0]
			}

			if !okteto.IsOkteto() {
				return errors.ErrContextIsNotOktetoCluster
			}

			if options.personal {
				namespace = okteto.Context().PersonalNamespace
			}

			nsCmd, err := NewCommand()
			if err != nil {
				return err
			}
			err = nsCmd.Use(ctx, namespace)

			analytics.TrackNamespace(err == nil, len(args) > 0)
			return err
		},
	}
	cmd.Flags().BoolVarP(&options.personal, "personal", "", false, "Load personal account")

	return cmd
}

func (nc *NamespaceCommand) Use(ctx context.Context, namespace string) error {
	var err error
	if namespace == "" {
		namespace, err = nc.getNamespaceFromSelector(ctx)
		if err != nil {
			return err
		}
	}

	return nc.ctxCmd.Run(
		ctx,
		&contextCMD.ContextOptions{
			Context:      okteto.Context().Name,
			Namespace:    namespace,
			Save:         true,
			Show:         false,
			IsCtxCommand: true,
		},
	)

}

func (nc *NamespaceCommand) getNamespaceFromSelector(ctx context.Context) (string, error) {
	namespaces, currentIndex, err := getNamespacesSelection(ctx)
	if err != nil {
		return "", err
	}
	selector := oktetoLog.NewOktetoSelector("Select the namespace you want to use:", namespaces, "Namespace")
	selector.SetInitialPosition(currentIndex)

	ns, err := selector.Ask()
	if err != nil {
		return "", err
	}
	if ns == newNamespaceOption {
		ns = askForOktetoNamespace()
		createOptions := &CreateOptions{
			Namespace: ns,
			Show:      false,
		}
		if err := nc.Create(ctx, createOptions); err != nil {
			return "", err
		}
	}
	return ns, nil
}

func getNamespacesSelection(ctx context.Context) ([]oktetoLog.SelectorItem, int, error) {
	currentIndx := -1
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, currentIndx, err
	}
	spaces, err := oktetoClient.Namespaces().List(ctx)
	if err != nil {
		return nil, currentIndx, fmt.Errorf("failed to get namespaces: %s", err)
	}

	namespaces := []oktetoLog.SelectorItem{}
	for _, space := range spaces {
		if space.Status != "Deleting" {
			namespaces = append(namespaces, oktetoLog.SelectorItem{
				Name:   space.ID,
				Label:  space.ID,
				Enable: true,
			})
		}
	}

	namespaces = append(namespaces, []oktetoLog.SelectorItem{
		{
			Label:  "",
			Enable: false,
		},
		{
			Name:   newNamespaceOption,
			Label:  newNamespaceOption,
			Enable: true,
		},
	}...)

	currentContextName := okteto.CurrentStore.CurrentContext
	currentNamespace := okteto.CurrentStore.Contexts[currentContextName].Namespace
	for indx, namespace := range namespaces {
		if namespace.Name == currentNamespace {
			currentIndx = indx
		}
	}
	return namespaces, currentIndx, nil
}

func askForOktetoNamespace() string {
	var namespace string
	oktetoLog.Question("Enter the namespace you want to use: ")
	fmt.Scanln(&namespace)
	return namespace
}
