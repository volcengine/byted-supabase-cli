// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/go-errors/errors"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/errdefs"
	"github.com/volcengine/byted-supabase-cli/internal/gen/types"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

type genTypesOptions struct {
	WorkspaceID       string
	BranchID          string
	Language          utils.EnumFlag
	Schemas           []string
	SwiftAccess       utils.EnumFlag
	PostgrestV9Compat bool
}

func newGenCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "gen",
		Short:   "Run Code Generation Tools",
	}
	SetAuthPolicy(cmd, AuthPolicyVolcengine)
	cmd.AddCommand(newGenTypesCommand(ctx))
	return cmd
}

func newGenTypesCommand(ctx RuntimeContext) *cobra.Command {
	options := genTypesOptions{
		Language: utils.EnumFlag{
			Allowed: []string{types.LangTypescript, types.LangGo, types.LangSwift},
			Value:   types.LangTypescript,
		},
		SwiftAccess: utils.EnumFlag{
			Allowed: []string{types.SwiftInternalAccessControl, types.SwiftPublicAccessControl},
			Value:   types.SwiftInternalAccessControl,
		},
	}
	cmd := &cobra.Command{
		Use:   "types",
		Short: "Generate types from Postgres schema",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && args[0] != types.LangTypescript && !cmd.Flags().Changed("lang") {
				return errors.New("use --lang flag to specify the typegen language")
			}
			if options.PostgrestV9Compat && options.Language.Value != types.LangTypescript {
				return errors.New("--postgrest-v9-compat is supported only with --lang typescript")
			}
			if cmd.Flags().Changed("swift-access-control") && options.Language.Value != types.LangSwift {
				return errors.New("--swift-access-control is supported only with --lang swift")
			}
			workspaceID, err := (WorkspaceResolver{FS: ctx.FS}).Resolve(cmd.Context(), options.WorkspaceID, true)
			if err != nil {
				return err
			}
			options.WorkspaceID = workspaceID
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.WorkspaceID == "" {
				return errdefs.ErrMissingWorkspaceID
			}
			if err := flags.LoadConfig(ctx.FS); err != nil {
				return err
			}
			client, err := volcengineClientFromAuth(cmd)
			if err != nil {
				return err
			}
			return types.RunVolcengine(cmd.Context(), client, types.VolcengineParams{
				WorkspaceID:        options.WorkspaceID,
				BranchID:           options.BranchID,
				Lang:               options.Language.Value,
				Schemas:            options.Schemas,
				PostgrestV9Compat:  options.PostgrestV9Compat,
				SwiftAccessControl: options.SwiftAccess.Value,
			}, ctx.Output)
		},
		Example: `  byted-supabase-cli gen types --lang typescript
  byted-supabase-cli gen types --workspace-id workspace-id --lang go --schema public
  byted-supabase-cli gen types --workspace-id workspace-id --lang swift --swift-access-control public`,
	}
	cmd.Flags().StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	cmd.Flags().StringVar(&options.WorkspaceID, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	cmd.Flags().StringVar(&options.WorkspaceID, "project-id", "", "Project ID of the Supabase project. Alias of --workspace-id for Volcengine.")
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the default branch.")
	cmd.Flags().Var(&options.Language, "lang", "Output language of the generated types.")
	cmd.Flags().StringSliceVarP(&options.Schemas, "schema", "s", nil, "Comma separated list of schema to include.")
	cmd.Flags().Var(&options.SwiftAccess, "swift-access-control", "Access control for Swift generated types.")
	cmd.Flags().BoolVar(&options.PostgrestV9Compat, "postgrest-v9-compat", false, "Generate types compatible with PostgREST v9 and below.")
	return cmd
}
