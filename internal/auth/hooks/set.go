// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package hooks

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

// RunVolcengineSet modifies Auth Hooks config.
func RunVolcengineSet(ctx context.Context, client *volcengine.Client, workspaceID, branchID string, pairs []string, fromJSON string, fsys afero.Fs) error {
	patch, err := buildPatchBody(pairs, fromJSON, fsys)
	if err != nil {
		return err
	}
	if len(patch) == 0 {
		return errors.New("no hooks config values provided. Use KEY=VALUE arguments or --from-json <file>.")
	}
	return patchHooksConfig(ctx, client, workspaceID, branchID, patch)
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
		pair := strings.TrimSpace(arg)
		if pair == "" {
			continue
		}
		idx := strings.IndexByte(pair, '=')
		if idx < 1 {
			return nil, errors.Errorf("invalid config pair %q: expected KEY=VALUE format", pair)
		}
		key := pair[:idx]
		value := pair[idx+1:]
		var parsed interface{}
		if err := json.Unmarshal([]byte(value), &parsed); err == nil {
			patch[key] = parsed
		} else {
			patch[key] = value
		}
	}
	return patch, nil
}

// SetHooksConfig applies patch to the Auth Hooks config and returns the resulting
// values for the patched keys. Data-layer helper shared by the CLI (RunVolcengineSet)
// and the MCP server.
func SetHooksConfig(ctx context.Context, client *volcengine.Client, workspaceID, branchID string, patch map[string]interface{}) (map[string]interface{}, error) {
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return nil, errors.Errorf("failed to marshal hooks config patch: %w", err)
	}
	respBody, err := access.DoRequest(ctx, http.MethodPatch, "/auth/v1/config/hooks", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var fullConfig map[string]interface{}
	if err := json.Unmarshal(respBody, &fullConfig); err != nil {
		return nil, errors.Errorf("failed to decode hooks config patch response: %w", err)
	}
	result := make(map[string]interface{}, len(patch))
	for k := range patch {
		if v, ok := fullConfig[k]; ok {
			result[k] = v
		} else {
			result[k] = patch[k]
		}
	}
	return result, nil
}

func patchHooksConfig(ctx context.Context, client *volcengine.Client, workspaceID, branchID string, patch map[string]interface{}) error {
	result, err := SetHooksConfig(ctx, client, workspaceID, branchID, patch)
	if err != nil {
		return err
	}
	return outputHooksConfig(result)
}
