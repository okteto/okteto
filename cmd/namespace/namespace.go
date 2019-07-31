package namespace

import (
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"

	"github.com/spf13/cobra"
)

//Namespace fetch credentials for a cluster namespace
func Namespace() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespace [name]",
		Short: "Downloads k8s credentials for a namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting kubeconfig command")
			namespace := ""
			if len(args) > 0 {
				namespace = args[0]
			}

			err := RunNamespace(namespace)
			analytics.TrackNamespace(config.VersionString, err == nil)
			return err
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
