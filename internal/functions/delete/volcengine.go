// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package delete

import (
	"context"
	"fmt"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/functions/gateway"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// RunVolcengine deletes an Edge Function through the selected branch gateway.
func RunVolcengine(ctx context.Context, client *volcengine.Client, workspaceID, branchID, slug string) error {
	if err := utils.ValidateFunctionSlug(slug); err != nil {
		return err
	}
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return err
	}
	if err := gateway.Delete(ctx, access, slug); err != nil {
		if gateway.IsNotFound(err) {
			return errors.Errorf("Function %s does not exist on the Volcengine Supabase project: %w", slug, ErrNoDelete)
		}
		return err
	}
	fmt.Printf("Deleted Function %s from project %s.\n", utils.Aqua(slug), utils.Aqua(workspaceID))
	return nil
}
