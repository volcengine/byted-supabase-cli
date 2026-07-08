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

package buckets

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/storage/client"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/storage"
)

func Run(ctx context.Context, projectRef string, interactive bool, fsys afero.Fs) error {
	if len(projectRef) == 0 && len(utils.Config.Storage.Buckets) == 0 {
		return nil
	}
	api, err := client.NewStorageAPI(ctx, projectRef)
	if err != nil {
		return err
	}
	if err := runStandard(ctx, api, interactive, fsys); err != nil {
		return err
	}
	console := utils.NewConsole()
	if !interactive {
		console.IsTTY = false
	}
	prune := func(name string) bool {
		label := fmt.Sprintf("Bucket %s not found in %s. Do you want to prune it?", utils.Bold(name), utils.Bold(utils.ConfigPath))
		shouldPrune, err := console.PromptYesNo(ctx, label, false)
		if err != nil {
			fmt.Fprintln(utils.GetDebugLogger(), err)
		}
		return shouldPrune
	}
	if utils.Config.Storage.AnalyticsBuckets.Enabled && len(projectRef) > 0 {
		fmt.Fprintln(os.Stderr, "Updating analytics buckets...")
		if err := api.UpsertAnalyticsBuckets(ctx, utils.Config.Storage.AnalyticsBuckets.Buckets, prune); err != nil {
			return err
		}
	}
	if utils.Config.Storage.VectorBuckets.Enabled && len(projectRef) > 0 {
		fmt.Fprintln(os.Stderr, "Updating vector buckets...")
		if err := api.UpsertVectorBuckets(ctx, utils.Config.Storage.VectorBuckets.Buckets, prune); err != nil {
			return err
		}
	}
	return nil
}

// RunVolcengine seeds standard Storage buckets and objects through the branch
// gateway. Analytics and vector bucket APIs are not part of the VeSPB contract.
func RunVolcengine(ctx context.Context, api *volcengine.Client, workspaceID, branchID string, interactive bool, fsys afero.Fs) error {
	if len(utils.Config.Storage.Buckets) == 0 {
		return nil
	}
	storageAPI, err := client.NewVolcengineStorageAPI(ctx, api, workspaceID, branchID)
	if err != nil {
		return err
	}
	return runStandard(ctx, storageAPI, interactive, fsys)
}

func runStandard(ctx context.Context, api storage.StorageAPI, interactive bool, fsys afero.Fs) error {
	console := utils.NewConsole()
	if !interactive {
		console.IsTTY = false
	}
	filter := func(bucketId string) bool {
		label := fmt.Sprintf("Bucket %s already exists. Do you want to overwrite its properties?", utils.Bold(bucketId))
		shouldOverwrite, err := console.PromptYesNo(ctx, label, true)
		if err != nil {
			fmt.Fprintln(utils.GetDebugLogger(), err)
		}
		return shouldOverwrite
	}
	if err := api.UpsertBuckets(ctx, utils.Config.Storage.Buckets, filter); err != nil {
		return err
	}
	return api.UpsertObjects(ctx, utils.Config.Storage.Buckets, utils.NewRootFS(fsys))
}
