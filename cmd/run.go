package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/okteto/cnd/pkg/config"

	"github.com/okteto/cnd/pkg/analytics"
	"github.com/okteto/cnd/pkg/model"
	"github.com/spf13/cobra"
)

//Run executes a custom command on the CND container
func Run() *cobra.Command {
	var devPath string
	cmd := &cobra.Command{
		Use:   "run SCRIPT ARGS",
		Short: fmt.Sprintf("Run a script defined in your %s file directly in your cloud native environment", config.CNDManifestFileName()),
		RunE: func(cmd *cobra.Command, args []string) error {
			analytics.Send(analytics.EventRun, c.actionID)
			defer analytics.Send(analytics.EventRunEnd, c.actionID)
			return executeRun(devPath, args)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || args[0] == "" {
				return errors.New("run requires the SCRIPT argument")
			}

			return nil
		},
	}

	addDevPathFlag(cmd, &devPath)
	return cmd
}

func executeRun(devPath string, args []string) error {
	dev, err := model.ReadDev(devPath)
	if err != nil {
		return err
	}

	if val, ok := dev.Scripts[args[0]]; ok {
		return executeExec(parseArguments(val, args))
	}

	return fmt.Errorf("%s is not defined in %s. [%s]", args[0], devPath, strings.Join(getScripts(dev), ", "))

}

func parseArguments(scriptArgs string, extraArgs []string) []string {
	mergedArgs := strings.Split(scriptArgs, " ")
	if len(extraArgs) > 1 {
		mergedArgs = append(mergedArgs, extraArgs[1:]...)
	}

	return mergedArgs

}

func getScripts(d *model.Dev) []string {
	keys := make([]string, 0)
	for key := range d.Scripts {
		keys = append(keys, key)
	}

	return keys
}
