// Copyright (c) 2021 Supabase, Inc. and contributors
// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT
//
// This file has been modified by ByteDance Ltd. and/or its affiliates.
//
// Original file was released under MIT License, with the full license text
// available at https://github.com/supabase/cli/blob/main/LICENSE.
//
// This modified file is released under the same license.

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
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/storage"
)

var (
	storageCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "storage",
		Short:   "Manage Supabase Storage objects",
	}

	recursive       bool
	storageBranchID string
	bucketPublic    bool
	bucketPrivate   bool
	bucketFileLimit int64
	bucketMIMETypes []string

	lsCmd = &cobra.Command{
		Use:     "ls [path]",
		Example: "ls ss:///bucket/docs",
		Short:   "List objects by path prefix",
		Args:    cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineStorage(cmd, "Which project do you want to list Storage objects for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			objectPath := client.STORAGE_SCHEME + ":///"
			if len(args) > 0 {
				objectPath = args[0]
			}
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return ls.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, storageBranchID, objectPath, recursive, afero.NewOsFs())
		},
	}

	options storage.FileOptions
	maxJobs uint

	cpCmd = &cobra.Command{
		Use: "cp <src> <dst>",
		Example: `cp readme.md ss:///bucket/readme.md
cp -r docs ss:///bucket/docs
cp -r ss:///bucket/docs .
`,
		Short: "Copy objects from src to dst path",
		Args:  cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineStorage(cmd, "Which project do you want to copy Storage objects for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := func(fo *storage.FileOptions) {
				fo.CacheControl = options.CacheControl
				fo.ContentType = options.ContentType
			}
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return cp.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, storageBranchID, args[0], args[1], recursive, maxJobs, afero.NewOsFs(), opts)
		},
	}

	mvCmd = &cobra.Command{
		Use:     "mv <src> <dst>",
		Short:   "Move objects from src to dst path",
		Example: "mv -r ss:///bucket/docs ss:///bucket/www/docs",
		Args:    cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineStorage(cmd, "Which project do you want to move Storage objects for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return mv.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, storageBranchID, args[0], args[1], recursive, afero.NewOsFs())
		},
	}

	rmCmd = &cobra.Command{
		Use:   "rm <file> ...",
		Short: "Remove objects by file path",
		Example: `rm -r ss:///bucket/docs
rm ss:///bucket/docs/example.md ss:///bucket/readme.md
`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineStorage(cmd, "Which project do you want to remove Storage objects from?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return rm.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, storageBranchID, args, recursive, afero.NewOsFs())
		},
	}

	storageBucketsCmd = &cobra.Command{
		Use:   "buckets",
		Short: "Manage Storage buckets",
	}

	storageBucketsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List Storage buckets",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineStorage(cmd, "Which project do you want to list Storage buckets for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := newVolcengineStorageClient()
			if err != nil {
				return err
			}
			return storagebuckets.List(cmd.Context(), api, flags.ProjectRef, storageBranchID)
		},
	}

	storageBucketsGetCmd = &cobra.Command{
		Use:   "get <bucket-id>",
		Short: "Retrieve Storage bucket details",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineStorage(cmd, "Which project do you want to get a Storage bucket from?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := newVolcengineStorageClient()
			if err != nil {
				return err
			}
			return storagebuckets.Get(cmd.Context(), api, flags.ProjectRef, storageBranchID, args[0])
		},
	}

	storageBucketsCreateCmd = &cobra.Command{
		Use:   "create <bucket-id>",
		Short: "Create a Storage bucket",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineStorage(cmd, "Which project do you want to create a Storage bucket for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := newVolcengineStorageClient()
			if err != nil {
				return err
			}
			return storagebuckets.Create(cmd.Context(), api, flags.ProjectRef, storageBranchID, storageBucketWriteParams(cmd, args[0]))
		},
	}

	storageBucketsUpdateCmd = &cobra.Command{
		Use:   "update <bucket-id>",
		Short: "Update a Storage bucket",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineStorage(cmd, "Which project do you want to update a Storage bucket for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := newVolcengineStorageClient()
			if err != nil {
				return err
			}
			return storagebuckets.Update(cmd.Context(), api, flags.ProjectRef, storageBranchID, storageBucketWriteParams(cmd, args[0]))
		},
	}

	storageBucketsDeleteCmd = &cobra.Command{
		Use:   "delete <bucket-id>",
		Short: "Delete a Storage bucket",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineStorage(cmd, "Which project do you want to delete a Storage bucket from?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := newVolcengineStorageClient()
			if err != nil {
				return err
			}
			return storagebuckets.Delete(cmd.Context(), api, flags.ProjectRef, storageBranchID, args[0])
		},
	}
)

