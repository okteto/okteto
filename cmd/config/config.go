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

package config

import (
	"context"

	"github.com/okteto/okteto/cmd/config/view"
	"github.com/spf13/cobra"
)

//Config manages okteto configuration values of the authenticated user
func Config(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manages okteto configuration values of the authenticated user",
	}
	cmd.AddCommand(view.View(ctx))
	return cmd
}
