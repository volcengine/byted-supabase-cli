package solo

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	solomcp "github.com/volcengine/byted-supabase-cli/internal/mcp/solo"
)

func newMCPCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{
		GroupID: "tools",
		Use:     "mcp",
		Short:   "Model Context Protocol server",
	}
	command.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server for the linked branch over stdio",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return solomcp.Serve(cmd.Context(), solomcp.Options{
				Debug:    viper.GetBool("DEBUG"),
				ReadOnly: os.Getenv("READ_ONLY") == "1" || os.Getenv("READ_ONLY") == "true",
			})
		},
	})
	_ = ctx
	return command
}
