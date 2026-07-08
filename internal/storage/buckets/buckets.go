// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package buckets

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/storage/client"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/cast"
	"github.com/volcengine/byted-supabase-cli/pkg/storage"
)

type WriteParams struct {
	BucketID         string
	Public           bool
	Private          bool
	PublicSet        bool
	PrivateSet       bool
	FileSizeLimit    int64
	FileSizeLimitSet bool
	AllowedMimeTypes []string
	AllowedMimeSet   bool
}

func List(ctx context.Context, api *volcengine.Client, workspaceID, branchID string) error {
	storageAPI, err := client.NewVolcengineStorageAPI(ctx, api, workspaceID, branchID)
	if err != nil {
		return err
	}
	buckets, err := storageAPI.ListBuckets(ctx)
	if err != nil {
		return err
	}
	return outputBuckets(buckets)
}

func Get(ctx context.Context, api *volcengine.Client, workspaceID, branchID, bucketID string) error {
	storageAPI, err := client.NewVolcengineStorageAPI(ctx, api, workspaceID, branchID)
	if err != nil {
		return err
	}
	bucket, err := storageAPI.GetBucket(ctx, bucketID)
	if err != nil {
		return err
	}
	return outputBucket(bucket)
}

func Create(ctx context.Context, api *volcengine.Client, workspaceID, branchID string, params WriteParams) error {
	storageAPI, err := client.NewVolcengineStorageAPI(ctx, api, workspaceID, branchID)
	if err != nil {
		return err
	}
	req := storage.CreateBucketRequest{
		Name:             params.BucketID,
		FileSizeLimit:    params.FileSizeLimit,
		AllowedMimeTypes: params.AllowedMimeTypes,
	}
	if params.PublicSet {
		req.Public = cast.Ptr(params.Public)
	}
	if _, err := storageAPI.CreateBucket(ctx, req); err != nil {
		return err
	}
	bucket, err := storageAPI.GetBucket(ctx, params.BucketID)
	if err != nil {
		return err
	}
	if utils.OutputFormat.Value == utils.OutputPretty {
		fmt.Fprintln(os.Stderr, "Created Storage bucket:", params.BucketID)
	}
	return outputBucket(bucket)
}

func Update(ctx context.Context, api *volcengine.Client, workspaceID, branchID string, params WriteParams) error {
	if !params.PublicSet && !params.PrivateSet && !params.FileSizeLimitSet && !params.AllowedMimeSet {
		return errors.New("provide at least one of --public, --private, --file-size-limit, or --allowed-mime-type")
	}
	storageAPI, err := client.NewVolcengineStorageAPI(ctx, api, workspaceID, branchID)
	if err != nil {
		return err
	}
	req := storage.UpdateBucketRequest{Id: params.BucketID}
	if params.PublicSet {
		req.Public = cast.Ptr(params.Public)
	}
	if params.PrivateSet {
		req.Public = cast.Ptr(!params.Private)
	}
	if params.FileSizeLimitSet {
		req.FileSizeLimit = params.FileSizeLimit
	}
	if params.AllowedMimeSet {
		req.AllowedMimeTypes = params.AllowedMimeTypes
	}
	if _, err := storageAPI.UpdateBucket(ctx, req); err != nil {
		return err
	}
	bucket, err := storageAPI.GetBucket(ctx, params.BucketID)
	if err != nil {
		return err
	}
	if utils.OutputFormat.Value == utils.OutputPretty {
		fmt.Fprintln(os.Stderr, "Updated Storage bucket:", params.BucketID)
	}
	return outputBucket(bucket)
}

func Delete(ctx context.Context, api *volcengine.Client, workspaceID, branchID, bucketID string) error {
	storageAPI, err := client.NewVolcengineStorageAPI(ctx, api, workspaceID, branchID)
	if err != nil {
		return err
	}
	if _, err := storageAPI.GetBucket(ctx, bucketID); err != nil {
		return err
	}
	title := fmt.Sprintf("Do you want to delete Storage bucket %s? This action is irreversible.", utils.Aqua(bucketID))
	if shouldRun, err := utils.NewConsole().PromptYesNo(ctx, title, false); err != nil {
		return err
	} else if !shouldRun {
		return errors.New(context.Canceled)
	}
	result, err := storageAPI.DeleteBucket(ctx, bucketID)
	if err != nil {
		return err
	}
	if utils.OutputFormat.Value == utils.OutputPretty {
		fmt.Fprintln(os.Stderr, "Deleted Storage bucket:", bucketID)
		return nil
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
}

func outputBuckets(buckets []storage.BucketResponse) error {
	if utils.OutputFormat.Value != utils.OutputPretty {
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, buckets)
	}
	return outputBucketTable(buckets)
}

func outputBucket(bucket storage.BucketResponse) error {
	if utils.OutputFormat.Value != utils.OutputPretty {
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, bucket)
	}
	return outputBucketTable([]storage.BucketResponse{bucket})
}

func outputBucketTable(buckets []storage.BucketResponse) error {
	var table strings.Builder
	table.WriteString("|ID|NAME|PUBLIC|FILE SIZE LIMIT|ALLOWED MIME TYPES|CREATED AT|UPDATED AT|\n|-|-|-|-|-|-|-|\n")
	for _, bucket := range buckets {
		limit := ""
		if bucket.FileSizeLimit != nil {
			limit = fmt.Sprint(*bucket.FileSizeLimit)
		}
		fmt.Fprintf(&table, "|`%s`|`%s`|`%t`|`%s`|`%s`|`%s`|`%s`|\n",
			escape(bucket.Id),
			escape(bucket.Name),
			bucket.Public,
			limit,
			escape(strings.Join(bucket.AllowedMimeTypes, ",")),
			bucket.CreatedAt,
			bucket.UpdatedAt,
		)
	}
	return utils.RenderTable(table.String())
}

func escape(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}
