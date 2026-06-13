package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) packageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package <name>",
		Short: "Get info for a CRAN package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			a.progressf("fetching package %s...", name)
			pkg, err := a.client.Package(cmd.Context(), name)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.render(pkg)
		},
	}
	return cmd
}
