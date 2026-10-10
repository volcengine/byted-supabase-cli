// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSandboxToolsListed(t *testing.T) {
	names := listToolNames(t, connect(t, Options{Features: []string{featureSandbox}}))
	for _, name := range []string{
		"list_sandbox_templates",
		"list_sandboxes",
		"list_sandbox_user_summaries",
		"get_sandbox_global_quota",
		"set_sandbox_global_quota",
	} {
		assert.True(t, names[name], "expected %s to be listed", name)
	}
}

func TestSandboxFeatureGating(t *testing.T) {
	names := listToolNames(t, connect(t, Options{Features: []string{featureDatabase}}))
	assert.False(t, names["list_sandboxes"])
	assert.True(t, names["execute_sql"])
}

func TestSandboxToolsRemainVisibleInReadOnlyMode(t *testing.T) {
	names := listToolNames(t, connect(t, Options{ReadOnly: true, Features: []string{featureSandbox}}))
	assert.True(t, names["list_sandbox_templates"])
	assert.True(t, names["list_sandboxes"])
	assert.True(t, names["list_sandbox_user_summaries"])
	assert.True(t, names["get_sandbox_global_quota"])
	assert.False(t, names["set_sandbox_global_quota"])
}
