// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package hooks

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

// currentConfig mirrors the flat key shape returned by GET /auth/v1/config/hooks.
func currentConfig() map[string]interface{} {
	cfg := make(map[string]interface{})
	for _, name := range []string{"send_email", "send_sms", "custom_access_token"} {
		cfg["hook_"+name+"_enabled"] = false
		cfg["hook_"+name+"_uri"] = ""
		cfg["hook_"+name+"_secrets"] = ""
		cfg["hook_"+name+"_enable_jwt"] = false
		cfg["hook_"+name+"_jwt_ttl"] = float64(0)
		cfg["hook_"+name+"_enable_aggregate"] = false
		cfg["hook_"+name+"_aggr_count_threshold"] = float64(0)
		cfg["hook_"+name+"_aggr_duration_threshold"] = float64(0)
	}
	return cfg
}

func TestValidateHooksPatch(t *testing.T) {
	t.Run("accepts enabling a pg-functions hook", func(t *testing.T) {
		err := ValidateHooksPatch(map[string]interface{}{
			"hook_custom_access_token_enabled": true,
			"hook_custom_access_token_uri":     "pg-functions://postgres/public/custom_access_token_hook",
		}, currentConfig())
		assert.NoError(t, err)
	})

	t.Run("accepts enabling an https hook with secrets", func(t *testing.T) {
		err := ValidateHooksPatch(map[string]interface{}{
			"hook_send_email_enabled": true,
			"hook_send_email_uri":     "https://example.com/hook",
			"hook_send_email_secrets": "v1,whsec_dGVzdA==",
		}, currentConfig())
		assert.NoError(t, err)
	})

	t.Run("accepts disabling a hook without other fields", func(t *testing.T) {
		current := currentConfig()
		current["hook_send_sms_enabled"] = true
		current["hook_send_sms_uri"] = "https://example.com/hook"
		err := ValidateHooksPatch(map[string]interface{}{
			"hook_send_sms_enabled": false,
		}, current)
		assert.NoError(t, err)
	})

	t.Run("accepts enabling when uri is already configured", func(t *testing.T) {
		current := currentConfig()
		current["hook_custom_access_token_uri"] = "pg-functions://postgres/public/hook"
		err := ValidateHooksPatch(map[string]interface{}{
			"hook_custom_access_token_enabled": true,
		}, current)
		assert.NoError(t, err)
	})

	t.Run("rejects unknown key with suggestions", func(t *testing.T) {
		err := ValidateHooksPatch(map[string]interface{}{
			"hook_send_email_enable": true,
		}, currentConfig())
		assert.ErrorContains(t, err, `unknown hooks config key "hook_send_email_enable"`)
		assert.ErrorContains(t, err, "hook_send_email_enabled")
	})

	t.Run("rejects unknown key without close match", func(t *testing.T) {
		err := ValidateHooksPatch(map[string]interface{}{
			"totally_bogus": true,
		}, currentConfig())
		assert.ErrorContains(t, err, `unknown hooks config key "totally_bogus"`)
		assert.ErrorContains(t, err, "auth hooks get")
	})

	t.Run("rejects wrong value type for boolean key", func(t *testing.T) {
		err := ValidateHooksPatch(map[string]interface{}{
			"hook_send_email_enabled": "yes",
		}, currentConfig())
		assert.ErrorContains(t, err, "expected true or false")
	})

	t.Run("rejects wrong value type for numeric key", func(t *testing.T) {
		err := ValidateHooksPatch(map[string]interface{}{
			"hook_send_email_jwt_ttl": true,
		}, currentConfig())
		assert.ErrorContains(t, err, "expected a number")
	})

	t.Run("rejects wrong value type for string key", func(t *testing.T) {
		err := ValidateHooksPatch(map[string]interface{}{
			"hook_send_email_uri": float64(42),
		}, currentConfig())
		assert.ErrorContains(t, err, "expected a string")
	})

	t.Run("rejects enabling a hook without uri", func(t *testing.T) {
		err := ValidateHooksPatch(map[string]interface{}{
			"hook_send_sms_enabled": true,
		}, currentConfig())
		assert.ErrorContains(t, err, `cannot enable hook "send_sms" without a URI`)
	})

	t.Run("rejects clearing uri of an enabled hook", func(t *testing.T) {
		current := currentConfig()
		current["hook_send_sms_enabled"] = true
		current["hook_send_sms_uri"] = "pg-functions://postgres/public/hook"
		err := ValidateHooksPatch(map[string]interface{}{
			"hook_send_sms_uri": "",
		}, current)
		assert.ErrorContains(t, err, `cannot enable hook "send_sms" without a URI`)
	})

	t.Run("rejects http hook without secrets", func(t *testing.T) {
		err := ValidateHooksPatch(map[string]interface{}{
			"hook_send_email_enabled": true,
			"hook_send_email_uri":     "http://example.com/hook",
		}, currentConfig())
		assert.ErrorContains(t, err, "no signing secret")
		assert.ErrorContains(t, err, "hook_send_email_secrets")
	})

	t.Run("rejects unsupported uri scheme", func(t *testing.T) {
		err := ValidateHooksPatch(map[string]interface{}{
			"hook_send_email_uri": "ftp://example.com/hook",
		}, currentConfig())
		assert.ErrorContains(t, err, "only pg-functions:// and http(s):// URIs are supported")
	})
}

func TestBuildPatchBody(t *testing.T) {
	t.Run("parses typed values from pairs", func(t *testing.T) {
		patch, err := buildPatchBody([]string{
			"hook_send_email_enabled=true",
			"hook_send_email_jwt_ttl=300",
			"hook_send_email_uri=https://example.com/hook",
		}, "", afero.NewMemMapFs())
		assert.NoError(t, err)
		assert.Equal(t, map[string]interface{}{
			"hook_send_email_enabled": true,
			"hook_send_email_jwt_ttl": float64(300),
			"hook_send_email_uri":     "https://example.com/hook",
		}, patch)
	})

	t.Run("merges json file with pair overrides", func(t *testing.T) {
		fsys := afero.NewMemMapFs()
		assert.NoError(t, afero.WriteFile(fsys, "patch.json", []byte(`{"hook_send_sms_enabled": true, "hook_send_sms_uri": "https://a.example.com"}`), 0600))
		patch, err := buildPatchBody([]string{"hook_send_sms_uri=https://b.example.com"}, "patch.json", fsys)
		assert.NoError(t, err)
		assert.Equal(t, map[string]interface{}{
			"hook_send_sms_enabled": true,
			"hook_send_sms_uri":     "https://b.example.com",
		}, patch)
	})

	t.Run("rejects malformed pair", func(t *testing.T) {
		_, err := buildPatchBody([]string{"=value"}, "", afero.NewMemMapFs())
		assert.ErrorContains(t, err, "expected KEY=VALUE format")
	})
}
