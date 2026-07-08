// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package connection

import (
	"context"
	"strings"

	"github.com/go-errors/errors"
	"github.com/jackc/pgconn"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type VolcengineParams struct {
	WorkspaceID string
	BranchID    string
}

// GetVolcengineURL resolves the managed Postgres URL for an explicit user request.
func GetVolcengineURL(ctx context.Context, client *volcengine.Client, params VolcengineParams) (string, error) {
	branchID, computeID, err := client.ResolvePrimaryDatabaseComputeID(ctx, params.WorkspaceID, params.BranchID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeDBAccountConnection(ctx, volcengine.DescribeDBAccountConnectionParams{
		WorkspaceID:  params.WorkspaceID,
		BranchID:     branchID,
		ComputeID:    computeID,
		AccountName:  "postgres",
		DatabaseName: "postgres",
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.ConnectionURL) == "" {
		return "", errors.New("volcengine database connection URL is empty")
	}
	return result.ConnectionURL, nil
}

// GetVolcengineConfig converts the managed connection response into the
// pgconn configuration expected by PostgreSQL-based CLI commands.
func GetVolcengineConfig(ctx context.Context, client *volcengine.Client, params VolcengineParams) (pgconn.Config, error) {
	connectionURL, err := GetVolcengineURL(ctx, client, params)
	if err != nil {
		return pgconn.Config{}, err
	}
	parsed, err := pgconn.ParseConfig(extractPostgresURL(connectionURL))
	if err != nil {
		return pgconn.Config{}, errors.Errorf("failed to parse volcengine database connection string: %w", err)
	}
	return *parsed, nil
}

func extractPostgresURL(connectionURL string) string {
	connectionURL = strings.TrimSpace(connectionURL)
	start := strings.Index(connectionURL, "postgresql://")
	if start < 0 {
		start = strings.Index(connectionURL, "postgres://")
	}
	if start < 0 {
		return connectionURL
	}
	connectionURL = connectionURL[start:]
	if end := strings.IndexAny(connectionURL, "'\" \t\r\n"); end >= 0 {
		connectionURL = connectionURL[:end]
	}
	return connectionURL
}
