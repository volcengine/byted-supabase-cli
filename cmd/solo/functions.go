package solo

import (
	"fmt"

	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	newfunction "github.com/volcengine/byted-supabase-cli/internal/functions/new"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func newFunctionsCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{
		GroupID: "data-plane",
		Use:     "functions",
		Short:   "Manage Supabase Edge Functions",
	}
	command.AddCommand(
		newFunctionsListCommand(ctx),
		newFunctionsDownloadCommand(ctx),
		newFunctionsDeleteCommand(ctx),
		newFunctionsNewCommand(ctx),
		newFunctionsDeployCommand(ctx),
	)
	return command
}

func newFunctionsDeployCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var runtime string
	var noVerifyJWT bool
	command := &cobra.Command{
		Use:   "deploy [Function name]",
		Short: "Deploy a Function to Supabase",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("deploy currently requires one Function name")
			}
			if err := confirmSoloWrite(cmd, "deploy Function "+args[0]); err != nil {
				return err
			}
			client, err := soloconfig.LoadFunctionsClient()
			if err != nil {
				return err
			}
			var verifyOverride *bool
			if cmd.Flags().Changed("no-verify-jwt") {
				verifyOverride = &noVerifyJWT
			}
			if err := client.DeployLocal(cmd.Context(), args[0], runtime, verifyOverride, ctx.FS); err != nil {
				return err
			}
			_, err = fmt.Fprintf(ctx.Output, "Deployed Function %s.\n", args[0])
			return err
		},
	}
	command.Flags().StringVar(&runtime, "runtime", "deno", "Function runtime.")
	command.Flags().BoolVar(&noVerifyJWT, "no-verify-jwt", false, "Disable JWT verification for the Function.")
	return command
}

func newFunctionsNewCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var runtime string
	command := &cobra.Command{
		Use:   "new <Function name>",
		Short: "Create a new Function locally",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return newfunction.RunWithOptions(cmd.Context(), args[0], runtime, ctx.FS, false)
		},
	}
	command.Flags().StringVar(&runtime, "runtime", "deno", "Function runtime: deno, native-node20/v1, python3.9, python3.10, python3.12.")
	return command
}

func newFunctionsListCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all Functions in Supabase",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := soloconfig.LoadFunctionsClient()
			if err != nil {
				return err
			}
			functions, err := client.List(cmd.Context())
			if err != nil {
				return err
			}
			return writeJSON(ctx, functions)
		},
	}
}

func newFunctionsDownloadCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{
		Use:   "download [Function name]",
		Short: "Download a Function from Supabase",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := soloconfig.LoadFunctionsClient()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				if err := utils.ValidateFunctionSlug(args[0]); err != nil {
					return err
				}
				return client.Download(cmd.Context(), args[0], utils.FunctionsDir, ctx.FS)
			}
			functions, err := client.List(cmd.Context())
			if err != nil {
				return err
			}
			for _, function := range functions {
				if err := client.Download(cmd.Context(), function.Slug, utils.FunctionsDir, ctx.FS); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newFunctionsDeleteCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <Function name>",
		Short: "Delete a Function from Supabase",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := utils.ValidateFunctionSlug(args[0]); err != nil {
				return err
			}
			if err := confirmSoloWrite(cmd, "delete Function "+args[0]); err != nil {
				return err
			}
			client, err := soloconfig.LoadFunctionsClient()
			if err != nil {
				return err
			}
			if err := client.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			_, err = fmt.Fprintf(ctx.Output, "Deleted Function %s.\n", args[0])
			return err
		},
	}
}
