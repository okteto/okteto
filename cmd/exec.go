package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

//Exec runs a command in an active session of a cloud native environment
func Exec() *cobra.Command {
	var devPath string
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Runs a command in an active session of a cloud native environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeExec(devPath)
		},
	}
	cmd.Flags().StringVarP(&devPath, "file", "f", "dev.yml", "dev yml file")
	return cmd
}

func executeExec(devPath string) error {
	log.Println("Executing exec...")
	log.Println("Done!")
	return nil
}
