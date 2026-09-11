// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigMissingCredentialsPointsAtBothLinkPaths(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(EnvAccessKeyID, "")
	t.Setenv(EnvSecretAccessKey, "")
	t.Setenv(EnvProfile, "")

	_, err := loadConfig()

	require.Error(t, err)
	message := err.Error()
	require.Equal(t, MissingCredentialsHint(), message)
	require.Contains(t, message, "No Volcengine credentials found.")
	require.Contains(t, message, "byted-supabase-cli login")
	require.Contains(t, message, "byted-supabase-cli configure set --access-key <ak> --secret-key <sk> --region <region>")
	require.Contains(t, message, EnvAccessKeyID)
	require.Contains(t, message, EnvSecretAccessKey)
	require.Contains(t, message, "byted-supabase-cli link --supabase-url <url> --anon-key <key> --service-role-key <key>")
	require.NotContains(t, message, "`supabase configure set")
}
