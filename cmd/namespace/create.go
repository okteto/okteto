package namespace

import (
	"context"
	"errors"
	"fmt"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

//Create creates a namespace
func Create(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "namespace <name>",
		Short: fmt.Sprintf("Creates a namespace"),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := executeCreateNamespace(ctx, args[0])
			analytics.TrackCreateNamespace(err == nil)
			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("create namespace requires one argument")
			}
			return nil
		},
	}
}

func executeCreateNamespace(ctx context.Context, namespace string) error {
	oktetoNS, err := okteto.CreateNamespace(ctx, namespace)
	if err != nil {
		return err
	}
	log.Success("Namespace '%s' created", oktetoNS)

	if err := RunNamespace(ctx, namespace); err != nil {
		return err
	}

	return nil
}
