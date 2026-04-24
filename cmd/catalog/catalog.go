// Copyright 2025 The Okteto Authors
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

package catalog

import (
	"context"

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

// Command groups the shared state for catalog subcommands.
type Command struct {
	okClient types.OktetoInterface
}

// NewCommand builds a Command wired to the real Okteto client.
func NewCommand() (*Command, error) {
	c, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, err
	}
	return &Command{okClient: c}, nil
}

// Catalog returns the parent `okteto catalog` cobra command.
func Catalog(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Browse and deploy Okteto Catalog items",
		Long: `Browse and deploy pre-configured Development Environments from the Okteto Catalog.

Catalog items are templates defined by your administrator. Each item points at a
git repository, a branch, an Okteto manifest, and optional default variables.
Deploying a catalog item creates a Development Environment in your current Okteto
Namespace.`,
	}

	cmd.AddCommand(List(ctx))
	cmd.AddCommand(Deploy(ctx))
	cmd.AddCommand(Add(ctx))
	return cmd
}
