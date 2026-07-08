// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package api_config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// RunVolcengineGet retrieves DATA API (PostgREST) config and outputs to stdout.
func RunVolcengineGet(ctx context.Context, client *volcengine.Client, workspaceID, branchID string, keys []string) error {
	configMap, err := GetVolcengineConfigMap(ctx, client, workspaceID, branchID)
	if err != nil {
		return err
	}
	// Filter out internal fields
	delete(configMap, "config_source")
	if len(keys) > 0 {
		filtered := make(map[string]interface{})
		for k, v := range configMap {
			for _, pattern := range keys {
				if strings.Contains(strings.ToUpper(k), strings.ToUpper(pattern)) {
					filtered[k] = v
					break
				}
			}
		}
		if len(filtered) == 0 {
			return errors.Errorf("no config keys matched filter %v", keys)
		}
		configMap = filtered
	}
	return outputConfig(configMap)
}

func GetVolcengineConfigMap(ctx context.Context, client *volcengine.Client, workspaceID, branchID string) (map[string]interface{}, error) {
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return nil, err
	}
	body, err := access.DoRequest(ctx, http.MethodGet, "/rest/v1/config", nil, volcengine.WithTimeout(volcengine.DBRequestTimeout))
	if err != nil {
		return nil, err
	}
	var configMap map[string]interface{}
	if err := json.Unmarshal(body, &configMap); err != nil {
		return nil, errors.Errorf("failed to decode api config response: %w", err)
	}
	return configMap, nil
}

func outputConfig(configMap map[string]interface{}) error {
	keys := make([]string, 0, len(configMap))
	for k := range configMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		table := "|KEY|VALUE|\n|-|-|\n"
		for _, k := range keys {
			v := formatValue(configMap[k])
			table += fmt.Sprintf("|`%s`|`%s`|\n", escape(k), escape(v))
		}
		return utils.RenderTable(table)
	default:
		sorted := make(map[string]interface{}, len(keys))
		for _, k := range keys {
			sorted[k] = configMap[k]
		}
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, sorted)
	}
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "`", "\\`")
	return s
}
