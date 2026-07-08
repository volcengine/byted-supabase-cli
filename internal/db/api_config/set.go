// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package api_config

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// RunVolcengineSet modifies DATA API (PostgREST) config.
// It GETs the current config, merges user-specified fields, then PATCHes the full config.
func RunVolcengineSet(ctx context.Context, client *volcengine.Client, workspaceID, branchID string, pairs []string, fromJSON string, fsys afero.Fs) error {
	userPatch, err := buildPatchBody(pairs, fromJSON, fsys)
	if err != nil {
		return err
	}
	if len(userPatch) == 0 {
		return errors.New("no config values provided. Use KEY=VALUE arguments or --from-json <file>.")
	}
	// Get current config first
	current, err := GetVolcengineConfigMap(ctx, client, workspaceID, branchID)
	if err != nil {
		return err
	}
	// Merge: use current config as base, override with user-specified fields
	merged := make(map[string]interface{}, len(current))
	for k, v := range current {
		merged[k] = v
	}
	for k, v := range userPatch {
		merged[k] = v
	}
	// Send full PATCH
	result, err := patchAPIConfig(ctx, client, workspaceID, branchID, merged)
	if err != nil {
		return err
	}
	// Output only the keys the user set this time
	output := make(map[string]interface{}, len(userPatch))
	for k := range userPatch {
		if v, ok := result[k]; ok {
			output[k] = v
		} else {
			output[k] = userPatch[k]
		}
	}
	return outputConfig(output)
}

func buildPatchBody(pairs []string, fromJSON string, fsys afero.Fs) (map[string]interface{}, error) {
	patch := make(map[string]interface{})
	if fromJSON != "" {
		data, err := afero.ReadFile(fsys, fromJSON)
		if err != nil {
			return nil, errors.Errorf("failed to read JSON file %q: %w", fromJSON, err)
		}
		if err := json.Unmarshal(data, &patch); err != nil {
			return nil, errors.Errorf("failed to parse JSON file %q: %w", fromJSON, err)
		}
	}
	for _, arg := range pairs {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		idx := strings.IndexByte(arg, '=')
		if idx < 1 {
			return nil, errors.Errorf("invalid config pair %q: expected KEY=VALUE format", arg)
		}
		key := arg[:idx]
		value := arg[idx+1:]
		var parsed interface{}
		if err := json.Unmarshal([]byte(value), &parsed); err == nil {
			patch[key] = parsed
		} else {
			patch[key] = value
		}
	}
	return patch, nil
}

func patchAPIConfig(ctx context.Context, client *volcengine.Client, workspaceID, branchID string, payload map[string]interface{}) (map[string]interface{}, error) {
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Errorf("failed to marshal api config patch: %w", err)
	}
	respBody, err := access.DoRequest(ctx, http.MethodPatch, "/rest/v1/config", bytes.NewReader(body), volcengine.WithTimeout(volcengine.DBRequestTimeout))
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, errors.Errorf("failed to decode api config patch response: %w", err)
	}
	return result, nil
}
