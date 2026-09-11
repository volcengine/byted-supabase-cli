package solo

import (
	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	gentypes "github.com/volcengine/byted-supabase-cli/internal/gen/types"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func newGenCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{GroupID: "tools", Use: "gen", Short: "Run code generation tools"}
	command.AddCommand(newGenTypesCommand(ctx))
	return command
}

func newGenTypesCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var language = utils.EnumFlag{
		Allowed: []string{gentypes.LangTypescript, gentypes.LangGo, gentypes.LangSwift},
		Value:   gentypes.LangTypescript,
	}
	var swiftAccess = utils.EnumFlag{
		Allowed: []string{gentypes.SwiftInternalAccessControl, gentypes.SwiftPublicAccessControl},
		Value:   gentypes.SwiftInternalAccessControl,
	}
	var schemas []string
	var postgrestV9Compat bool
	command := &cobra.Command{
		Use:   "types",
		Short: "Generate types from the configured Postgres database",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			profile, err := soloconfig.LoadCurrentProfile()
			if err != nil {
				return err
			}
			return gentypes.RunSupabase(cmd.Context(), gentypes.SupabaseParams{
				SupabaseURL: profile.SupabaseURL, ServiceRoleKey: profile.ServiceRoleKey,
				Lang: language.Value, Schemas: schemas,
				PostgrestV9Compat: postgrestV9Compat, SwiftAccessControl: swiftAccess.Value,
			}, ctx.Output)
		},
	}
	command.Flags().Var(&language, "lang", "Output language of the generated types.")
	command.Flags().StringSliceVarP(&schemas, "schema", "s", nil, "Comma separated list of schemas to include.")
	command.Flags().Var(&swiftAccess, "swift-access-control", "Access control for Swift generated types.")
	command.Flags().BoolVar(&postgrestV9Compat, "postgrest-v9-compat", false, "Generate types compatible with PostgREST v9 and below.")
	return command
}
