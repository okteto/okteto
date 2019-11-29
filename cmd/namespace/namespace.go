package namespace

import (
	"context"
	"net/url"
	"strings"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"

	"github.com/spf13/cobra"
)

//Namespace fetch credentials for a cluster namespace
func Namespace(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespace [name]",
		Short: "Downloads k8s credentials for a namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting kubeconfig command")
			namespace := ""
			if len(args) > 0 {
				namespace = args[0]
			}

			err := RunNamespace(ctx, namespace)
			analytics.TrackNamespace(err == nil)
			return err
		},
	}
	return cmd
}

//RunNamespace starts the kubeconfig sequence
func RunNamespace(ctx context.Context, namespace string) error {
	cred, err := okteto.GetCredentials(ctx, namespace)
	if err != nil {
		return err
	}

	kubeConfigFile := config.GetKubeConfigFile()

	u, _ := url.Parse(okteto.GetURL())
	parsedHost := strings.ReplaceAll(u.Host, ".", "_")

	if err := okteto.SetKubeConfig(cred, kubeConfigFile, namespace, okteto.GetUserID(), parsedHost); err != nil {
		return err
	}

	log.Success("Updated context '%s' in '%s'", parsedHost, kubeConfigFile)
	return nil
}
