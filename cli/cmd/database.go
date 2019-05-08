package cmd

import (
	"fmt"

	"github.com/okteto/app/cli/pkg/analytics"
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
	var space string
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
			return RunDatabase(args[0], space)
		},
	}
	cmd.Flags().StringVarP(&space, "space", "s", "", "space where the database command is executed")
	return cmd
}

//RunDatabase creates a database
func RunDatabase(name, space string) error {
	if space != "" {
		var err error
		space, err = okteto.GetSpaceID(space)
		if err != nil {
			return err
		}
	}

	progress := newProgressBar("Creating your cloud database...")
	progress.start()

	db, err := okteto.CreateDatabase(name, space)
	progress.stop()

	if err != nil {
		return err
	}

	printDisplayContext(fmt.Sprintf("Your %s instance is ready", db.Name), db.Name, []string{db.Endpoint})
	analytics.TrackCreateDatabase(name, VersionString)
	return nil
}

func printDBDisplayContext(name, endpoint string) {
	log.Success("Your %s instance is ready", name)
	log.Println(fmt.Sprintf("    %s     %s", log.BlueString("Name:"), name))
	log.Println(fmt.Sprintf("    %s %s", log.BlueString("Endpoint:"), endpoint))
	fmt.Println()
}
