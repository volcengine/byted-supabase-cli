// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseGroupListed(t *testing.T) {
	cs := connect(t, Options{Features: []string{featureDatabase}})
	names := listToolNames(t, cs)
	for _, n := range []string{"execute_sql", "list_tables", "list_extensions", "list_migrations", "apply_migration", "get_database_connection_string", "get_database_api_config", "get_database_advisors"} {
		assert.True(t, names[n], "expected %s to be listed", n)
	}
}

func TestDatabaseWriteToolsHiddenInReadOnly(t *testing.T) {
	cs := connect(t, Options{ReadOnly: true, Features: []string{featureDatabase}})
	names := listToolNames(t, cs)
	// Write tools (execute_sql for arbitrary SQL, apply_migration) are hidden in read-only mode.
	assert.False(t, names["execute_sql"], "execute_sql must be hidden in read-only")
	assert.False(t, names["apply_migration"], "apply_migration must be hidden in read-only")
	// Built-in read-only tools are still available.
	assert.True(t, names["list_tables"])
	assert.True(t, names["list_extensions"])
	assert.True(t, names["list_migrations"])
	assert.True(t, names["get_database_connection_string"])
	assert.True(t, names["get_database_api_config"])
	assert.True(t, names["get_database_advisors"])
}

func TestListTablesRejectsInvalidSchema(t *testing.T) {
	// Schema names are interpolated into SQL; non-identifiers are rejected before any database access.
	cs := connect(t, Options{Features: []string{featureDatabase}})
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_tables",
		Arguments: map[string]any{"schemas": []string{"public; drop table x"}},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "invalid schema name")
}

func TestApplyMigrationValidatesInput(t *testing.T) {
	cs := connect(t, Options{Features: []string{featureDatabase}})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "apply_migration",
		Arguments: map[string]any{"name": "  ", "query": "create table t(id int)"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "name is required")

	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "apply_migration",
		Arguments: map[string]any{"name": "add_users", "query": "   "},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "query is required")
}

// normalizeSchemas is the injection barrier before schema names are interpolated into
// SQL: it fills in defaults, trims whitespace, and rejects any non-plain-identifier.
// Exercises each branch directly without going through the network.
func TestNormalizeSchemas(t *testing.T) {
	t.Run("empty defaults to public", func(t *testing.T) {
		got, err := normalizeSchemas(nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"public"}, got)
	})
	t.Run("trims and drops blanks", func(t *testing.T) {
		got, err := normalizeSchemas([]string{"  public  ", "", "auth"})
		require.NoError(t, err)
		assert.Equal(t, []string{"public", "auth"}, got)
	})
	t.Run("all blank is rejected", func(t *testing.T) {
		_, err := normalizeSchemas([]string{"  ", ""})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one schema is required")
	})
	t.Run("rejects injection attempts", func(t *testing.T) {
		for _, bad := range []string{"public; drop table x", "a'b", "a b", "a-b", `a"b`} {
			_, err := normalizeSchemas([]string{bad})
			require.Error(t, err, "schema %q should be rejected", bad)
			assert.Contains(t, err.Error(), "invalid schema name")
		}
	})
}

func TestIsPlainIdent(t *testing.T) {
	for _, ok := range []string{"public", "auth", "_private", "schema_1", "MixedCase", "café"} {
		assert.True(t, isPlainIdent(ok), "%q should be a plain identifier", ok)
	}
	for _, bad := range []string{"a b", "a;b", "a-b", "a.b", "a'b", "", "a)b"} {
		// An empty string contains no illegal characters, so isPlainIdent considers it valid; normalizeSchemas separately strips empty entries.
		if bad == "" {
			assert.True(t, isPlainIdent(bad))
			continue
		}
		assert.False(t, isPlainIdent(bad), "%q should be rejected", bad)
	}
}
