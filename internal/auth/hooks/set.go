// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// hookFieldSuffixes are the per-hook field suffixes of flat hooks config keys
// (hook_<name>_<field>). Used to derive the hook name from a key so related
// fields (enabled, uri, secrets) can be cross-validated as one hook.
var hookFieldSuffixes = []string{
	"_aggr_count_threshold",
	"_aggr_duration_threshold",
	"_enable_aggregate",
	"_enable_jwt",
	"_enabled",
	"_jwt_ttl",
	"_secrets",
	"_uri",
}

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
// values for the patched keys. The patch is validated against the current remote
// config first: unknown keys, mismatched value types, and inconsistent hook states
// (enabled without a URI, HTTP(S) URI without secrets) are rejected before anything
// is written, because the backend accepts such patches silently and the resulting
// state breaks both the hook at runtime and the console hooks page. Data-layer
// helper shared by the CLI (RunVolcengineSet) and the MCP server.
func SetHooksConfig(ctx context.Context, client *volcengine.Client, workspaceID, branchID string, patch map[string]interface{}) (map[string]interface{}, error) {
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return nil, err
	}
	current, err := fetchHooksConfig(ctx, access)
	if err != nil {
		return nil, err
	}
	if err := ValidateHooksPatch(patch, current); err != nil {
		return nil, err
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return nil, errors.Errorf("failed to marshal hooks config patch: %w", err)
	}
	// The PATCH response body is the general auth config in uppercase env-style
	// keys, not the hooks config, so it cannot be used to report results. Re-fetch
	// the hooks config instead: it has the canonical lowercase keys and reflects
	// what was actually persisted.
	if _, err := access.DoRequest(ctx, http.MethodPatch, "/auth/v1/config/hooks", bytes.NewReader(body)); err != nil {
		return nil, err
	}
	updated, err := fetchHooksConfig(ctx, access)
	if err != nil {
		return nil, errors.Errorf("hooks config was updated, but reading it back failed: %w", err)
	}
	result := make(map[string]interface{}, len(patch))
	for k := range patch {
		v, ok := updated[k]
		if !ok {
			return nil, errors.Errorf("hooks config key %q was not applied by the server; run `auth hooks get` to inspect the current config", k)
		}
		result[k] = v
	}
	return result, nil
}

// ValidateHooksPatch rejects patches the backend would accept but that leave the
// hooks config broken: unknown keys (silently ignored server-side), values whose
// JSON type differs from the current config, and per-hook states that can never
// work (enabled without a URI, an unsupported URI scheme, or an HTTP(S) hook
// without signing secrets). current is the full remote hooks config the patch is
// merged onto.
func ValidateHooksPatch(patch, current map[string]interface{}) error {
	for key, value := range patch {
		cur, ok := current[key]
		if !ok {
			return unknownKeyError(key, current)
		}
		if err := checkValueType(key, value, cur); err != nil {
			return err
		}
	}
	merged := make(map[string]interface{}, len(current))
	for k, v := range current {
		merged[k] = v
	}
	for k, v := range patch {
		merged[k] = v
	}
	for _, name := range hookNamesInPatch(patch) {
		if err := validateHookState(name, merged); err != nil {
			return err
		}
	}
	return nil
}

func unknownKeyError(key string, current map[string]interface{}) error {
	var suggestions []string
	for k := range current {
		if strings.HasPrefix(k, key) || strings.HasPrefix(key, k) {
			suggestions = append(suggestions, k)
		}
	}
	sort.Strings(suggestions)
	if len(suggestions) > 0 {
		return errors.Errorf("unknown hooks config key %q. Did you mean one of: %s?", key, strings.Join(suggestions, ", "))
	}
	return errors.Errorf("unknown hooks config key %q. Run `auth hooks get` to list valid keys.", key)
}

func checkValueType(key string, value, current interface{}) error {
	switch current.(type) {
	case bool:
		if _, ok := value.(bool); !ok {
			return errors.Errorf("invalid value for %q: expected true or false, got %v", key, jsonValue(value))
		}
	case float64:
		if _, ok := value.(float64); !ok {
			return errors.Errorf("invalid value for %q: expected a number, got %v", key, jsonValue(value))
		}
	case string:
		if _, ok := value.(string); !ok {
			return errors.Errorf("invalid value for %q: expected a string, got %v", key, jsonValue(value))
		}
	}
	return nil
}

func jsonValue(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

// hookNamesInPatch returns the sorted hook names (e.g. "send_email") touched by
// the patch. Keys that don't match the hook_<name>_<field> shape are skipped.
func hookNamesInPatch(patch map[string]interface{}) []string {
	seen := make(map[string]struct{})
	for key := range patch {
		trimmed, ok := strings.CutPrefix(key, "hook_")
		if !ok {
			continue
		}
		for _, suffix := range hookFieldSuffixes {
			if name, ok := strings.CutSuffix(trimmed, suffix); ok && name != "" {
				seen[name] = struct{}{}
				break
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// validateHookState checks the post-patch state of one hook for combinations
// that the backend stores without complaint but that leave the hook unusable.
func validateHookState(name string, merged map[string]interface{}) error {
	enabled, _ := merged["hook_"+name+"_enabled"].(bool)
	uri, _ := merged["hook_"+name+"_uri"].(string)
	secrets, _ := merged["hook_"+name+"_secrets"].(string)

	scheme := ""
	if uri != "" {
		parsed, err := url.Parse(uri)
		if err != nil {
			return errors.Errorf("invalid hook_%s_uri %q: %v", name, uri, err)
		}
		scheme = strings.ToLower(parsed.Scheme)
		switch scheme {
		case "pg-functions", "http", "https":
		default:
			return errors.Errorf("invalid hook_%s_uri %q: only pg-functions:// and http(s):// URIs are supported", name, uri)
		}
	}
	if !enabled {
		return nil
	}
	if uri == "" {
		return errors.Errorf("cannot enable hook %q without a URI: set hook_%s_uri in the same command", name, name)
	}
	if (scheme == "http" || scheme == "https") && secrets == "" {
		return errors.Errorf("cannot enable hook %q with an HTTP(S) URI and no signing secret: set hook_%s_secrets (format: v1,whsec_<base64-secret>) in the same command", name, name)
	}
	return nil
}

func patchHooksConfig(ctx context.Context, client *volcengine.Client, workspaceID, branchID string, patch map[string]interface{}) error {
	result, err := SetHooksConfig(ctx, client, workspaceID, branchID, patch)
	if err != nil {
		return err
	}
	return outputHooksConfig(result)
}
