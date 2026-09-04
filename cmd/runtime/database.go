package runtime

import (
	"context"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

func ParseDatabase(ctx context.Context, cmd *cobra.Command, fsys afero.Fs, remote bool) error {
	if !remote || cmd.Flags().Changed("db-url") {
		return flags.ParseDatabaseConfig(ctx, cmd.Flags(), fsys)
	}
	return nil
}
