package main

import (
	"fmt"
	"os"

	"github.com/okteto/app/cli/pkg/config"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/spf13/cobra"

	"github.com/sirupsen/logrus"

	"github.com/okteto/app/cli/cmd"

	// Load the GCP library for authentication

	"k8s.io/apimachinery/pkg/util/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func init() {
	config.SetConfig(&config.Config{
		FolderName:       ".okteto",
		ManifestFileName: "okteto.yml",
	})

	// override client-go error handlers to downgrade the "logging before flag.Parse" error
	errorHandlers := []func(error){
		func(e error) {
			log.Debugf("unhandled error: %s", e)
		},
	}

	runtime.ErrorHandlers = errorHandlers
}

func main() {
	log.Init(logrus.WarnLevel)
	var logLevel string

	root := &cobra.Command{
		Use:           fmt.Sprintf("%s COMMAND [ARG...]", config.GetBinaryName()),
		Short:         "Manage cloud dev environments",
		SilenceErrors: true,
		PersistentPreRun: func(ccmd *cobra.Command, args []string) {
			log.SetLevel(logLevel)
			ccmd.SilenceUsage = true
		},
	}

	root.PersistentFlags().StringVarP(&logLevel, "loglevel", "l", "warn", "amount of information outputted (debug, info, warn, error)")
	root.AddCommand(cmd.Up())
	root.AddCommand(cmd.Database())
	root.AddCommand(cmd.Run())
	root.AddCommand(cmd.Exec())
	root.AddCommand(cmd.Login())
	root.AddCommand(cmd.KubeConfig())
	root.AddCommand(cmd.Version())

	if err := root.Execute(); err != nil {
		log.Fail(err.Error())
		os.Exit(1)
	}
}
