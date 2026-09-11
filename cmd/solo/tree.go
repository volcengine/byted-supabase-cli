package solo

import (
	"fmt"

	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	"github.com/volcengine/byted-supabase-cli/internal/solo"
)

func NewCommandTree(ctx commandtree.RuntimeContext) *cobra.Command {
	root := &cobra.Command{
		Use:           "byted-supabase-cli",
		Short:         "Byted Supabase CLI",
		SilenceErrors: true,
	}
	root.AddCommand(
		newConfigureCommand(ctx),
		newAuthCommand(ctx),
		newAIGatewayCommand(ctx),
		newEndpointsCommand(ctx),
		newPagesCommand(ctx),
		newFunctionsCommand(ctx),
		newSecretsCommand(ctx),
		newSeedCommand(ctx),
		newStorageCommand(ctx),
		newDBCommand(ctx),
		newGenCommand(ctx),
		newInitCommand(ctx),
		newInspectCommand(ctx),
	)
	for _, command := range root.Commands() {
		if command.Name() == "configure" {
			command.GroupID = "local-dev"
		}
	}
	return root
}

func newConfigureCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	configure := &cobra.Command{
		Use:   "configure",
		Short: "Manage the linked branch configuration",
	}
	configure.AddCommand(
		&cobra.Command{
			Use:   "get",
			Short: "Show the linked branch configuration",
			RunE: func(cmd *cobra.Command, _ []string) error {
				profile, err := solo.LoadCurrentProfile()
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(ctx.Output, "Linked branch %q: %s\n", profile.Name, profile.SupabaseURL)
				return err
			},
		},
	)
	return configure
}

func newPlaceholderCommand(use, short, groupID string) *cobra.Command {
	return &cobra.Command{
		GroupID: groupID,
		Use:     use,
		Short:   short,
		RunE: func(*cobra.Command, []string) error {
			return fmt.Errorf("%s is not implemented for a directly linked branch yet", use)
		},
	}
}
