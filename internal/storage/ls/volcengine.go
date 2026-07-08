// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package ls

import (
	"context"
	"fmt"

	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/storage/client"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// RunVolcengine lists remote Storage paths through the branch Storage gateway.
func RunVolcengine(ctx context.Context, api *volcengine.Client, workspaceID, branchID, objectPath string, recursive bool, fsys afero.Fs) error {
	remotePath, err := client.ParseStorageURL(objectPath)
	if err != nil {
		return err
	}
	storageAPI, err := client.NewVolcengineStorageAPI(ctx, api, workspaceID, branchID)
	if err != nil {
		return err
	}
	callback := func(objectPath string) error {
		fmt.Println(objectPath)
		return nil
	}
	if recursive {
		return IterateStoragePathsAll(ctx, storageAPI, remotePath, callback)
	}
	return IterateStoragePaths(ctx, storageAPI, remotePath, callback)
}
