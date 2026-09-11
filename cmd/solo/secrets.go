package solo

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
)

func newSecretsCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{
		GroupID: "data-plane",
		Use:     "secrets",
		Short:   "Manage Supabase secrets",
	}
	command.AddCommand(
		newSecretsListCommand(ctx),
		newSecretsSetCommand(ctx),
		newSecretsUnsetCommand(ctx),
	)
	return command
}

func newSecretsListCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all secrets on Supabase",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := soloconfig.LoadSecretsClient()
			if err != nil {
				return err
			}
			secrets, err := client.List(cmd.Context())
			if err != nil {
				return err
			}
			return writeJSON(ctx, secrets)
		},
	}
}

func newSecretsSetCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var envFile string
	command := &cobra.Command{
		Use:   "set <NAME=VALUE> ...",
		Short: "Set one or more secrets on Supabase",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			secrets, err := soloconfig.ParseSecrets(envFile, ctx.FS, args...)
			if err != nil {
				return err
			}
			if len(secrets) == 0 {
				return fmt.Errorf("No arguments found. Use --env-file to read from a .env file.")
			}
			if err := confirmSoloWrite(cmd, fmt.Sprintf("set %d Supabase secrets", len(secrets))); err != nil {
				return err
			}
			client, err := soloconfig.LoadSecretsClient()
			if err != nil {
				return err
			}
			if err := client.Set(cmd.Context(), secrets); err != nil {
				return err
			}
			_, err = fmt.Fprintln(ctx.Output, "Finished secrets set.")
			return err
		},
	}
	command.Flags().StringVar(&envFile, "env-file", "", "Read secrets from a .env file.")
	return command
}

func newSecretsUnsetCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{
		Use:   "unset [NAME] ...",
		Short: "Unset one or more secrets on Supabase",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := soloconfig.LoadSecretsClient()
			if err != nil {
				return err
			}
			names := append([]string(nil), args...)
			if len(names) == 0 {
				secrets, listErr := client.List(cmd.Context())
				if listErr != nil {
					return listErr
				}
				for _, secret := range secrets {
					if !strings.HasPrefix(secret.Name, "SUPABASE_") {
						names = append(names, secret.Name)
					}
				}
			}
			if len(names) == 0 {
				_, err := fmt.Fprintln(ctx.Output, "No user-managed secrets found.")
				return err
			}
			if err := confirmSoloWrite(cmd, fmt.Sprintf("unset Supabase secrets %v", names)); err != nil {
				return err
			}
			if err := client.Unset(cmd.Context(), names); err != nil {
				return err
			}
			_, err = fmt.Fprintln(ctx.Output, "Finished secrets unset.")
			return err
		},
	}
}
