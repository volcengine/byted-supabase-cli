package cmd

import (
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/services"
)

var (
	servicesCmd = &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "services",
		Short:   "Show versions of all Supabase services",
		RunE: func(cmd *cobra.Command, args []string) error {
			return services.Run(cmd.Context(), afero.NewOsFs())
		},
	}
)

func init() {
	// Volcengine CLI does not support Supabase local stack service version sync in the current phase.
	// Keep the original command registered code here for future local development support.
	// rootCmd.AddCommand(servicesCmd)
}
