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
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// VersionString the version of the cli
var VersionString string

func init() {
	config.SetConfig(&config.Config{
		FolderName:       ".stereo",
		ManifestFileName: "stereo.yml",
	})
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
	root.AddCommand(cmd.Exec())
	if err := root.Execute(); err != nil {
		log.Errorf(err.Error())
		os.Exit(1)
	}
}
