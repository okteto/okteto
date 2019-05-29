package cmd

import (
	"github.com/okteto/app/cli/pkg/config"
	"github.com/okteto/app/cli/pkg/k8s/client"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/okteto"

	"github.com/spf13/cobra"
)

//KubeConfig fetch credentials for the cluster
func KubeConfig() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Downloads k8s credentials for a Okteto Space",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting select command")
			space := ""
			if len(args) > 0 {
				var err error
				space, err = okteto.GetSpaceID(args[0])
				if err != nil {
					return err
				}
			}

			return RunKubeConfig(space)
		},
	}
	return cmd
}

//RunKubeConfig starts the kubeconfig sequence
func RunKubeConfig(space string) error {
	kubeConfigFile := config.GetKubeConfigFile()
	if err := client.SetKubeConfig(kubeConfigFile, space); err != nil {
		return err
	}
	log.Success("Updated context '%s' in '%s'", okteto.GetURLWithUnderscore(), kubeConfigFile)
	return nil
}
