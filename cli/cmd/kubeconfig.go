package cmd

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/okteto/app/cli/pkg/config"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/okteto"

	"github.com/spf13/cobra"
)

//KubeConfig fetch credentials for the cluster
func KubeConfig() *cobra.Command {
	var space string
	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Downloads your k8s cluster credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting kubeconfig command")
			if space != "" {
				var err error
				space, err = okteto.GetSpaceID(space)
				if err != nil {
					return err
				}
			}

			return RunKubeConfig(space)
		},
	}
	cmd.Flags().StringVarP(&space, "space", "s", "", "space where to get the k8s credentials")
	return cmd
}

//RunKubeConfig starts the kubeconfig sequence
func RunKubeConfig(space string) error {
	home := config.GetHome()
	configFile := filepath.Join(home, ".kubeconfig")
	configB64, err := okteto.GetK8sB64Config(space)
	if err != nil {
		return err
	}
	configValue, err := base64.StdEncoding.DecodeString(configB64)
	if err != nil {
		return fmt.Errorf("Error decoding credentials: %s", err)
	}
	if err := ioutil.WriteFile(configFile, []byte(configValue), 0600); err != nil {
		return fmt.Errorf("Error writing credentials: %s", err)
	}
	log.Success("Kubeconfig stored at %s", configFile)
	log.Information("Configure kubectl to work on your Okteto Space by running:")
	fmt.Printf("    export KUBECONFIG=%s\n", configFile)
	return nil
}
