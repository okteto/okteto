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

//Delete deletes a namespace
func Delete() *cobra.Command {
	return &cobra.Command{
		Use:   "namespace",
		Short: fmt.Sprintf("Deletes a namespace"),
		RunE: func(cmd *cobra.Command, args []string) error {
			analytics.TrackDeleteNamespace(config.VersionString)
			return executeDeleteNamespace(args[0])
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("delete namespace requires one argument")
			}
			return nil
		},
	}
}

func executeDeleteNamespace(namespace string) error {
	if err := okteto.DeleteNamespace(namespace); err != nil {
		return err
	}
	log.Success("Namespace '%s' deleted", namespace)
	return nil
}
