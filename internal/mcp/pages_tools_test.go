// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/pages"
)

// fast_create_pages must surface the same demo app archive the CLI `pages fast create`
// help shows — both reference the shared pages.DemoAppArchiveURL, so this guards against
// a future literal sneaking back in and drifting from the CLI.
func TestFastCreatePagesDescriptionHasDemoURL(t *testing.T) {
	var desc string
	for _, s := range pagesTools() {
		if s.meta.name == "fast_create_pages" {
			desc = s.meta.description
		}
	}
	require.NotEmpty(t, desc)
	assert.Contains(t, desc, pages.DemoAppArchiveURL)
}

func TestPagesToolsMeta(t *testing.T) {
	specs := pagesTools()

	byName := make(map[string]meta, len(specs))
	for _, s := range specs {
		byName[s.meta.name] = s.meta
	}
	require.Len(t, specs, 11)

	want := map[string]bool{ // name -> mutating
		"list_pages_projects":    false,
		"list_pages_deployments": false,
		"get_pages_env_vars":     false,
		"get_pages_binding":      false,
		"upload_pages_resource":  true,
		"create_pages_project":   true,
		"deploy_pages_project":   true,
		"bind_pages_project":     true,
		"unbind_pages_project":   true,
		"sync_pages_env_vars":    true,
		"fast_create_pages":      true,
	}
	for name, mutating := range want {
		m, ok := byName[name]
		require.True(t, ok, "expected tool %s to be defined", name)
		assert.Equal(t, featurePages, m.feature, "tool %s should belong to the pages feature", name)
		assert.Equal(t, mutating, m.mutating, "tool %s mutating flag mismatch", name)
	}
}

// Preview-URL guidance must reach the model two ways: in the tool descriptions it reads
// while planning, and in the note/next_step strings returned in results. This locks in the
// load-bearing tokens so the bare-domain-vs-openable distinction can't silently regress.
func TestPagesToolsPreviewGuidance(t *testing.T) {
	desc := make(map[string]string)
	for _, s := range pagesTools() {
		desc[s.meta.name] = s.meta.description
	}
	// The list tools return the bare (non-openable) domain — say so and route onward.
	for _, name := range []string{"list_pages_projects", "list_pages_deployments"} {
		assert.Contains(t, desc[name], "NOT directly openable", name)
		assert.Contains(t, desc[name], "get_pages_binding", name)
	}
	// deploy/bind return no URL — both must point to get_pages_binding next.
	assert.Contains(t, desc["deploy_pages_project"], "get_pages_binding")
	assert.Contains(t, desc["bind_pages_project"], "get_pages_binding")
	// get_pages_binding is the openable source.
	assert.Contains(t, desc["get_pages_binding"], "openable preview URL")

	// Result-borne guidance strings.
	assert.Contains(t, pagesBareDomainNote, "get_pages_binding")
	assert.Contains(t, pagesBareDomainNote, "will NOT open")
	assert.Contains(t, pagesBindingOpenableNote, "openable site URL")
	assert.Contains(t, pagesDeployNextStep, "get_pages_binding")
	assert.Contains(t, pagesDeployNextStep, "no site URL")
}

// The pages group is on by default and is NOT hidden under --workspace-ref (unlike account),
// because project/upload/deploy tools are account-level.
func TestPagesToolsExposedByDefaultAndUnderWorkspaceRef(t *testing.T) {
	def := listToolNames(t, connect(t, Options{}))
	assert.True(t, def["create_pages_project"], "pages tools must be on by default")
	assert.True(t, def["list_pages_projects"])

	scoped := listToolNames(t, connect(t, Options{WorkspaceRef: "ws-123"}))
	assert.True(t, scoped["create_pages_project"], "pages tools stay visible under --workspace-ref")
	assert.False(t, scoped["list_workspaces"], "account group is still hidden under --workspace-ref")
}

// Required-argument validation runs before any client/credential resolution, so these
// reach an error without touching the network or triggering the login flow.

func TestCreatePagesProjectValidatesName(t *testing.T) {
	p := newPolicy(Options{Features: []string{featurePages}})
	_, err := createPagesProject(context.Background(), &p, createPagesProjectInput{Name: "Bad_Name", ResourceID: "res-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pages project name")
}

func TestCreatePagesProjectRequiresResourceID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featurePages}})
	_, err := createPagesProject(context.Background(), &p, createPagesProjectInput{Name: "my-site"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource_id is required")
}

func TestCreatePagesProjectRejectsInvalidScope(t *testing.T) {
	p := newPolicy(Options{Features: []string{featurePages}})
	_, err := createPagesProject(context.Background(), &p, createPagesProjectInput{Name: "my-site", ResourceID: "res-1", Scope: "mars"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scope")
}

func TestDeployPagesProjectRequiresProjectID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featurePages}})
	_, err := deployPagesProject(context.Background(), &p, deployPagesProjectInput{ResourceID: "res-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pages_project_id is required")
}

func TestDeployPagesProjectRequiresResourceID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featurePages}})
	_, err := deployPagesProject(context.Background(), &p, deployPagesProjectInput{PagesProjectID: "pp-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource_id is required")
}

func TestUploadPagesResourceRequiresPath(t *testing.T) {
	p := newPolicy(Options{Features: []string{featurePages}})
	_, err := uploadPagesResource(context.Background(), &p, uploadPagesResourceInput{FilePath: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file_path is required")
}

func TestListPagesDeploymentsRequiresProjectID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featurePages}})
	_, err := listPagesDeployments(context.Background(), &p, listPagesDeploymentsInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pages_project_id is required")
}

func TestBindPagesProjectRequiresProjectID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featurePages}})
	_, err := bindPagesProject(context.Background(), &p, bindPagesProjectInput{WorkspaceID: "ws-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pages_project_id is required")
}

func TestFastCreatePagesRequiresNameAndFilePath(t *testing.T) {
	p := newPolicy(Options{Features: []string{featurePages}})
	_, err := fastCreatePages(context.Background(), &p, fastCreatePagesInput{FilePath: "./dist"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")

	_, err = fastCreatePages(context.Background(), &p, fastCreatePagesInput{Name: "my-site"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file_path is required")
}

// fast_create_pages creates a brand-new workspace, so it is rejected (before any network
// call) when the server is hard-scoped to a single workspace.
func TestFastCreatePagesRejectedUnderWorkspaceRef(t *testing.T) {
	p := newPolicy(Options{Features: []string{featurePages}, WorkspaceRef: "ws-123"})
	_, err := fastCreatePages(context.Background(), &p, fastCreatePagesInput{Name: "my-site", FilePath: "./dist"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available when the server is scoped")
}

// Under --workspace-ref the list filter rejects a mismatched caller-supplied workspace.
func TestPagesWorkspaceFilterRejectsMismatch(t *testing.T) {
	p := newPolicy(Options{Features: []string{featurePages}, WorkspaceRef: "ws-123"})
	_, err := pagesWorkspaceFilter(&p, "ws-other")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")

	got, err := pagesWorkspaceFilter(&p, "")
	require.NoError(t, err)
	assert.Equal(t, "ws-123", got, "empty input resolves to the scoped workspace")
}
