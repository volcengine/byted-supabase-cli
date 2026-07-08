// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package query

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type VolcengineParams struct {
	WorkspaceID string
	BranchID    string
	SQL         string
	ReadOnly    bool
	Format      string
	AgentMode   bool
}

type volcengineQueryRequest struct {
	Query    string `json:"query"`
	ReadOnly bool   `json:"read_only"`
}

// RunVolcengine executes SQL through the Supabase postgres-meta route exposed by Kong.
func RunVolcengine(ctx context.Context, client *volcengine.Client, params VolcengineParams, w io.Writer) error {
	body, err := ExecuteVolcengine(ctx, client, params)
	if err != nil {
		return err
	}
	return formatLinkedResponse(w, body, params.Format, params.AgentMode)
}

// ExecuteVolcengine executes SQL through postgres-meta and returns its JSON response.
func ExecuteVolcengine(ctx context.Context, client *volcengine.Client, params VolcengineParams) ([]byte, error) {
	access, err := client.ResolvePgMetaAccess(ctx, params.WorkspaceID, params.BranchID)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(volcengineQueryRequest{Query: params.SQL, ReadOnly: params.ReadOnly})
	if err != nil {
		return nil, errors.Errorf("failed to encode query request: %w", err)
	}
	return access.DoRequest(ctx, http.MethodPost, "/postgres/query", bytes.NewReader(payload), volcengine.WithTimeout(volcengine.DBRequestTimeout))
}

func runVolcengineHTTPQuery(ctx context.Context, queryURL, serviceRoleKey, sql, format string, agentMode bool, w io.Writer) error {
	body, err := runVolcengineHTTPQueryRaw(ctx, queryURL, serviceRoleKey, sql, false)
	if err != nil {
		return err
	}
	return formatLinkedResponse(w, body, format, agentMode)
}

func runVolcengineHTTPQueryRaw(ctx context.Context, queryURL, serviceRoleKey, sql string, readOnly bool) ([]byte, error) {
	payload, err := json.Marshal(volcengineQueryRequest{Query: sql, ReadOnly: readOnly})
	if err != nil {
		return nil, errors.Errorf("failed to encode query request: %w", err)
	}
	access := volcengine.PgMetaAccess{ServiceRoleKey: serviceRoleKey}
	return access.DoRequestRaw(ctx, http.MethodPost, queryURL, bytes.NewReader(payload), volcengine.WithTimeout(volcengine.DBRequestTimeout))
}

func formatLinkedResponse(w io.Writer, body []byte, format string, agentMode bool) error {
	var rows []map[string]interface{}
	if err := json.Unmarshal(body, &rows); err != nil {
		_, err := w.Write(append(body, '\n'))
		return err
	}
	if len(rows) == 0 {
		return formatOutput(w, format, agentMode, nil, nil, nil)
	}
	cols := orderedKeys(body)
	if len(cols) == 0 {
		for key := range rows[0] {
			cols = append(cols, key)
		}
	}
	data := make([][]interface{}, len(rows))
	for i, row := range rows {
		values := make([]interface{}, len(cols))
		for j, col := range cols {
			values[j] = row[col]
		}
		data[i] = values
	}
	return formatOutput(w, format, agentMode, cols, data, nil)
}

func resolveQueryURL(endpoints []volcengine.Endpoint) (string, error) {
	baseURL, err := volcengine.ResolvePgMetaBaseURL(endpoints)
	if err != nil {
		return "", err
	}
	return volcengine.BuildPgMetaURL(baseURL, "/postgres/query")
}
