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
	"fmt"
	"os"

	"github.com/docker/docker-credential-helpers/credentials"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/pkg/auth/dockercredentials"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// TODO @jpf-okteto the following commit, included in v0.8.0 defines
// consts for each action directly at the dependency.
//
// https://github.com/docker/docker-credential-helpers/commit/129017a3cdb99cd8190c525a75d4da6e1d8a9506
const (
	ActionStore   = "store"
	ActionGet     = "get"
	ActionErase   = "erase"
	ActionList    = "list"
	ActionVersion = "version"
)

type regCreds struct {
	okteto.Config
}

func (r regCreds) GetRegistryCredentials(host string) (string, string, error) {
	return r.GetExternalRegistryCredentials(host)
}

func RegistryToken(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registrytoken",
		Short: "docker credentials helper for private registries registered in okteto",
		Long: `Acts as a docker credentials helper printing out private registry credentials previously configured in okteto:

Usage: echo gcr.io | okteto registrytoken get

Valid arguments are: store, get, erase, list, version.

At this time only "get" is supported

More info about docker credentials helpers here: https://github.com/docker/docker-credential-helpers
  `,
		Hidden:    true,
		ValidArgs: []string{ActionStore, ActionGet, ActionErase, ActionList, ActionVersion},
		Args:      cobra.MatchAll(cobra.ExactArgs(1)),
	}

	cmd.Run = func(_ *cobra.Command, args []string) {
		if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{}); err != nil {
			_, _ = fmt.Fprintln(os.Stdout, err)
			os.Exit(1) // skipcq: RVV-A0003
		}
		conf := okteto.Config{}
		if !conf.IsOktetoCluster() {
			_, _ = fmt.Fprintln(os.Stdout, errors.ErrContextIsNotOktetoCluster)
			os.Exit(1) // skipcq: RVV-A0003
		}
		h := dockercredentials.NewOktetoClusterHelper(regCreds{conf})
		action := args[0]
		if err := credentials.HandleCommand(h, action, os.Stdin, os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stdout, err)
			os.Exit(1) // skipcq: RVV-A0003
		}
	}

	cmd.AddCommand(Install())
	cmd.AddCommand(Uninstall())

	return cmd
}

func IsRegistryCredentialHelperCommand(args []string) bool {
	validArgsLength := 3
	if len(args) != validArgsLength {
		return false
	}

	if args[1] != "registrytoken" {
		return false
	}

	switch args[2] {
	case ActionStore, ActionGet, ActionErase, ActionList, ActionVersion:
		return true
	default:
		return false
	}
}
