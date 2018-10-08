package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// BaseCmd provides helpers for generic sub-commands.
type BaseCmd struct {
	Root  string
	Cmd   *cobra.Command
	Viper *viper.Viper
}

// Init sets up the cmd to have flags added.
func (b *BaseCmd) Init(root string, cmd *cobra.Command) {
	b.Root = root
	b.Cmd = cmd
	b.Viper = viper.New()
}

// BindFlag plumbs a flag into viper so that it can be used exclusively.
func (b *BaseCmd) BindFlag(name string) error {
	return BindFlag(b.Viper, b.Cmd.Flags().Lookup(name), b.Root)
}
