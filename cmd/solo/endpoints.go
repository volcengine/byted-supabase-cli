package solo

import (
	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
)

func newEndpointsCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{GroupID: "data-plane", Use: "endpoints", Short: "Manage Supabase branch endpoints"}
	command.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List Supabase service endpoints",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				client, err := soloconfig.LoadEndpointsClient()
				if err != nil {
					return err
				}
				return writeJSON(ctx, client.List())
			},
		},
		&cobra.Command{
			Use:   "check",
			Short: "Check Supabase service endpoints",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				client, err := soloconfig.LoadEndpointsClient()
				if err != nil {
					return err
				}
				return writeJSON(ctx, client.Check(cmd.Context()))
			},
		},
	)
	return command
}
