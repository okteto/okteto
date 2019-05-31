package cmd

import (
	"github.com/okteto/app/cli/pkg/config"
	"github.com/okteto/app/cli/pkg/k8s/client"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/okteto"

	"github.com/spf13/cobra"
)

//Namespace fetch credentials for a cluster namespace
func Namespace() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespace",
		Short: "Downloads k8s credentials for a namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting kubeconfig command")
			namespace := ""
			if len(args) > 0 {
				namespace = args[0]
			}

			return RunNamespace(namespace)
		},
	}
	return cmd
}

//RunNamespace starts the kubeconfig sequence
func RunNamespace(namespace string) error {
	kubeConfigFile := config.GetKubeConfigFile()
	if err := client.SetKubeConfig(kubeConfigFile, namespace); err != nil {
		return err
	}
	log.Success("Updated context '%s' in '%s'", okteto.GetURLWithUnderscore(), kubeConfigFile)
	return nil
}
