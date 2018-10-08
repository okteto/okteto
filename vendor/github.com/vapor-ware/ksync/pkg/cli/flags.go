package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// BindFlag moves cobra flags into viper for exclusive use there.
func BindFlag(v *viper.Viper, flag *pflag.Flag, root string) error {
	if err := v.BindPFlag(flag.Name, flag); err != nil {
		return err
	}

	if err := v.BindEnv(
		flag.Name,
		strings.Replace(
			strings.ToUpper(
				fmt.Sprintf("%s_%s", root, flag.Name)), "-", "_", -1)); err != nil {

		return err
	}

	return nil
}

// DefaultFlags configures a standard set of flags for every command and
// sub-command.
func DefaultFlags(cmd *cobra.Command, name string) error {
	flags := cmd.PersistentFlags()

	// TODO: can this be limited to a selection?
	flags.String(
		"log-level",
		"info",
		"log level to use.")
	return BindFlag(viper.GetViper(), flags.Lookup("log-level"), name)
}
