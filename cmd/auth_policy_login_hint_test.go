// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func TestPrintVolcengineLoginHintOffersBothLinkPaths(t *testing.T) {
	var output bytes.Buffer

	printVolcengineLoginHint(&output)

	hint := output.String()
	require.True(t, strings.HasPrefix(hint, volcengine.MissingCredentialsHint()),
		"the interactive hint must open with the same text as the non-interactive error:\n%s", hint)
	require.Contains(t, hint, "Starting Console Login in your browser now")
	require.Contains(t, hint, "Press Ctrl+C to cancel.")
	require.Contains(t, hint, volcengine.DefaultConsoleLoginRegion)
	require.Contains(t, hint, "byted-supabase-cli configure region <region>")
	require.NotContains(t, hint, "Mode")
	require.NotContains(t, hint, "mode")
}

func TestMissingCredentialsHintCoversBothLinkPaths(t *testing.T) {
	hint := volcengine.MissingCredentialsHint()

	require.Contains(t, hint, "No Volcengine credentials found.")
	require.Contains(t, hint, "If you have a Volcengine account, sign in first:")
	require.Contains(t, hint, "byted-supabase-cli login")
	require.Contains(t, hint, "byted-supabase-cli configure set --access-key <ak> --secret-key <sk> --region <region>")
	require.Contains(t, hint, volcengine.EnvAccessKeyID)
	require.Contains(t, hint, volcengine.EnvSecretAccessKey)
	require.Contains(t, hint, "If you don't, but you were given a Supabase branch's URL and API keys:")
	require.Contains(t, hint, "byted-supabase-cli link --supabase-url <url> --anon-key <key> --service-role-key <key>")
}
