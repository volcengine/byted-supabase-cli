// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	storagebuckets "github.com/volcengine/byted-supabase-cli/internal/storage/buckets"
	"github.com/volcengine/byted-supabase-cli/internal/storage/client"
	"github.com/volcengine/byted-supabase-cli/internal/storage/cp"
	"github.com/volcengine/byted-supabase-cli/internal/storage/ls"
	"github.com/volcengine/byted-supabase-cli/internal/storage/mv"
	"github.com/volcengine/byted-supabase-cli/internal/storage/rm"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/storage"
)

type storageOptions struct {
	WorkspaceID string
	Linked      bool
	BranchID    string
}

func storageCommandGroup() CommandGroup {
	return CommandGroup{Factories: []CommandFactory{newStorageCommand}}
}

func newStorageCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "storage",
		Short:   "Manage Supabase Storage Objects",
	}
	SetAuthPolicy(cmd, AuthPolicyVolcengine)
	cmd.AddCommand(
		newStorageListCommand(ctx),
		newStorageCopyCommand(ctx),
		newStorageMoveCommand(ctx),
		newStorageRemoveCommand(ctx),
		newStorageBucketsCommand(ctx),
	)
	return cmd
}

func addStorageWorkspaceFlags(cmd *cobra.Command, options *storageOptions) {
	flags := cmd.Flags()
	flags.BoolVar(&options.Linked, "linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	flags.StringVar(&options.WorkspaceID, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	flags.StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	flags.StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	markFlagTelemetrySafe(flags.Lookup("project-ref"))
	markFlagTelemetrySafe(flags.Lookup("workspace-id"))
}

func prepareStorageWorkspace(cmd *cobra.Command, fsys afero.Fs, options *storageOptions, prompt string) error {
	workspace := cmd.Flags().Changed("workspace-id") || cmd.Flags().Changed("project-ref")
	if cmd.Flags().Changed("linked") && options.Linked && workspace {
		return errors.New("--linked cannot be combined with --workspace-id or --project-ref")
	}
	workspaceID, err := (WorkspaceResolver{FS: fsys}).Resolve(cmd.Context(), options.WorkspaceID, options.Linked)
	if err == nil {
		options.WorkspaceID = workspaceID
	}
	return err
}

func newStorageListCommand(ctx RuntimeContext) *cobra.Command {
	parent := &storageOptions{}
	var recursive bool
	cmd := &cobra.Command{
		Use: "ls [path]", Example: "ls ss:///bucket/docs", Short: "List objects by path prefix", Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareStorageWorkspace(cmd, ctx.FS, parent, "Which project do you want to list Storage objects for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			objectPath := client.STORAGE_SCHEME + ":///"
			if len(args) > 0 {
				objectPath = args[0]
			}
			client, err := volcengineClientFromAuth(cmd)
			if err != nil {
				return err
			}
			return ls.RunVolcengine(cmd.Context(), client, parent.WorkspaceID, parent.BranchID, objectPath, recursive, afero.NewOsFs())
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively list a directory.")
	addStorageWorkspaceFlags(cmd, parent)
	_ = ctx
	return cmd
}

func newStorageCopyCommand(ctx RuntimeContext) *cobra.Command {
	parent := &storageOptions{}
	var recursive bool
	var maxJobs uint
	var options storage.FileOptions
	cmd := &cobra.Command{
		Use: "cp <src> <dst>", Example: "cp readme.md ss:///bucket/readme.md\ncp -r docs ss:///bucket/docs\ncp -r ss:///bucket/docs .", Short: "Copy objects from src to dst path", Args: cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareStorageWorkspace(cmd, ctx.FS, parent, "Which project do you want to copy Storage objects for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := volcengineClientFromAuth(cmd)
			if err != nil {
				return err
			}
			opts := func(fileOptions *storage.FileOptions) {
				fileOptions.CacheControl = options.CacheControl
				fileOptions.ContentType = options.ContentType
			}
			return cp.RunVolcengine(cmd.Context(), client, parent.WorkspaceID, parent.BranchID, args[0], args[1], recursive, maxJobs, afero.NewOsFs(), opts)
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively copy a directory.")
	cmd.Flags().StringVar(&options.CacheControl, "cache-control", "max-age=3600", "Custom Cache-Control header for HTTP upload.")
	cmd.Flags().StringVar(&options.ContentType, "content-type", "", "Custom Content-Type header for HTTP upload.")
	cmd.Flags().Lookup("content-type").DefValue = "auto-detect"
	cmd.Flags().UintVarP(&maxJobs, "jobs", "j", 1, "Maximum number of parallel jobs.")
	addStorageWorkspaceFlags(cmd, parent)
	_ = ctx
	return cmd
}

func newStorageMoveCommand(ctx RuntimeContext) *cobra.Command {
	parent := &storageOptions{}
	var recursive bool
	cmd := &cobra.Command{
		Use: "mv <src> <dst>", Short: "Move objects from src to dst path", Example: "mv -r ss:///bucket/docs ss:///bucket/www/docs", Args: cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareStorageWorkspace(cmd, ctx.FS, parent, "Which project do you want to move Storage objects for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := volcengineClientFromAuth(cmd)
			if err != nil {
				return err
			}
			return mv.RunVolcengine(cmd.Context(), client, parent.WorkspaceID, parent.BranchID, args[0], args[1], recursive, afero.NewOsFs())
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively move a directory.")
	addStorageWorkspaceFlags(cmd, parent)
	_ = ctx
	return cmd
}

func newStorageRemoveCommand(ctx RuntimeContext) *cobra.Command {
	parent := &storageOptions{}
	var recursive bool
	cmd := &cobra.Command{
		Use: "rm <file> ...", Short: "Remove objects by file path", Example: "rm -r ss:///bucket/docs\nrm ss:///bucket/docs/example.md ss:///bucket/readme.md",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareStorageWorkspace(cmd, ctx.FS, parent, "Which project do you want to remove Storage objects from?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := volcengineClientFromAuth(cmd)
			if err != nil {
				return err
			}
			return rm.RunVolcengine(cmd.Context(), client, parent.WorkspaceID, parent.BranchID, args, recursive, afero.NewOsFs())
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively remove a directory.")
	addStorageWorkspaceFlags(cmd, parent)
	_ = ctx
	return cmd
}

func newStorageBucketsCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "buckets", Short: "Manage Storage buckets"}
	cmd.AddCommand(
		newStorageBucketsListCommand(ctx),
		newStorageBucketsGetCommand(ctx),
		newStorageBucketsCreateCommand(ctx),
		newStorageBucketsUpdateCommand(ctx),
		newStorageBucketsDeleteCommand(ctx),
	)
	return cmd
}

func newStorageBucketsListCommand(ctx RuntimeContext) *cobra.Command {
	parent := &storageOptions{}
	return newStorageBucketCommand("list", "List Storage buckets", ctx, parent, cobra.NoArgs, func(cmd *cobra.Command, args []string, api *volcengine.Client) error {
		return storagebuckets.List(cmd.Context(), api, parent.WorkspaceID, parent.BranchID)
	})
}

func newStorageBucketsGetCommand(ctx RuntimeContext) *cobra.Command {
	parent := &storageOptions{}
	return newStorageBucketCommand("get <bucket-id>", "Retrieve Storage bucket details", ctx, parent, cobra.ExactArgs(1), func(cmd *cobra.Command, args []string, api *volcengine.Client) error {
		return storagebuckets.Get(cmd.Context(), api, parent.WorkspaceID, parent.BranchID, args[0])
	})
}

func newStorageBucketsCreateCommand(ctx RuntimeContext) *cobra.Command {
	parent := &storageOptions{}
	var options storageBucketOptions
	cmd := newStorageBucketCommand("create <bucket-id>", "Create a Storage bucket", ctx, parent, cobra.ExactArgs(1), func(cmd *cobra.Command, args []string, api *volcengine.Client) error {
		return storagebuckets.Create(cmd.Context(), api, parent.WorkspaceID, parent.BranchID, options.writeParams(cmd, args[0]))
	})
	addStorageBucketWriteFlags(cmd, &options, false)
	return cmd
}

func newStorageBucketsUpdateCommand(ctx RuntimeContext) *cobra.Command {
	parent := &storageOptions{}
	var options storageBucketOptions
	cmd := newStorageBucketCommand("update <bucket-id>", "Update a Storage bucket", ctx, parent, cobra.ExactArgs(1), func(cmd *cobra.Command, args []string, api *volcengine.Client) error {
		return storagebuckets.Update(cmd.Context(), api, parent.WorkspaceID, parent.BranchID, options.writeParams(cmd, args[0]))
	})
	addStorageBucketWriteFlags(cmd, &options, true)
	cmd.MarkFlagsMutuallyExclusive("public", "private")
	return cmd
}

func newStorageBucketsDeleteCommand(ctx RuntimeContext) *cobra.Command {
	parent := &storageOptions{}
	return newStorageBucketCommand("delete <bucket-id>", "Delete a Storage bucket", ctx, parent, cobra.ExactArgs(1), func(cmd *cobra.Command, args []string, api *volcengine.Client) error {
		return storagebuckets.Delete(cmd.Context(), api, parent.WorkspaceID, parent.BranchID, args[0])
	})
}

func newStorageBucketCommand(use, short string, ctx RuntimeContext, parent *storageOptions, args cobra.PositionalArgs, run func(*cobra.Command, []string, *volcengine.Client) error) *cobra.Command {
	cmd := &cobra.Command{
		Use: use, Short: short, Args: args,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareStorageWorkspace(cmd, ctx.FS, parent, "Which project do you want to access Storage buckets for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := newVolcengineStorageClientFactory(cmd)
			if err != nil {
				return err
			}
			return run(cmd, args, api)
		},
	}
	addStorageWorkspaceFlags(cmd, parent)
	_ = ctx
	return cmd
}

type storageBucketOptions struct {
	Public           bool
	Private          bool
	FileSizeLimit    int64
	AllowedMimeTypes []string
}

func addStorageBucketWriteFlags(cmd *cobra.Command, options *storageBucketOptions, includePrivate bool) {
	cmd.Flags().BoolVar(&options.Public, "public", false, "Make the bucket publicly readable.")
	if includePrivate {
		cmd.Flags().BoolVar(&options.Private, "private", false, "Make the bucket private.")
	}
	cmd.Flags().Int64Var(&options.FileSizeLimit, "file-size-limit", 0, "Maximum object file size in bytes.")
	cmd.Flags().StringSliceVar(&options.AllowedMimeTypes, "allowed-mime-type", nil, "Allowed MIME type. Can be specified multiple times.")
}

func (options storageBucketOptions) writeParams(cmd *cobra.Command, bucketID string) storagebuckets.WriteParams {
	return storagebuckets.WriteParams{
		BucketID: bucketID, Public: options.Public, Private: options.Private,
		PublicSet: cmd.Flags().Changed("public"), PrivateSet: cmd.Flags().Changed("private"),
		FileSizeLimit: options.FileSizeLimit, FileSizeLimitSet: cmd.Flags().Changed("file-size-limit"),
		AllowedMimeTypes: options.AllowedMimeTypes, AllowedMimeSet: cmd.Flags().Changed("allowed-mime-type"),
	}
}

func newVolcengineStorageClientFactory(cmd *cobra.Command) (*volcengine.Client, error) {
	return volcengineClientFromAuth(cmd)
}
