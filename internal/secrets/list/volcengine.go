// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package list

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

// RunVolcengine lists Edge Function secret digests through the selected branch gateway.
func RunVolcengine(ctx context.Context, client *volcengine.Client, workspaceID, branchID string) error {
	secrets, err := GetSecretDigestsVolcengine(ctx, client, workspaceID, branchID)
	if err != nil {
		return err
	}
	return outputSecrets(secrets)
}

// GetSecretDigestsVolcengine resolves the selected branch and fetches secret digests.
func GetSecretDigestsVolcengine(ctx context.Context, client *volcengine.Client, workspaceID, branchID string) ([]api.SecretResponse, error) {
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return nil, err
	}
	body, err := access.DoRequest(ctx, http.MethodGet, "/v1/projects/default/secrets", nil)
	if err != nil {
		return nil, err
	}
	var secrets []api.SecretResponse
	if err := json.Unmarshal(body, &secrets); err != nil {
		return nil, errors.Errorf("failed to decode secrets list response: %w", err)
	}
	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i].Name < secrets[j].Name
	})
	return secrets, nil
}
