// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunConsoleLoginImportsCredentialFile(t *testing.T) {
	home := t.TempDir()
	cacheDir := filepath.Join(home, "login-cache")
	t.Setenv("HOME", home)
	t.Setenv(loginCacheDirectoryEnv, cacheDir)

	sourcePath := writeLoginCredentialFile(t, LoginTokenCache{
		LoginSession: "import-session",
		AccessToken:  testLoginAccessToken(t),
		RefreshToken: "refresh-token",
		Scope:        ScopeAllAll,
		ClientID:     consoleClientIDSameDevice,
		EndpointURL:  DefaultConsoleEndpoint,
		IssuedAt:     time.Now().UTC().Format(time.RFC3339),
		ExpiresIn:    3600,
		TokenType:    "Bearer",
	})
	var output bytes.Buffer
	err := RunConsoleLogin(context.Background(), ConsoleLoginParams{
		Profile:        "imported",
		Region:         "cn-beijing",
		CredentialFile: sourcePath,
		AgentPlanConfig: &SupabaseProfileConfig{
			IsAgentPlan:     true,
			AgentPlanSeatID: "seat-123",
		},
	}, bytes.NewReader(nil), &output)
	require.NoError(t, err)

	cfg, err := LoadFileConfig()
	require.NoError(t, err)
	assert.Equal(t, "imported", cfg.Current)
	require.Contains(t, cfg.Profiles, "imported")
	assert.Equal(t, &Profile{
		Name:         "imported",
		Mode:         ModeConsoleLogin,
		Region:       "cn-beijing",
		LoginSession: "import-session",
	}, cfg.Profiles["imported"])

	importedCache, err := readLoginCache("import-session")
	require.NoError(t, err)
	assert.Equal(t, "import-session", importedCache.LoginSession)
	assert.JSONEq(t, string(testLoginAccessToken(t)), string(importedCache.AccessToken))

	supabaseCfg, err := LoadSupabaseFileConfig()
	require.NoError(t, err)
	assert.Equal(t, SupabaseProfileConfig{
		IsAgentPlan:     true,
		AgentPlanSeatID: "seat-123",
	}, supabaseCfg["imported"])
	assert.Contains(t, output.String(), "Credentials cached for profile: imported")
}

func TestRunConsoleLoginClearsExistingAgentPlanConfigWhenOmitted(t *testing.T) {
	home := t.TempDir()
	cacheDir := filepath.Join(home, "login-cache")
	t.Setenv("HOME", home)
	t.Setenv(loginCacheDirectoryEnv, cacheDir)

	require.NoError(t, SetSupabaseProfileConfig("imported", SupabaseProfileConfig{
		IsAgentPlan:     true,
		AgentPlanSeatID: "seat-stale",
	}))
	require.NoError(t, SetSupabaseProfileConfig("other", SupabaseProfileConfig{
		IsAgentPlan: true,
	}))

	sourcePath := writeLoginCredentialFile(t, LoginTokenCache{
		LoginSession: "import-session",
		AccessToken:  testLoginAccessToken(t),
		RefreshToken: "refresh-token",
		Scope:        ScopeAllAll,
		ClientID:     consoleClientIDSameDevice,
		EndpointURL:  DefaultConsoleEndpoint,
		IssuedAt:     time.Now().UTC().Format(time.RFC3339),
		ExpiresIn:    3600,
		TokenType:    "Bearer",
	})
	err := RunConsoleLogin(context.Background(), ConsoleLoginParams{
		Profile:        "imported",
		Region:         "cn-beijing",
		CredentialFile: sourcePath,
	}, bytes.NewReader(nil), &bytes.Buffer{})
	require.NoError(t, err)

	supabaseCfg, err := LoadSupabaseFileConfig()
	require.NoError(t, err)
	assert.NotContains(t, supabaseCfg, "imported")
	assert.Equal(t, SupabaseProfileConfig{IsAgentPlan: true}, supabaseCfg["other"])
}

func TestImportConsoleLoginCredentialFileRejectsExpiredSessionWithoutRefreshToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(loginCacheDirectoryEnv, filepath.Join(home, "login-cache"))

	sourcePath := writeLoginCredentialFile(t, LoginTokenCache{
		LoginSession: "expired-session",
		AccessToken:  testLoginAccessToken(t),
		Scope:        ScopeAllAll,
		ClientID:     consoleClientIDSameDevice,
		EndpointURL:  DefaultConsoleEndpoint,
		IssuedAt:     time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339),
		ExpiresIn:    3600,
		TokenType:    "Bearer",
	})
	_, err := importConsoleLoginCredentialFile(context.Background(), ConsoleLoginParams{
		Profile:        "expired",
		Region:         "cn-beijing",
		CredentialFile: sourcePath,
	}, bytes.NewReader(nil), &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired or near expiry")

	cfg, loadErr := LoadFileConfig()
	require.NoError(t, loadErr)
	assert.NotContains(t, cfg.Profiles, "expired")
}

func writeLoginCredentialFile(t *testing.T, cache LoginTokenCache) string {
	t.Helper()
	data, err := json.Marshal(cache)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "credential.json")
	require.NoError(t, os.WriteFile(path, data, 0600))
	return path
}

func testLoginAccessToken(t *testing.T) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(STSCredentials{
		AccessKeyID:     "test-ak",
		SecretAccessKey: "test-sk",
		SessionToken:    "test-session-token",
	})
	require.NoError(t, err)
	return data
}
