package kubetoken

import (
	"fmt"

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

func KubeToken() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubetoken",
		Short: "Gets a token to access the Kubernetes API with client authentication",
	}

	cmd.RunE = func(*cobra.Command, []string) error {
		c, err := okteto.NewKubeTokenClient()
		if err != nil {
			return fmt.Errorf("failed to initialize the kubetoken client: %w", err)
		}

		out, err := c.GetKubeToken()
		if err != nil {
			return fmt.Errorf("failed to get the kubetoken: %w", err)
		}

		cmd.Print(out)
		return nil
	}

	return cmd
}
