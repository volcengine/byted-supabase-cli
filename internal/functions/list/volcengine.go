// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package list

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

type volcengineFunctionResponse struct {
	CreatedAt      string                     `json:"created_at"`
	EntrypointPath *string                    `json:"entrypoint_path,omitempty"`
	EzbrSHA256     *string                    `json:"ezbr_sha256,omitempty"`
	ID             string                     `json:"id"`
	ImportMap      *bool                      `json:"import_map,omitempty"`
	ImportMapPath  *string                    `json:"import_map_path,omitempty"`
	Name           string                     `json:"name"`
	Slug           string                     `json:"slug"`
	Status         api.FunctionResponseStatus `json:"status"`
	UpdatedAt      string                     `json:"updated_at"`
	VerifyJWT      *bool                      `json:"verify_jwt,omitempty"`
	Version        int                        `json:"version"`
}

// RunVolcengine lists Edge Functions through the selected branch gateway.
func RunVolcengine(ctx context.Context, client *volcengine.Client, workspaceID, branchID string) error {
	functions, err := GetVolcengineFunctions(ctx, client, workspaceID, branchID)
	if err != nil {
		return err
	}
	return outputFunctions(functions)
}

// GetVolcengineFunctions resolves the selected branch and fetches deployed functions.
func GetVolcengineFunctions(ctx context.Context, client *volcengine.Client, workspaceID, branchID string) ([]api.FunctionResponse, error) {
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return nil, err
	}
	return GetVolcengineFunctionsWithAccess(ctx, access)
}

// GetVolcengineFunctionsWithAccess fetches functions using already resolved branch access.
func GetVolcengineFunctionsWithAccess(ctx context.Context, access volcengine.PgMetaAccess) ([]api.FunctionResponse, error) {
	// The trailing slash is required by the edge-function-go route for /functions/v1/.
	body, err := access.DoRequest(ctx, http.MethodGet, "/functions/v1/", nil)
	if err != nil {
		return nil, err
	}
	var remoteFunctions []volcengineFunctionResponse
	if err := json.Unmarshal(body, &remoteFunctions); err != nil {
		return nil, errors.Errorf("failed to decode functions list response: %w", err)
	}
	functions := make([]api.FunctionResponse, 0, len(remoteFunctions))
	for _, remoteFunction := range remoteFunctions {
		createdAt, err := parseVolcengineTimestamp(remoteFunction.CreatedAt)
		if err != nil {
			return nil, errors.Errorf("failed to decode function %s created_at: %w", remoteFunction.Slug, err)
		}
		updatedAt, err := parseVolcengineTimestamp(remoteFunction.UpdatedAt)
		if err != nil {
			return nil, errors.Errorf("failed to decode function %s updated_at: %w", remoteFunction.Slug, err)
		}
		functions = append(functions, api.FunctionResponse{
			CreatedAt:      createdAt,
			EntrypointPath: remoteFunction.EntrypointPath,
			EzbrSha256:     remoteFunction.EzbrSHA256,
			Id:             remoteFunction.ID,
			ImportMap:      remoteFunction.ImportMap,
			ImportMapPath:  remoteFunction.ImportMapPath,
			Name:           remoteFunction.Name,
			Slug:           remoteFunction.Slug,
			Status:         remoteFunction.Status,
			UpdatedAt:      updatedAt,
			VerifyJwt:      remoteFunction.VerifyJWT,
			Version:        remoteFunction.Version,
		})
	}
	return functions, nil
}

func parseVolcengineTimestamp(value string) (int64, error) {
	timestamp, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return 0, err
	}
	return timestamp.UnixMilli(), nil
}
