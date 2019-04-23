package cmd

import (
	"fmt"
	"os"

	"github.com/okteto/app/cli/pkg/config"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/model"
	"github.com/okteto/app/cli/pkg/okteto"

	"github.com/spf13/cobra"
)

//Run runs a docker image in a okteto space
func Run() *cobra.Command {
	var devPath string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a docker image in your Okteto Space",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting run command")
			devPath = getFullPath(devPath)

			if _, err := os.Stat(devPath); os.IsNotExist(err) {
				return fmt.Errorf("'%s' does not exist", devPath)
			}

			dev, err := model.Get(devPath)
			if err != nil {
				return err
			}

			if len(args) == 1 {
				dev.Image = args[0]
			}

			return RunRun(dev)
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", config.ManifestFileName(), "path to the manifest file")
	return cmd
}

//RunRun creates a service
func RunRun(dev *model.Dev) error {
	progress := newProgressBar(fmt.Sprintf("Running in '%s' your Okteto Space...", dev.Image))
	progress.start()

	err := okteto.RunImage(dev)
	progress.stop()

	if err != nil {
		return err
	}

	log.Success("Your '%s' instance is ready", dev.Name)
	return nil
}
