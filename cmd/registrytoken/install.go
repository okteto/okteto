package registrytoken

import (
	"fmt"

	"github.com/docker/cli/cli/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type InstallOptions struct {
	Overwrite bool
}

func Install() *cobra.Command {
	options := &InstallOptions{}
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Okteto's docker-credential-helper",
		RunE: func(cmd *cobra.Command, args []string) error {
			confDir := config.Dir()
			conf, err := config.Load(confDir)
			if err != nil {
				return errors.Wrapf(err, "couldn't load docker config file from %q", confDir)
			}

			if conf.CredentialsStore == "okteto" {
				oktetoLog.Warning("Okteto's registry credential helper is already installed, skipping ...")
				return nil
			}

			if conf.CredentialsStore != "" && !options.Overwrite {
				return errors.New(fmt.Sprintf("credentials store is currently set to %q, use --force to overwrite", conf.CredentialsStore))
			}

			conf.CredentialsStore = "okteto"

			if err := conf.Save(); err != nil {
				return errors.Wrapf(err, "couldn't save docker config file at %q", confDir)
			}

			oktetoLog.Success("Okteto's registry credential helper successfully installed at %q", confDir)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&options.Overwrite, "force", "", false, "force overwrite existing credential store")
	return cmd
}