func newVolcengineStorageClient() (*volcengine.Client, error) {
	cfg, err := volcengine.LoadConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return volcengine.NewClient(cfg), nil
}

func storageBucketWriteParams(cmd *cobra.Command, bucketID string) storagebuckets.WriteParams {
	commandFlags := cmd.Flags()
	return storagebuckets.WriteParams{
		BucketID:         bucketID,
		Public:           bucketPublic,
		Private:          bucketPrivate,
		PublicSet:        commandFlags.Changed("public"),
		PrivateSet:       commandFlags.Changed("private"),
		FileSizeLimit:    bucketFileLimit,
		FileSizeLimitSet: commandFlags.Changed("file-size-limit"),
		AllowedMimeTypes: bucketMIMETypes,
		AllowedMimeSet:   commandFlags.Changed("allowed-mime-type"),
	}
}

func preRunVolcengineStorage(cmd *cobra.Command, prompt string) error {
	commandFlags := cmd.Flags()
	linked := commandFlags.Changed("linked")
	workspace := commandFlags.Changed("workspace-id") || commandFlags.Changed("project-ref")
	if linked && workspace {
		return errors.New("--linked cannot be combined with --workspace-id or --project-ref")
	}
	return loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), prompt)
}

func init() {
	storageFlags := storageCmd.PersistentFlags()
	storageFlags.Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	// Volcengine Storage operates on remote branch data-plane endpoints only:
	// storageFlags.Bool("local", false, "Connects to Storage API of the local database.")
	// storageCmd.MarkFlagsMutuallyExclusive("linked", "local")
	storageFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	storageFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	storageFlags.StringVar(&storageBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	lsCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively list a directory.")
	storageCmd.AddCommand(lsCmd)
	cpFlags := cpCmd.Flags()
	cpFlags.BoolVarP(&recursive, "recursive", "r", false, "Recursively copy a directory.")
	cpFlags.StringVar(&options.CacheControl, "cache-control", "max-age=3600", "Custom Cache-Control header for HTTP upload.")
	cpFlags.StringVar(&options.ContentType, "content-type", "", "Custom Content-Type header for HTTP upload.")
	cpFlags.Lookup("content-type").DefValue = "auto-detect"
	cpFlags.UintVarP(&maxJobs, "jobs", "j", 1, "Maximum number of parallel jobs.")
	storageCmd.AddCommand(cpCmd)
	rmCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively remove a directory.")
	storageCmd.AddCommand(rmCmd)
	mvCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively move a directory.")
	storageCmd.AddCommand(mvCmd)
	createBucketFlags := storageBucketsCreateCmd.Flags()
	createBucketFlags.BoolVar(&bucketPublic, "public", false, "Make the bucket publicly readable.")
	createBucketFlags.Int64Var(&bucketFileLimit, "file-size-limit", 0, "Maximum object file size in bytes.")
	createBucketFlags.StringSliceVar(&bucketMIMETypes, "allowed-mime-type", nil, "Allowed MIME type. Can be specified multiple times.")
	updateBucketFlags := storageBucketsUpdateCmd.Flags()
	updateBucketFlags.BoolVar(&bucketPublic, "public", false, "Make the bucket publicly readable.")
	updateBucketFlags.BoolVar(&bucketPrivate, "private", false, "Make the bucket private.")
	updateBucketFlags.Int64Var(&bucketFileLimit, "file-size-limit", 0, "Maximum object file size in bytes.")
	updateBucketFlags.StringSliceVar(&bucketMIMETypes, "allowed-mime-type", nil, "Allowed MIME type. Can be specified multiple times.")
	storageBucketsUpdateCmd.MarkFlagsMutuallyExclusive("public", "private")
	storageBucketsCmd.AddCommand(storageBucketsListCmd)
	storageBucketsCmd.AddCommand(storageBucketsGetCmd)
	storageBucketsCmd.AddCommand(storageBucketsCreateCmd)
	storageBucketsCmd.AddCommand(storageBucketsUpdateCmd)
	storageBucketsCmd.AddCommand(storageBucketsDeleteCmd)
	storageCmd.AddCommand(storageBucketsCmd)
	rootCmd.AddCommand(storageCmd)
}
