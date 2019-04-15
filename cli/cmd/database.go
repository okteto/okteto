package cmd

import (
	"fmt"
	"time"

	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/okteto"

	"github.com/briandowns/spinner"
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
		Short: "Creates a cloud database",
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
	progress := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	progress.Suffix = " Creating your cloud database..."
	progress.Start()

	err := okteto.CreateDatabase(name)
	progress.Stop()

	if err != nil {
		return err
	}

	log.Success("Your '%s' instance is ready", name)
	return nil
}
