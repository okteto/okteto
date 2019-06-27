package namespace

import (
	"errors"
	"fmt"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

//Create creates a namespace
func Create() *cobra.Command {
	return &cobra.Command{
		Use:   "namespace",
		Short: fmt.Sprintf("Creates a namespace"),
		RunE: func(cmd *cobra.Command, args []string) error {
			analytics.TrackCreateNamespace(config.VersionString)
			return executeCreateNamespace(args[0])
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("create namespace requires one argument")
			}
			return nil
		},
	}
}

func executeCreateNamespace(namespace string) error {
	oktetoNS, err := okteto.CreateNamespace(namespace)
	if err != nil {
		return err
	}
	log.Success("Namespace '%s' created", oktetoNS)

	if err := RunNamespace(namespace); err != nil {
		return err
	}

	return nil
}
