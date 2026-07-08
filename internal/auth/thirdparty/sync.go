// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package thirdparty

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// SyncProviders triggers an immediate JWKS public key sync for all Third-Party Auth
// providers. Data-layer helper shared by the CLI (RunVolcengineSync) and the MCP server.
func SyncProviders(ctx context.Context, client *volcengine.Client, workspaceID, branchID string) error {
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return err
	}
	if _, err := access.DoRequest(ctx, http.MethodGet, "/auth/v1/config/third-party-auth/sync", nil, volcengine.WithTimeout(30*time.Second)); err != nil {
		return err
	}
	return nil
}

func RunVolcengineSync(ctx context.Context, client *volcengine.Client, workspaceID, branchID string) error {
	if err := SyncProviders(ctx, client, workspaceID, branchID); err != nil {
		return err
	}
	fmt.Println("Successfully synced third-party auth provider JWKS keys.")
	return nil
}
