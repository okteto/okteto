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

package registrytoken

import (
	"context"
	"os"

	contextCMD "github.com/okteto/okteto/cmd/context"

	"github.com/okteto/okteto/pkg/auth/dockercredentials"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"

	"github.com/docker/docker-credential-helpers/credentials"
)

type regCreds struct {
	okteto.Config
}

func (r regCreds) GetRegistryCredentials(host string) (string, string, error) {
	return r.GetExternalRegistryCredentials(host)
}

func RegistryToken() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registrytoken",
		Short: "docker credentials helper for private registries registered in okteto",
		Long: `Acts as a docker credentials helper printing out private registry credentials previously configured in okteto:

Usage: echo gcr.io | okteto registrytoken get

More info about docker credentials helpers here: https://github.com/docker/docker-credential-helpers
  `,
		Hidden:    true,
		ValidArgs: []string{"get"},
		Args:      cobra.OnlyValidArgs,
	}

	cmd.RunE = func(_ *cobra.Command, args []string) error {
		action := args[0]
		ctx := context.Background()
		if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.ContextOptions{}); err != nil {
			return err
		}
		conf := okteto.Config{}
		if !conf.IsOktetoCluster() {
			return errors.ErrContextIsNotOktetoCluster
		}
		h := dockercredentials.NewOktetoClusterHelper(regCreds{conf})
		return credentials.HandleCommand(h, action, os.Stdin, os.Stdout)
	}

	return cmd
}
