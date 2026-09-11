package solo

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	"github.com/volcengine/byted-supabase-cli/internal/seed/buckets"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
	storageclient "github.com/volcengine/byted-supabase-cli/internal/storage/client"
	"github.com/volcengine/byted-supabase-cli/internal/storage/cp"
	"github.com/volcengine/byted-supabase-cli/internal/storage/mv"
	"github.com/volcengine/byted-supabase-cli/internal/storage/rm"
	"github.com/volcengine/byted-supabase-cli/pkg/storage"
)

func newStorageCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	storageCommand := &cobra.Command{
		GroupID: "data-plane",
		Use:     "storage",
		Short:   "Manage Supabase Storage objects",
	}
	storageCommand.AddCommand(
		newStorageListCommand(ctx),
		newStorageCopyCommand(ctx),
		newStorageMoveCommand(ctx),
		newStorageRemoveCommand(ctx),
		newStorageBucketsCommand(ctx),
	)
	return storageCommand
}

func newStorageListCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var recursive bool
	command := &cobra.Command{
		Use:   "ls [path]",
		Short: "List Storage objects",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			objectPath := "ss:///"
			if len(args) == 1 {
				objectPath = args[0]
			}
			remotePath, err := storageclient.ParseStorageURL(objectPath)
			if err != nil {
				return err
			}
			api, err := soloconfig.LoadStorageAPI()
			if err != nil {
				return err
			}
			return listStoragePaths(cmd.Context(), api, remotePath, recursive, ctx.Output)
		},
	}
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively list a directory.")
	return command
}

func newStorageCopyCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var recursive bool
	var cacheControl string
	var contentType string
	command := &cobra.Command{
		Use:   "cp <src> <dst>",
		Short: "Copy objects between local files and Storage",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := confirmSoloWrite(cmd, fmt.Sprintf("copy Storage object from %s to %s", args[0], args[1])); err != nil {
				return err
			}
			api, err := soloconfig.LoadStorageAPI()
			if err != nil {
				return err
			}
			options := func(fileOptions *storage.FileOptions) {
				fileOptions.CacheControl = cacheControl
				fileOptions.ContentType = contentType
			}
			return cp.RunWithAPI(cmd.Context(), api, args[0], args[1], recursive, 1, ctx.FS, options)
		},
	}
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively copy a directory.")
	command.Flags().StringVar(&cacheControl, "cache-control", "max-age=3600", "Custom Cache-Control header for upload.")
	command.Flags().StringVar(&contentType, "content-type", "", "Content-Type for upload.")
	return command
}

func newStorageMoveCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var recursive bool
	command := &cobra.Command{
		Use:   "mv <src> <dst>",
		Short: "Move a Storage object",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := confirmSoloWrite(cmd, fmt.Sprintf("move Storage object from %s to %s", args[0], args[1])); err != nil {
				return err
			}
			api, err := soloconfig.LoadStorageAPI()
			if err != nil {
				return err
			}
			return mv.RunWithAPI(cmd.Context(), api, args[0], args[1], recursive, ctx.FS)
		},
	}
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively move a directory.")
	return command
}

func newStorageRemoveCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var recursive bool
	command := &cobra.Command{
		Use:   "rm <file> ...",
		Short: "Remove Storage objects",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := confirmSoloWrite(cmd, fmt.Sprintf("remove Storage objects %v", args)); err != nil {
				return err
			}
			api, err := soloconfig.LoadStorageAPI()
			if err != nil {
				return err
			}
			skipConfirmation, err := cmd.Flags().GetBool("yes")
			if err != nil {
				return err
			}
			return rm.RunWithAPIConfirm(cmd.Context(), api, args, recursive, ctx.FS, skipConfirmation)
		},
	}
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively remove a directory.")
	return command
}

func newSeedCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{
		GroupID: "data-plane",
		Use:     "seed",
		Short:   "Manage Supabase seed data",
	}
	command.AddCommand(&cobra.Command{
		Use:   "buckets",
		Short: "Seed buckets declared in the local Supabase config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := confirmSoloWrite(cmd, "seed Storage buckets"); err != nil {
				return err
			}
			api, err := soloconfig.LoadStorageAPI()
			if err != nil {
				return err
			}
			return buckets.RunWithAPI(cmd.Context(), api, true, ctx.FS)
		},
	})
	return command
}

func newStorageBucketsCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{Use: "buckets", Short: "Manage Storage buckets"}
	command.AddCommand(
		newStorageBucketsListCommand(ctx),
		newStorageBucketsGetCommand(ctx),
		newStorageBucketsCreateCommand(ctx),
		newStorageBucketsUpdateCommand(ctx),
		newStorageBucketsDeleteCommand(ctx),
	)
	return command
}

func newStorageBucketsListCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List Storage buckets", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			api, err := soloconfig.LoadStorageAPI()
			if err != nil {
				return err
			}
			buckets, err := api.ListBuckets(cmd.Context())
			if err != nil {
				return err
			}
			return writeJSON(ctx, buckets)
		},
	}
}

func newStorageBucketsGetCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	return storageBucketCommand("get <bucket-id>", "Retrieve Storage bucket details", cobra.ExactArgs(1), ctx,
		func(cmd *cobra.Command, api storage.StorageAPI, args []string) (interface{}, error) {
			return api.GetBucket(cmd.Context(), args[0])
		})
}

func newStorageBucketsCreateCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var public bool
	var fileSizeLimit int64
	var mimeTypes []string
	command := storageBucketCommand("create <bucket-id>", "Create a Storage bucket", cobra.ExactArgs(1), ctx,
		func(cmd *cobra.Command, api storage.StorageAPI, args []string) (interface{}, error) {
			var publicValue *bool
			if cmd.Flags().Changed("public") {
				publicValue = &public
			}
			return api.CreateBucket(cmd.Context(), storage.CreateBucketRequest{
				Name: args[0], Public: publicValue, FileSizeLimit: fileSizeLimit, AllowedMimeTypes: mimeTypes,
			})
		})
	command.PreRunE = func(cmd *cobra.Command, args []string) error {
		return confirmSoloWrite(cmd, "create Storage bucket "+args[0])
	}
	command.Flags().BoolVar(&public, "public", false, "Make the bucket publicly readable.")
	command.Flags().Int64Var(&fileSizeLimit, "file-size-limit", 0, "Maximum object file size in bytes.")
	command.Flags().StringSliceVar(&mimeTypes, "allowed-mime-type", nil, "Allowed MIME type.")
	return command
}

func newStorageBucketsUpdateCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var public bool
	var private bool
	var fileSizeLimit int64
	var mimeTypes []string
	command := storageBucketCommand("update <bucket-id>", "Update a Storage bucket", cobra.ExactArgs(1), ctx,
		func(cmd *cobra.Command, api storage.StorageAPI, args []string) (interface{}, error) {
			var publicValue *bool
			switch {
			case cmd.Flags().Changed("public"):
				publicValue = &public
			case cmd.Flags().Changed("private"):
				value := !private
				publicValue = &value
			}
			return api.UpdateBucket(cmd.Context(), storage.UpdateBucketRequest{
				Id: args[0], Public: publicValue, FileSizeLimit: fileSizeLimit, AllowedMimeTypes: mimeTypes,
			})
		})
	command.PreRunE = func(cmd *cobra.Command, args []string) error {
		return confirmSoloWrite(cmd, "update Storage bucket "+args[0])
	}
	command.Flags().BoolVar(&public, "public", false, "Make the bucket publicly readable.")
	command.Flags().BoolVar(&private, "private", false, "Make the bucket private.")
	command.Flags().Int64Var(&fileSizeLimit, "file-size-limit", 0, "Maximum object file size in bytes.")
	command.Flags().StringSliceVar(&mimeTypes, "allowed-mime-type", nil, "Allowed MIME type.")
	command.MarkFlagsMutuallyExclusive("public", "private")
	return command
}

func newStorageBucketsDeleteCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := storageBucketCommand("delete <bucket-id>", "Delete a Storage bucket", cobra.ExactArgs(1), ctx,
		func(cmd *cobra.Command, api storage.StorageAPI, args []string) (interface{}, error) {
			return api.DeleteBucket(cmd.Context(), args[0])
		})
	command.PreRunE = func(cmd *cobra.Command, args []string) error {
		return confirmSoloWrite(cmd, "delete Storage bucket "+args[0])
	}
	return command
}

func storageBucketCommand(use, short string, args cobra.PositionalArgs, ctx commandtree.RuntimeContext,
	run func(*cobra.Command, storage.StorageAPI, []string) (interface{}, error)) *cobra.Command {
	return &cobra.Command{
		Use: use, Short: short, Args: args,
		RunE: func(cmd *cobra.Command, values []string) error {
			api, err := soloconfig.LoadStorageAPI()
			if err != nil {
				return err
			}
			result, err := run(cmd, api, values)
			if err != nil {
				return err
			}
			return writeJSON(ctx, result)
		},
	}
}

func listStoragePaths(ctx context.Context, api storage.StorageAPI, remotePath string, recursive bool, output io.Writer) error {
	bucket, prefix := storageclient.SplitBucketPrefix(remotePath)
	if bucket == "" || prefix == "" && !strings.HasSuffix(remotePath, "/") {
		buckets, err := api.ListBuckets(ctx)
		if err != nil {
			return err
		}
		for _, item := range buckets {
			if strings.HasPrefix(item.Name, bucket) {
				fmt.Fprintln(output, item.Name+"/")
			}
		}
		return nil
	}
	objects, err := api.ListObjects(ctx, bucket, prefix, 0)
	if err != nil {
		return err
	}
	for _, object := range objects {
		name := object.Name
		if object.Id == nil {
			name += "/"
		}
		fmt.Fprintln(output, name)
	}
	return nil
}

func copyStorageObject(cmd *cobra.Command, api storage.StorageAPI, src, dst, cacheControl, contentType string, fsys afero.Fs) error {
	srcURL, dstURL := strings.HasPrefix(src, "ss://"), strings.HasPrefix(dst, "ss://")
	if srcURL == dstURL {
		return fmt.Errorf("copying requires one local path and one ss:/// Storage path")
	}
	if dstURL {
		remote, err := storageclient.ParseStorageURL(dst)
		if err != nil {
			return err
		}
		local := src
		file, err := fsys.Open(local)
		if err != nil {
			return err
		}
		defer file.Close()
		return api.UploadObjectStream(cmd.Context(), remote, file, storage.FileOptions{CacheControl: cacheControl, ContentType: contentType})
	}
	remote, err := storageclient.ParseStorageURL(src)
	if err != nil {
		return err
	}
	file, err := fsys.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	return api.DownloadObjectStream(cmd.Context(), remote, file)
}

func moveStorageObject(cmd *cobra.Command, api storage.StorageAPI, src, dst string) error {
	srcPath, err := storageclient.ParseStorageURL(src)
	if err != nil {
		return err
	}
	dstPath, err := storageclient.ParseStorageURL(dst)
	if err != nil {
		return err
	}
	srcBucket, srcKey := storageclient.SplitBucketPrefix(srcPath)
	dstBucket, dstKey := storageclient.SplitBucketPrefix(dstPath)
	if srcBucket != dstBucket {
		return fmt.Errorf("moving between buckets is unsupported")
	}
	_, err = api.MoveObject(cmd.Context(), srcBucket, srcKey, dstKey)
	return err
}

func removeStorageObjects(cmd *cobra.Command, api storage.StorageAPI, paths []string) error {
	groups := make(map[string][]string)
	for _, objectPath := range paths {
		remotePath, err := storageclient.ParseStorageURL(objectPath)
		if err != nil {
			return err
		}
		bucket, key := storageclient.SplitBucketPrefix(remotePath)
		if bucket == "" || key == "" {
			return fmt.Errorf("a bucket and object path are required")
		}
		groups[bucket] = append(groups[bucket], key)
	}
	for bucket, keys := range groups {
		if _, err := api.DeleteObjects(cmd.Context(), bucket, keys); err != nil {
			return err
		}
	}
	return nil
}
