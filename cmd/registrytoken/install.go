package registrytoken

import (
	"context"

	"github.com/docker/cli/cli/config"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type InstallOptions struct {
	Overwrite bool
}

func Install(ctx context.Context) *cobra.Command {
	options := &InstallOptions{}
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Okteto's docker-credential-helper",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.ContextOptions{}); err != nil {
				return err
			}
			confDir := config.Dir()
			conf, err := config.Load(confDir)
			if err != nil {
				return errors.Wrapf(err, "couldn't load docker config file from %q", confDir)
			}

			if conf.CredentialsStore == "okteto" {
				return nil
			}

			if conf.CredentialsStore != "" && !options.Overwrite {
				return errors.New("credentials store is currently set to %q, use --force to overwrite")
			}

			conf.CredentialsStore = "okteto"

			return conf.Save()
		},
	}

	cmd.Flags().BoolVarP(&options.Overwrite, "force", "", false, "force overwrite existing credential store")
	return cmd
}
