package cmd

import (
	"fmt"

	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/okteto"

	"github.com/spf13/cobra"
)

var supportedDatabases = map[string]bool{
	"redis":    true,
	"mongo":    true,
	"postgres": true,
}

//Database starts a cloud dev environment
func Database() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "database",
		Short: "Creates a cloud database in your Okteto Space",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting database command")

			if len(args) != 1 {
				return fmt.Errorf("'database' command expects one argument ['mongo, 'redis' or 'postgres']")
			}
			if _, ok := supportedDatabases[args[0]]; !ok {
				return fmt.Errorf("'database' command expects one argument ['mongo, 'redis' or 'postgres']")
			}
			return RunDatabase(args[0])
		},
	}

	return cmd
}

//RunDatabase creates a database
func RunDatabase(name string) error {
	progress := newProgressBar("Creating your cloud database...")
	progress.start()

	err := okteto.CreateDatabase(name)
	progress.stop()

	if err != nil {
		return err
	}

	log.Success("Your '%s' instance is ready", name)
	return nil
}
