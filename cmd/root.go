package cmd

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	logLevel string
	devPath  string
	root     = &cobra.Command{
		Use:   "cnd [flags] COMMAND [ARG...]",
		Short: "Manage cloud native environments",
		PersistentPreRun: func(ccmd *cobra.Command, args []string) {
			l, err := log.ParseLevel(logLevel)
			if err == nil {
				log.SetLevel(l)
			}

			ccmd.SilenceUsage = true
		},
	}
)

func init() {
	root.PersistentFlags().StringVarP(&logLevel, "loglevel", "l", "warn", "The amount of information outputted (debug, info, warn, error)")
	root.PersistentFlags().StringVarP(&devPath, "file", "f", "cnd.yml", "manifest file")
	root.AddCommand(
		Up(),
		Exec(),
		Down(),
		Rm(),
		Version(),
	)
}

// Execute runs the root command
func Execute() {
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
