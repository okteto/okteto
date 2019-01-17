package cmd

import (
	"fmt"
	"os"
	"sync"

	"github.com/okteto/cnd/pkg/analytics"
	"github.com/okteto/cnd/pkg/config"
	"github.com/okteto/cnd/pkg/k8/client"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	runtime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	// Load the GCP library for authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

type cliConfig struct {
	logLevel string
	actionID string
}

var (
	c = &cliConfig{
		actionID: analytics.NewActionID(),
	}

	analyticsWG = sync.WaitGroup{}
)

// Execute runs the root command
func Execute() {
	root := &cobra.Command{
		Use:   fmt.Sprintf("%s COMMAND [ARG...]", config.GetBinaryName()),
		Short: "Manage cloud native environments",
		PersistentPreRun: func(ccmd *cobra.Command, args []string) {
			l, err := log.ParseLevel(c.logLevel)
			if err == nil {
				log.SetLevel(l)
			}

			ccmd.SilenceUsage = true
		},
	}

	root.PersistentFlags().StringVarP(&c.logLevel, "loglevel", "l", "warn", "amount of information outputted (debug, info, warn, error)")
	root.AddCommand(
		Up(),
		Exec(),
		Down(),
		Version(),
		List(),
		Run(),
		Create(),
		Analytics(),
	)

	// override client-go error handlers to downgrade the "logging before flag.Parse" error
	errorHandlers := []func(error){
		func(e error) {
			log.Debugf("unhandled error: %s", e)
		},
	}

	runtime.ErrorHandlers = errorHandlers

	exitCode := 0
	if err := root.Execute(); err != nil {
		exitCode = 1
	}

	analytics.Wait()
	os.Exit(exitCode)
}

func getKubernetesClient(namespace string) (string, *kubernetes.Clientset, *rest.Config, error) {
	return client.Get(namespace)
}

func addDevPathFlag(cmd *cobra.Command, devPath *string) {
	cmd.Flags().StringVarP(devPath, "file", "f", config.CNDManifestFileName(), "path to the manifest file")
}
