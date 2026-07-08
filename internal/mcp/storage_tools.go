// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"strings"

	"github.com/go-errors/errors"
	storageclient "github.com/volcengine/byted-supabase-cli/internal/storage/client"
	"github.com/volcengine/byted-supabase-cli/pkg/cast"
	"github.com/volcengine/byted-supabase-cli/pkg/storage"
)

// storageTools registers the storage group: Storage bucket management and service configuration.
//
// Builds a storage.StorageAPI via internal/storage/client.NewVolcengineStorageAPI
// (resolving the branch gateway and service-role key from volcengine.Client), then
// calls pkg/storage methods directly and serialises the results.
// This group is enabled by default (unlike the upstream default of off).
func storageTools() []toolSpec {
	return []toolSpec{
		defineTool(meta{
			name:        "list_storage_buckets",
			title:       "List storage buckets",
			feature:     featureStorage,
			description: "List the Storage buckets in a Supabase branch.",
		}, listStorageBuckets),
		defineTool(meta{
			name:        "create_storage_bucket",
			title:       "Create storage bucket",
			feature:     featureStorage,
			mutating:    true,
			description: "Create a new Storage bucket.",
		}, createStorageBucket),
		defineTool(meta{
			name:        "delete_storage_bucket",
			title:       "Delete storage bucket",
			feature:     featureStorage,
			mutating:    true,
			description: "Delete a Storage bucket.",
		}, deleteStorageBucket),
		defineTool(meta{
			name:        "get_storage_config",
			title:       "Get storage config",
			feature:     featureStorage,
			description: "Get the Storage service configuration: the global per-file size limit (fileSizeLimit) and the total storage limit across all buckets (totalFileSizeLimit).",
		}, getStorageConfig),
	}
}

// storageTarget is the workspace/branch selector shared by all storage tools.
type storageTarget struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

// bucketView is the slim projection returned to the client.
type bucketView struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Public           bool     `json:"public"`
	Owner            string   `json:"owner,omitempty"`
	FileSizeLimit    *int     `json:"file_size_limit,omitempty"`
	AllowedMimeTypes []string `json:"allowed_mime_types,omitempty"`
	CreatedAt        string   `json:"created_at,omitempty"`
	UpdatedAt        string   `json:"updated_at,omitempty"`
}

func toBucketView(b storage.BucketResponse) bucketView {
	return bucketView{
		ID:               b.Id,
		Name:             b.Name,
		Public:           b.Public,
		Owner:            b.Owner,
		FileSizeLimit:    b.FileSizeLimit,
		AllowedMimeTypes: b.AllowedMimeTypes,
		CreatedAt:        b.CreatedAt,
		UpdatedAt:        b.UpdatedAt,
	}
}

func listStorageBuckets(ctx context.Context, p *policy, in storageTarget) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	storageAPI, err := storageclient.NewVolcengineStorageAPI(ctx, client, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	buckets, err := storageAPI.ListBuckets(ctx)
	if err != nil {
		return "", err
	}
	views := make([]bucketView, 0, len(buckets))
	for _, b := range buckets {
		views = append(views, toBucketView(b))
	}
	return toJSON(map[string]any{
		"buckets": views,
		"count":   len(views),
	})
}

type createStorageBucketInput struct {
	Name        string `json:"name" jsonschema:"name for the new bucket; also serves as its id and cannot be changed after creation"`
	Public      bool   `json:"public,omitempty" jsonschema:"whether the bucket is publicly readable; defaults to false"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func createStorageBucket(ctx context.Context, p *policy, in createStorageBucketInput) (string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "", errors.New("name is required")
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	storageAPI, err := storageclient.NewVolcengineStorageAPI(ctx, client, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	if _, err := storageAPI.CreateBucket(ctx, storage.CreateBucketRequest{
		Name:   name,
		Public: cast.Ptr(in.Public),
	}); err != nil {
		return "", err
	}
	bucket, err := storageAPI.GetBucket(ctx, name)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success": true,
		"bucket":  toBucketView(bucket),
	})
}

type deleteStorageBucketInput struct {
	Name        string `json:"name" jsonschema:"name (id) of the bucket to delete"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func deleteStorageBucket(ctx context.Context, p *policy, in deleteStorageBucketInput) (string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "", errors.New("name is required")
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	storageAPI, err := storageclient.NewVolcengineStorageAPI(ctx, client, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	if _, err := storageAPI.DeleteBucket(ctx, name); err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success": true,
		"message": "Bucket deleted successfully",
		"name":    name,
	})
}

func getStorageConfig(ctx context.Context, p *policy, in storageTarget) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	storageAPI, err := storageclient.NewVolcengineStorageAPI(ctx, client, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	cfg, err := storageAPI.GetStorageConfig(ctx)
	if err != nil {
		return "", err
	}
	return toJSON(cfg)
}
