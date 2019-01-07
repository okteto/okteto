package cmd

import (
	"os"
	"sync"

	"github.com/okteto/cnd/pkg/analytics"
	"github.com/okteto/cnd/pkg/k8/client"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type config struct {
	logLevel string
	actionID string
}

var (
	c = &config{
		actionID: analytics.NewActionID(),
	}

	analyticsWG = sync.WaitGroup{}

	root = &cobra.Command{
		Use:   "cnd COMMAND [ARG...]",
		Short: "Manage cloud native environments",
		PersistentPreRun: func(ccmd *cobra.Command, args []string) {
			l, err := log.ParseLevel(c.logLevel)
			if err == nil {
				log.SetLevel(l)
			}

			ccmd.SilenceUsage = true
		},
	}
)

func init() {
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
}

// Execute runs the root command
func Execute() {
	if err := root.Execute(); err != nil {
		exit()
	}
}

func getKubernetesClient(namespace string) (string, *kubernetes.Clientset, *rest.Config, error) {
	return client.Get(namespace)
}

func addDevPathFlag(cmd *cobra.Command, devPath *string) {
	cmd.Flags().StringVarP(devPath, "file", "f", "cnd.yml", "path to the cnd manifest file")
}

func exit() {
	analytics.Wait()
	os.Exit(1)
}
