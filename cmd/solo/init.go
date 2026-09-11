package solo

import (
	"fmt"

	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	initlocal "github.com/volcengine/byted-supabase-cli/internal/init"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func newInitCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var overwrite bool
	command := &cobra.Command{
		GroupID: "tools",
		Use:     "init",
		Short:   "Initialize a local Supabase project",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := initlocal.Run(cmd.Context(), ctx.FS, false, utils.InitParams{Overwrite: overwrite}); err != nil {
				return err
			}
			_, err := fmt.Fprintln(ctx.Output, "Finished byted-supabase-cli init.")
			return err
		},
	}
	command.Flags().BoolVar(&overwrite, "force", false, "Overwrite an existing config.toml.")
	return command
}
