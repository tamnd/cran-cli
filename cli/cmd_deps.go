package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) depsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps <package>",
		Short: "Show direct dependencies of a CRAN package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			a.progressf("fetching deps for %s...", name)
			deps, err := a.client.Deps(cmd.Context(), name)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(deps, len(deps))
		},
	}
	return cmd
}
