package cmd

import (
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/cloudnativedevelopment/cnd/pkg/analytics"
	"github.com/cloudnativedevelopment/cnd/pkg/config"
	"github.com/cloudnativedevelopment/cnd/pkg/k8/client"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/spf13/cobra"
	runtime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kExec "k8s.io/client-go/util/exec"

	// Load the GCP library for authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

type cliConfig struct {
	logLevel string
	actionID string
}

type commandFunc func() *cobra.Command

var (
	c = &cliConfig{
		actionID: analytics.NewActionID(),
	}

	analyticsWG = sync.WaitGroup{}
	commandsFN  = []commandFunc{
		Up,
		Exec,
		Down,
		Version,
		List,
		Run,
		Create,
		Analytics,
	}
)

// Execute runs the root command
func Execute() int {
	root := &cobra.Command{
		Use:           fmt.Sprintf("%s COMMAND [ARG...]", config.GetBinaryName()),
		Short:         "Manage cloud native environments",
		SilenceErrors: true,
		PersistentPreRun: func(ccmd *cobra.Command, args []string) {
			log.SetLevel(c.logLevel)
			ccmd.SilenceUsage = true
		},
	}

	root.PersistentFlags().StringVarP(&c.logLevel, "loglevel", "l", "warn", "amount of information outputted (debug, info, warn, error)")

	for _, fn := range commandsFN {
		root.AddCommand(fn())
	}

	// override client-go error handlers to downgrade the "logging before flag.Parse" error
	errorHandlers := []func(error){
		func(e error) {
			log.Debugf("unhandled error: %s", e)
		},
	}

	runtime.ErrorHandlers = errorHandlers

	exitCode := 0
	if err := root.Execute(); err != nil {

		if cerr, ok := err.(kExec.CodeExitError); ok {
			exitCode = cerr.ExitStatus()
			log.Red("Command failed")
		} else {
			log.Red(err.Error())
			exitCode = 1
		}
	}

	analytics.Wait()
	return exitCode
}

// GetKubernetesClient returns the configured kubernetes client for the specified namespace, or the default if empty
func GetKubernetesClient(namespace string) (string, *kubernetes.Clientset, *rest.Config, string, error) {
	kubePath := path.Join(config.GetCNDHome(), "kubeconfig")
	if _, err := os.Stat(kubePath); os.IsNotExist(err) {
		defaultConfigPath := path.Join(os.Getenv("HOME"), ".kube/config")
		return client.Get(namespace, defaultConfigPath)
	}

	return client.Get(namespace, kubePath)
}

func addDevPathFlag(cmd *cobra.Command, devPath *string) {
	cmd.Flags().StringVarP(devPath, "file", "f", config.CNDManifestFileName(), "path to the manifest file")
}

// GetActionID returns the actionID used to correlate different actions in the same command
func GetActionID() string {
	return c.actionID
}

// Register registers a new command with cnd's root command
func Register(fn commandFunc) {
	commandsFN = append(commandsFN, fn)
}
