package root

import (
	"github.com/odit-services/cnpg-plugin-pgdump/cmd/plugin"
	"github.com/spf13/cobra"
)

// New creates the root CLI and adds the plugin subcommand.
func New(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "cnpg-plugin-pgdump",
		Short: "CNPG-I plugin for logical PostgreSQL backups with pg_dump",
	}
	root.AddCommand(plugin.New(version))
	return root
}
