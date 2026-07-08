// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package thirdparty

import (
	"context"
	"fmt"
	"net/http"

	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// RemoveProvider deletes a Third-Party Auth provider by ID. Data-layer helper shared by
// the CLI (RunVolcengineRemove) and the MCP server.
func RemoveProvider(ctx context.Context, client *volcengine.Client, workspaceID, branchID, providerID string) error {
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return err
	}
	if _, err := access.DoRequest(ctx, http.MethodDelete, "/auth/v1/config/third-party-auth/"+providerID, nil); err != nil {
		return err
	}
	return nil
}

func RunVolcengineRemove(ctx context.Context, client *volcengine.Client, workspaceID, branchID, providerID string) error {
	if err := RemoveProvider(ctx, client, workspaceID, branchID, providerID); err != nil {
		return err
	}
	fmt.Printf("Successfully removed third-party auth provider: %s\n", providerID)
	return nil
}
