// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"strings"

	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/errdefs"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type WorkspaceResolver struct {
	FS afero.Fs
}

func (r WorkspaceResolver) Resolve(ctx context.Context, explicitWorkspaceID string, allowLinked bool) (string, error) {
	workspaceID := strings.TrimSpace(explicitWorkspaceID)
	if workspaceID != "" {
		if err := ensureVolcengineRegion(ctx, r.FS, false); err != nil {
			return "", err
		}
		return workspaceID, nil
	}

	if allowLinked {
		linkedWorkspaceID, err := volcengine.LoadLinkedWorkspaceID(r.FS)
		if err != nil {
			return "", err
		}
		if linkedWorkspaceID != "" {
			if err := ensureVolcengineRegion(ctx, r.FS, true); err != nil {
				return "", err
			}
			return linkedWorkspaceID, nil
		}
	}

	if err := ensureVolcengineRegion(ctx, r.FS, false); err != nil {
		return "", err
	}
	return "", errdefs.ErrMissingWorkspaceID
}
