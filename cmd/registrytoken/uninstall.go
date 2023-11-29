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
	"fmt"

	"github.com/docker/cli/cli/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type UninstallOptions struct {
	Overwrite bool
}

func Uninstall() *cobra.Command {
	options := &UninstallOptions{}
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Okteto's docker-credential-helper",
		RunE: func(cmd *cobra.Command, args []string) error {
			confDir := config.Dir()
			conf, err := config.Load(confDir)
			if err != nil {
				return errors.Wrapf(err, "couldn't load docker config file from %q", confDir)
			}

			if conf.CredentialsStore == "" {
				oktetoLog.Warning("Okteto's registry credential helper is already uninstalled, skipping ...")
				return nil
			}

			if conf.CredentialsStore != "okteto" && !options.Overwrite {
				return errors.New(fmt.Sprintf("credentials store is not 'okteto', currently set to %q, use --force to overwrite", conf.CredentialsStore))
			}

			conf.CredentialsStore = ""

			if err := conf.Save(); err != nil {
				return errors.Wrapf(err, "couldn't save docker config file at %q", confDir)
			}

			oktetoLog.Success("Okteto's registry credential helper successfully uninstalled from %q", confDir)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&options.Overwrite, "force", "", false, "force overwrite existing credential store")
	return cmd
}
