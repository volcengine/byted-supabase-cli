// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/db/advisors"
	dbapiconfig "github.com/volcengine/byted-supabase-cli/internal/db/api_config"
	dbconnection "github.com/volcengine/byted-supabase-cli/internal/db/connection"
	"github.com/volcengine/byted-supabase-cli/internal/db/query"
)

// databaseTools registers the database group.
//
// Reuses the fork-specific db/query helpers (RunVolcengine / ExecuteVolcengine,
// which already return JSON) without touching upstream shared code. Read-only tools
// follow the same code path as execute_sql but run fixed SQL.
func databaseTools() []toolSpec {
	return []toolSpec{
		defineTool(meta{
			name:        "execute_sql",
			title:       "Execute SQL",
			feature:     featureDatabase,
			mutating:    true,
			description: "Run arbitrary SQL against a Supabase branch's Postgres database and return the rows as JSON. Hidden in read-only mode — use the list_* tools for read-only access.",
		}, executeSQL),
		defineTool(meta{
			name:        "list_tables",
			title:       "List tables",
			feature:     featureDatabase,
			description: "List tables in one or more schemas (defaults to the public schema).",
		}, listTables),
		defineTool(meta{
			name:        "list_extensions",
			title:       "List extensions",
			feature:     featureDatabase,
			description: "List installed PostgreSQL extensions with their schema and version.",
		}, listExtensions),
		defineTool(meta{
			name:        "list_migrations",
			title:       "List migrations",
			feature:     featureDatabase,
			description: "List applied migrations from supabase_migrations.schema_migrations.",
		}, listMigrations),
		defineTool(meta{
			name:        "apply_migration",
			title:       "Apply migration",
			feature:     featureDatabase,
			mutating:    true,
			description: "Apply a named migration (DDL) and record it in supabase_migrations.schema_migrations.",
		}, applyMigration),
		defineTool(meta{
			name:        "get_database_connection_string",
			title:       "Get database connection string",
			feature:     featureDatabase,
			description: "Get the managed Postgres connection string for a workspace branch. Values are masked unless reveal is true.",
		}, getDatabaseConnectionString),
		defineTool(meta{
			name:        "get_database_api_config",
			title:       "Get database API config",
			feature:     featureDatabase,
			description: "Get DATA API (PostgREST) configuration for a workspace branch.",
		}, getDatabaseAPIConfig),
		defineTool(meta{
			name:        "get_database_advisors",
			title:       "Get database advisors",
			feature:     featureDatabase,
			description: "Run database security/performance advisors for a workspace branch and return matching issues.",
		}, getDatabaseAdvisors),
	}
}

// target is the workspace/branch selector shared by all database tools.
type target struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

