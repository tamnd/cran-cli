package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) topCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top",
		Short: "List recently updated CRAN packages",
		RunE: func(cmd *cobra.Command, _ []string) error {
			limit := a.effectiveLimit(20)
			a.progressf("fetching top %d packages...", limit)
			entries, err := a.client.Latest(cmd.Context(), limit)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(entries, len(entries))
		},
	}
	return cmd
}
