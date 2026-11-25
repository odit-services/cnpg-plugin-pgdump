package root

import (
	"github.com/odit-services/cnpg-plugin-pgdump/cmd/instance"
	"github.com/odit-services/cnpg-plugin-pgdump/cmd/plugin"
	"github.com/spf13/cobra"
)

// New creates the root CLI and adds the plugin (operator side) and instance (sidecar) subcommands.
func New(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "cnpg-plugin-pgdump",
		Short: "CNPG-I plugin for logical PostgreSQL backups with pg_dump",
	}
	root.AddCommand(plugin.New(version))
	root.AddCommand(instance.New(version))
	return root
}