type executeSQLInput struct {
	SQL         string `json:"sql" jsonschema:"SQL statement(s) to execute"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

type databaseConnectionStringInput struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
	Reveal      bool   `json:"reveal,omitempty" jsonschema:"return the full connection string including password; defaults to false"`
}

func getDatabaseConnectionString(ctx context.Context, p *policy, in databaseConnectionStringInput) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	url, err := dbconnection.GetVolcengineURL(ctx, client, dbconnection.VolcengineParams{
		WorkspaceID: workspaceID,
		BranchID:    strings.TrimSpace(in.BranchID),
	})
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"workspace_id":      workspaceID,
		"connection_string": maskConnectionString(url, in.Reveal),
		"masked":            !in.Reveal,
	})
}

func maskConnectionString(value string, reveal bool) string {
	if reveal {
		return value
	}
	for _, scheme := range []string{"postgresql://", "postgres://"} {
		start := strings.Index(value, scheme)
		if start < 0 {
			continue
		}
		credStart := start + len(scheme)
		at := strings.Index(value[credStart:], "@")
		if at < 0 {
			continue
		}
		at += credStart
		colon := strings.LastIndex(value[credStart:at], ":")
		if colon < 0 {
			continue
		}
		passwordStart := credStart + colon + 1
		return value[:passwordStart] + "****" + value[at:]
	}
	return value
}

type databaseAPIConfigInput struct {
	WorkspaceID string   `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string   `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
	Keys        []string `json:"keys,omitempty" jsonschema:"optional case-insensitive key filters"`
}

func getDatabaseAPIConfig(ctx context.Context, p *policy, in databaseAPIConfigInput) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	configMap, err := dbapiconfig.GetVolcengineConfigMap(ctx, client, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	delete(configMap, "config_source")
	if len(in.Keys) > 0 {
		filtered := make(map[string]interface{})
		for k, v := range configMap {
			for _, pattern := range in.Keys {
				if strings.Contains(strings.ToUpper(k), strings.ToUpper(strings.TrimSpace(pattern))) {
					filtered[k] = v
					break
				}
			}
		}
		configMap = filtered
	}
	return toJSON(map[string]any{
		"workspace_id": workspaceID,
		"config":       configMap,
	})
}

type databaseAdvisorsInput struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
	Type        string `json:"type,omitempty" jsonschema:"advisor type: all, security, or performance; defaults to all"`
	Level       string `json:"level,omitempty" jsonschema:"minimum issue level: info, warn, or error; defaults to warn"`
}

func getDatabaseAdvisors(ctx context.Context, p *policy, in databaseAdvisorsInput) (string, error) {
	advisorType := strings.TrimSpace(in.Type)
	if advisorType == "" {
		advisorType = "all"
	}
	if !contains(advisors.AllowedTypes, advisorType) {
		return "", errors.Errorf("unsupported advisor type %q", advisorType)
	}
	level := strings.TrimSpace(in.Level)
	if level == "" {
		level = "warn"
	}
	if !contains(advisors.AllowedLevels, level) {
		return "", errors.Errorf("unsupported advisor level %q", level)
	}
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	lints, err := advisors.GetVolcengineLints(ctx, client, workspaceID, strings.TrimSpace(in.BranchID), advisorType, level)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"workspace_id": workspaceID,
		"issues":       lints,
		"count":        len(lints),
	})
}

func contains(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}

// executeSQL runs arbitrary SQL and is treated as a mutating tool: it is hidden in
// read-only mode — use list_tables / list_extensions / list_migrations for read-only
// access. pg-meta's read_only flag is unreliable, so instead of relying on it the
// tool is simply removed in read-only mode.
func executeSQL(ctx context.Context, p *policy, in executeSQLInput) (string, error) {
	if strings.TrimSpace(in.SQL) == "" {
		return "", errors.New("sql is required")
	}
	return runSQL(ctx, p, target{in.WorkspaceID, in.BranchID}, in.SQL, false)
}

type listTablesInput struct {
	Schemas     []string `json:"schemas,omitempty" jsonschema:"schemas to list tables from; defaults to [public]"`
	WorkspaceID string   `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string   `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func listTables(ctx context.Context, p *policy, in listTablesInput) (string, error) {
	schemas, err := normalizeSchemas(in.Schemas)
	if err != nil {
		return "", err
	}
	sql := fmt.Sprintf(`SELECT schemaname AS schema, tablename AS name
FROM pg_tables
WHERE schemaname IN ('%s')
ORDER BY schemaname, tablename`, strings.Join(schemas, "', '"))
	return runSQL(ctx, p, target{in.WorkspaceID, in.BranchID}, sql, true)
}

func listExtensions(ctx context.Context, p *policy, in target) (string, error) {
	const sql = `SELECT e.extname AS name, n.nspname AS schema, e.extversion AS version
FROM pg_extension e
JOIN pg_namespace n ON n.oid = e.extnamespace
ORDER BY e.extname`
	return runSQL(ctx, p, in, sql, true)
}

const migrationsExistSQL = `SELECT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = 'supabase_migrations' AND table_name = 'schema_migrations'
) AS exists`

const migrationsSelectSQL = `SELECT version, name
FROM supabase_migrations.schema_migrations
ORDER BY version DESC`

func listMigrations(ctx context.Context, p *policy, in target) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	branchID := strings.TrimSpace(in.BranchID)
	// Guard against the migrations table not existing: a direct query would fail at the
	// plan stage, so probe for existence first (raw, envelope-free JSON).
	raw, err := query.ExecuteVolcengine(ctx, client, query.VolcengineParams{
		WorkspaceID: workspaceID,
		BranchID:    branchID,
		SQL:         migrationsExistSQL,
		ReadOnly:    true,
	})
	if err != nil {
		return "", err
	}
	var existRows []struct {
		Exists bool `json:"exists"`
	}
	if err := json.Unmarshal(raw, &existRows); err != nil {
		return "", errors.Errorf("failed to parse migrations probe: %w", err)
	}
	if len(existRows) == 0 || !existRows[0].Exists {
		return "[]", nil
	}
	var buf bytes.Buffer
	if err := query.RunVolcengine(ctx, client, query.VolcengineParams{
		WorkspaceID: workspaceID,
		BranchID:    branchID,
		SQL:         migrationsSelectSQL,
		ReadOnly:    true,
		Format:      "json",
		AgentMode:   true,
	}, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type applyMigrationInput struct {
	Name        string `json:"name" jsonschema:"migration name (used as the recorded label)"`
	Query       string `json:"query" jsonschema:"migration SQL (DDL) to apply"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func applyMigration(ctx context.Context, p *policy, in applyMigrationInput) (string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "", errors.New("name is required")
	}
	if strings.TrimSpace(in.Query) == "" {
		return "", errors.New("query is required")
	}
	now := time.Now().UTC()
	version := now.Format("20060102150405") + fmt.Sprintf("%06d", now.Nanosecond()/1000)
	escapedName := strings.ReplaceAll(name, "'", "''")
	sql := fmt.Sprintf(`BEGIN;
CREATE SCHEMA IF NOT EXISTS supabase_migrations;
CREATE TABLE IF NOT EXISTS supabase_migrations.schema_migrations (
    version text PRIMARY KEY,
    name text NOT NULL,
    inserted_at timestamptz NOT NULL DEFAULT now()
);
%s
INSERT INTO supabase_migrations.schema_migrations (version, name)
VALUES ('%s', '%s')
ON CONFLICT (version) DO UPDATE SET name = EXCLUDED.name;
COMMIT;`, in.Query, version, escapedName)

	if _, err := runSQL(ctx, p, target{in.WorkspaceID, in.BranchID}, sql, false); err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Migration %s applied successfully", name),
		"version": version,
		"name":    name,
	})
}

// runSQL executes SQL via the database path and returns formatted JSON (with an injection-safe envelope).
// readOnly selects a read-only transaction.
func runSQL(ctx context.Context, p *policy, t target, sql string, readOnly bool) (string, error) {
	client, workspaceID, err := p.client(ctx, t.WorkspaceID)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := query.RunVolcengine(ctx, client, query.VolcengineParams{
		WorkspaceID: workspaceID,
		BranchID:    strings.TrimSpace(t.BranchID),
		SQL:         sql,
		ReadOnly:    readOnly,
		Format:      "json",
		AgentMode:   true,
	}, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// normalizeSchemas validates and fills in the default schema list. Schema names are interpolated into SQL, so they must be plain identifiers.
func normalizeSchemas(schemas []string) ([]string, error) {
	if len(schemas) == 0 {
		return []string{"public"}, nil
	}
	out := make([]string, 0, len(schemas))
	for _, s := range schemas {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !isPlainIdent(s) {
			return nil, errors.Errorf("invalid schema name: %q", s)
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, errors.New("at least one schema is required")
	}
	return out, nil
}

func isPlainIdent(s string) bool {
	for _, r := range s {
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
