package registrytoken

import (
	"context"
	"fmt"

	"github.com/docker/cli/cli/config"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type UninstallOptions struct {
	Overwrite bool
}

func Uninstall(ctx context.Context) *cobra.Command {
	options := &UninstallOptions{}
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Okteto's docker-credential-helper",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.ContextOptions{}); err != nil {
				return err
			}
			confDir := config.Dir()
			conf, err := config.Load(confDir)
			if err != nil {
				return errors.Wrapf(err, "couldn't load docker config file from %q", confDir)
			}

			if conf.CredentialsStore == "" {
				return nil
			}

			if conf.CredentialsStore != "okteto" && !options.Overwrite {
				return errors.New(fmt.Sprintf("credentials store is not 'okteto', currently set to %q, use --force to overwrite", conf.CredentialsStore))
			}

			conf.CredentialsStore = ""

			return conf.Save()
		},
	}

	cmd.Flags().BoolVarP(&options.Overwrite, "force", "", false, "force overwrite existing credential store")
	return cmd
}
