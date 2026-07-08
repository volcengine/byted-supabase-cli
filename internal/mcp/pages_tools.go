// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/pages"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// Preview-URL guidance shared across the pages tools. The CLI prints equivalent stderr
// hints; here the model only sees a tool's returned JSON, so the same guidance rides
// along as `note` / `next_step` fields. The crux: `get_pages_binding` is the ONLY source
// of an openable preview URL (its pages_project.preview_domain carries a one-time token);
// the list tools return the bare, non-openable base domain, and deploy/bind return no URL.
const (
	// pagesBareDomainNote rides on list_pages_projects / list_pages_deployments results.
	pagesBareDomainNote = "The preview_domain in these results is the project's bare base domain — it carries no access token and will NOT open on its own. To get an openable preview URL, call get_pages_binding (workspace_id, branch_id) and read its pages_project.preview_domain (carries a one-time token; short-lived/single-use, re-fetch for a fresh link)."

	// pagesBindingOpenableNote rides on get_pages_binding results that include a preview domain.
	pagesBindingOpenableNote = "pages_project.preview_domain is the openable site URL — open it as-is (it carries a one-time access token), unlike the bare domain the list tools return. Short-lived/single-use; call get_pages_binding again for a fresh link."

	// pagesDeployNextStep rides on deploy_pages_project results.
	pagesDeployNextStep = "This deploy returns no site URL. Call get_pages_binding for the workspace/branch this project is bound to and read pages_project.preview_domain for the openable preview link (one-time token; short-lived/single-use — re-fetch for a fresh link)."
)

// pagesTools registers the pages group: IGA Pages (static-site hosting) projects,
// deployments, and their binding to a Supabase branch's environment variables.
//
// Like the rest of the MCP surface these are thin adapters over the fork-specific
// volcengine.Client methods and the shared pages helpers (pages.UploadResourceWithClient,
// pages.ValidateProjectName); they never call the cobra command Run() functions.
//
// Unlike the account group, this group is not hidden under --workspace-ref: the project,
// upload, and deploy tools are account-level (not workspace-bound), while the workspace-
// scoped tools (env vars, binding, bind/unbind/sync) honour the scoped workspace.
func pagesTools() []toolSpec {
	return []toolSpec{
		defineTool(meta{
			name:        "list_pages_projects",
			title:       "List Pages projects",
			feature:     featurePages,
			description: "List IGA Pages projects, optionally filtered by name or by the Supabase workspace/branch they are bound to. Note: the preview_domain in the results is the bare base domain and is NOT directly openable — only get_pages_binding returns the openable, token-carrying preview URL.",
		}, listPagesProjects),
		defineTool(meta{
			name:        "list_pages_deployments",
			title:       "List Pages deployments",
			feature:     featurePages,
			description: "List the deployments of an IGA Pages project (newest first), including status and preview domain. Note: that preview domain is the bare base domain and is NOT directly openable — use get_pages_binding for the openable, token-carrying preview URL.",
		}, listPagesDeployments),
		defineTool(meta{
			name:        "get_pages_env_vars",
			title:       "Get Pages env vars",
			feature:     featurePages,
			description: "List the Supabase environment variables available to inject into an IGA Pages deployment for a branch. Values are masked unless show_values is true.",
		}, getPagesEnvVars),
		defineTool(meta{
			name:        "get_pages_binding",
			title:       "Get Pages binding",
			feature:     featurePages,
			description: "Show the IGA Pages project bound to a Supabase branch (and which env vars are synced to it), if any. Its pages_project.preview_domain is the openable preview URL (carries a one-time token, short-lived); the list tools' preview_domain is the bare, non-openable domain.",
		}, getPagesBinding),
		defineTool(meta{
			name:        "upload_pages_resource",
			title:       "Upload Pages resource",
			feature:     featurePages,
			mutating:    true,
			description: "Upload a built static-site archive from a local file path and return its resource_id. Pass that resource_id to create_pages_project or deploy_pages_project. This is the first step of a Pages deployment (provider upload_v2).",
		}, uploadPagesResource),
		defineTool(meta{
			name:        "create_pages_project",
			title:       "Create Pages project",
			feature:     featurePages,
			mutating:    true,
			description: "Create an IGA Pages project from an uploaded resource (upload_pages_resource first to obtain resource_id). Project name must be 2-63 lowercase letters, digits, or hyphens (no leading/trailing hyphen). Set no_deploy to create without an initial deployment.",
		}, createPagesProject),
		defineTool(meta{
			name:        "deploy_pages_project",
			title:       "Deploy Pages project",
			feature:     featurePages,
			mutating:    true,
			description: "Create a new deployment of an existing IGA Pages project from an uploaded resource (upload_pages_resource first to obtain resource_id). Returns no site URL; afterward call get_pages_binding and read pages_project.preview_domain for the openable (short-lived) preview link.",
		}, deployPagesProject),
		defineTool(meta{
			name:        "bind_pages_project",
			title:       "Bind Pages project",
			feature:     featurePages,
			mutating:    true,
			description: "Bind an IGA Pages project to a Supabase branch so the branch's environment variables can be synced into Pages deployments. Use custom_prefix to bind a Pages project already used by another branch, and framework_prefix to set the client prefix for browser-exposed env vars (e.g. VITE_, NEXT_PUBLIC_). Binding alone serves nothing — call deploy_pages_project next, then get_pages_binding for the preview URL.",
		}, bindPagesProject),
		defineTool(meta{
			name:        "unbind_pages_project",
			title:       "Unbind Pages project",
			feature:     featurePages,
			mutating:    true,
			description: "Remove the IGA Pages binding from a Supabase branch.",
		}, unbindPagesProject),
		defineTool(meta{
			name:        "sync_pages_env_vars",
			title:       "Sync Pages env vars",
			feature:     featurePages,
			mutating:    true,
			description: "Sync the Supabase environment variables of a branch into its bound IGA Pages project. Fails if no Pages project is bound to the branch (bind_pages_project first).",
		}, syncPagesEnvVars),
		defineTool(meta{
			name:     "fast_create_pages",
			title:    "Fast create Pages + Supabase",
			feature:  featurePages,
			mutating: true,
			description: "One-shot: stage a frontend (a local directory is auto-zipped, or pass a prebuilt archive) and a brand-new Supabase workspace. " +
				"Uploads the frontend, creates the Pages project, creates a new Supabase workspace, waits for it to become ready, optionally applies SQL migrations (migrations_init: a directory of .sql files run in filename order) and deploys Edge Functions (functions_init: a backend root whose <backend>/functions/<slug> dirs are deployed), binds Pages to the new workspace, deploys, and waits for the deployment to succeed. " +
				"Blocks until the deployment succeeds — this can take several minutes (workspace provisioning + deploy). Creates a NEW workspace, so it is not available when the server is scoped to a single workspace (--workspace-ref). " +
				"Demo app archive to try it (download, then pass as file_path): " + pages.DemoAppArchiveURL,
		}, fastCreatePages),
	}
}

// pagesScopedInput targets a single Supabase branch for the workspace-scoped pages tools.
type pagesScopedInput struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

type listPagesProjectsInput struct {
	Name        string `json:"name,omitempty" jsonschema:"filter by Pages project name"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"optional Supabase workspace (project) id filter; applied automatically when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"optional Supabase branch id filter"`
	Count       int    `json:"count,omitempty" jsonschema:"maximum number of projects to return per page; defaults to 10"`
	Offset      int    `json:"offset,omitempty" jsonschema:"pagination offset (must be a multiple of count); defaults to 0"`
}

func listPagesProjects(ctx context.Context, p *policy, in listPagesProjectsInput) (string, error) {
	workspaceFilter, err := pagesWorkspaceFilter(p, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	client, err := p.readClient(ctx)
	if err != nil {
		return "", err
	}
	result, err := client.ListPagesProjects(volcengine.ListPagesProjectsParams{
		Name:   strings.TrimSpace(in.Name),
		Limit:  listLimit(in.Count),
		Offset: in.Offset,
		Filter: volcengine.VolcSupabaseFilter{
			WorkspaceID: workspaceFilter,
			BranchID:    strings.TrimSpace(in.BranchID),
		},
	})
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"projects": result.Projects,
		"count":    len(result.Projects),
		"total":    result.Total,
	}
	for _, project := range result.Projects {
		if strings.TrimSpace(project.PreviewDomain) != "" {
			payload["note"] = pagesBareDomainNote
			break
		}
	}
	return toJSON(payload)
}

type listPagesDeploymentsInput struct {
	PagesProjectID string `json:"pages_project_id" jsonschema:"id of the Pages project to list deployments for"`
	Count          int    `json:"count,omitempty" jsonschema:"maximum number of deployments to return per page; defaults to 10"`
	Offset         int    `json:"offset,omitempty" jsonschema:"pagination offset (must be a multiple of count); defaults to 0"`
}

func listPagesDeployments(ctx context.Context, p *policy, in listPagesDeploymentsInput) (string, error) {
	pagesProjectID := strings.TrimSpace(in.PagesProjectID)
	if pagesProjectID == "" {
		return "", errors.New("pages_project_id is required")
	}
	client, err := p.readClient(ctx)
	if err != nil {
		return "", err
	}
	result, err := client.ListPagesDeploy(volcengine.ListPagesDeployParams{
		PagesProjectID: pagesProjectID,
		Limit:          listLimit(in.Count),
		Offset:         in.Offset,
	})
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"deployments": result.Deployments,
		"count":       len(result.Deployments),
		"total":       result.Total,
	}
	for _, deploy := range result.Deployments {
		if strings.TrimSpace(deploy.PreviewDomain) != "" {
			payload["note"] = pagesBareDomainNote
			break
		}
	}
	return toJSON(payload)
}

type getPagesEnvVarsInput struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
	ShowValues  bool   `json:"show_values,omitempty" jsonschema:"return raw secret values instead of masked placeholders; defaults to false (masked)"`
}

type pagesEnvVarView struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func getPagesEnvVars(ctx context.Context, p *policy, in getPagesEnvVarsInput) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	branchID, err := resolvePagesBranch(ctx, client, workspaceID, in.BranchID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeSupabaseDeployEnvVars(workspaceID, branchID)
	if err != nil {
		return "", err
	}
	views := make([]pagesEnvVarView, 0, len(result.EnvVars))
	for _, kv := range result.EnvVars {
		value := kv.Value
		if !in.ShowValues && value != "" {
			value = "******"
		}
		views = append(views, pagesEnvVarView{Key: kv.Key, Value: value})
	}
	return toJSON(map[string]any{
		"workspace_id": workspaceID,
		"branch_id":    branchID,
		"env_vars":     views,
		"count":        len(views),
		"masked":       !in.ShowValues,
	})
}

func getPagesBinding(ctx context.Context, p *policy, in pagesScopedInput) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	branchID, err := resolvePagesBranch(ctx, client, workspaceID, in.BranchID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribePagesBinding(workspaceID, branchID)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"workspace_id":  workspaceID,
		"branch_id":     branchID,
		"has_binding":   result.Binding != nil,
		"binding":       result.Binding,
		"pages_project": result.PagesProject,
	}
	if result.PagesProject != nil && strings.TrimSpace(result.PagesProject.PreviewDomain) != "" {
		payload["note"] = pagesBindingOpenableNote
	}
	return toJSON(payload)
}

type uploadPagesResourceInput struct {
	FilePath string `json:"file_path" jsonschema:"local path to a built static-site archive to upload as the deployment resource"`
}

func uploadPagesResource(ctx context.Context, p *policy, in uploadPagesResourceInput) (string, error) {
	filePath := strings.TrimSpace(in.FilePath)
	if filePath == "" {
		return "", errors.New("file_path is required")
	}
	client, err := p.writeClient(ctx)
	if err != nil {
		return "", err
	}
	// Gate frontend deployment on the Pages access role (DCDNAccessAIDPRole).
	if err := client.CheckPagesAccessRole(); err != nil {
		return "", err
	}
	result, err := pages.UploadResourceWithClient(ctx, client, filePath)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success":     true,
		"file_name":   result.FileName,
		"size":        result.Size,
		"resource_id": result.ProjectDeployResourceID,
	})
}

type createPagesProjectInput struct {
	Name          string `json:"name" jsonschema:"Pages project name: 2-63 lowercase letters, digits, or hyphens (no leading/trailing hyphen)"`
	ResourceID    string `json:"resource_id" jsonschema:"ProjectDeployResourceID returned by upload_pages_resource"`
	Scope         string `json:"scope,omitempty" jsonschema:"project scope: domestic (default), overseas, or global"`
	Provider      string `json:"provider,omitempty" jsonschema:"deploy provider; only upload_v2 is supported (default)"`
	Framework     string `json:"framework,omitempty" jsonschema:"optional framework hint"`
	RootDir       string `json:"root_dir,omitempty" jsonschema:"optional project root directory"`
	OutputDir     string `json:"output_dir,omitempty" jsonschema:"optional build output directory"`
	BuildCmd      string `json:"build_cmd,omitempty" jsonschema:"optional build command"`
	InstallCmd    string `json:"install_cmd,omitempty" jsonschema:"optional dependency install command"`
	NodejsVersion string `json:"nodejs_version,omitempty" jsonschema:"optional Node.js version"`
	NoDeploy      bool   `json:"no_deploy,omitempty" jsonschema:"create the project without starting an initial deployment (resource_id is still required)"`
}

func createPagesProject(ctx context.Context, p *policy, in createPagesProjectInput) (string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "", errors.New("name is required")
	}
	if err := pages.ValidateProjectName(name); err != nil {
		return "", err
	}
	provider := strings.TrimSpace(in.Provider)
	if provider == "" {
		provider = "upload_v2"
	}
	if provider != "upload_v2" {
		return "", errors.New("only upload_v2 provider is currently supported")
	}
	scope := strings.TrimSpace(in.Scope)
	if scope == "" {
		scope = "domestic"
	}
	switch scope {
	case "domestic", "overseas", "global":
	default:
		return "", errors.New("invalid scope; expected domestic, overseas, or global")
	}
	resourceID := strings.TrimSpace(in.ResourceID)
	if resourceID == "" {
		return "", errors.New("resource_id is required (upload an archive with upload_pages_resource first)")
	}
	client, err := p.writeClient(ctx)
	if err != nil {
		return "", err
	}
	// Gate frontend deployment on the Pages access role (DCDNAccessAIDPRole).
	if err := client.CheckPagesAccessRole(); err != nil {
		return "", err
	}
	result, err := client.CreatePagesProject(volcengine.CreatePagesProjectParams{
		Name:                    name,
		Provider:                provider,
		Scope:                   scope,
		ProjectDeployResourceID: resourceID,
		ProjectParam: volcengine.PagesProjectParam{
			Framework:     strings.TrimSpace(in.Framework),
			RootDir:       strings.TrimSpace(in.RootDir),
			OutputDir:     strings.TrimSpace(in.OutputDir),
			BuildCmd:      strings.TrimSpace(in.BuildCmd),
			InstallCmd:    strings.TrimSpace(in.InstallCmd),
			NodejsVersion: strings.TrimSpace(in.NodejsVersion),
		},
		NoDeploy: in.NoDeploy,
	})
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success": true,
		"project": result,
	})
}

type deployPagesProjectInput struct {
	PagesProjectID string `json:"pages_project_id" jsonschema:"id of the Pages project to deploy"`
	ResourceID     string `json:"resource_id" jsonschema:"ProjectDeployResourceID returned by upload_pages_resource"`
}

func deployPagesProject(ctx context.Context, p *policy, in deployPagesProjectInput) (string, error) {
	pagesProjectID := strings.TrimSpace(in.PagesProjectID)
	if pagesProjectID == "" {
		return "", errors.New("pages_project_id is required")
	}
	resourceID := strings.TrimSpace(in.ResourceID)
	if resourceID == "" {
		return "", errors.New("resource_id is required (upload an archive with upload_pages_resource first)")
	}
	client, err := p.writeClient(ctx)
	if err != nil {
		return "", err
	}
	// Gate frontend deployment on the Pages access role (DCDNAccessAIDPRole).
	if err := client.CheckPagesAccessRole(); err != nil {
		return "", err
	}
	result, err := client.CreatePagesDeploy(volcengine.CreatePagesDeployParams{
		PagesProjectID:          pagesProjectID,
		ProjectDeployResourceID: resourceID,
	})
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success":   true,
		"deploy":    result,
		"next_step": pagesDeployNextStep,
	})
}

type bindPagesProjectInput struct {
	PagesProjectID  string `json:"pages_project_id" jsonschema:"id of the Pages project to bind"`
	WorkspaceID     string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID        string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
	CustomPrefix    string `json:"custom_prefix,omitempty" jsonschema:"environment variable prefix, required to bind a Pages project already used by another Supabase branch"`
	FrameworkPrefix string `json:"framework_prefix,omitempty" jsonschema:"framework client prefix for browser-exposed Supabase env vars (e.g. VITE_, NEXT_PUBLIC_)"`
}

func bindPagesProject(ctx context.Context, p *policy, in bindPagesProjectInput) (string, error) {
	pagesProjectID := strings.TrimSpace(in.PagesProjectID)
	if pagesProjectID == "" {
		return "", errors.New("pages_project_id is required")
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	// Gate frontend deployment on the Pages access role (DCDNAccessAIDPRole).
	if err := client.CheckPagesAccessRole(); err != nil {
		return "", err
	}
	branchID, err := resolvePagesBranch(ctx, client, workspaceID, in.BranchID)
	if err != nil {
		return "", err
	}
	if err := client.BindPagesProject(volcengine.BindPagesProjectParams{
		WorkspaceID:     workspaceID,
		BranchID:        branchID,
		PagesProjectID:  pagesProjectID,
		CustomPrefix:    strings.TrimSpace(in.CustomPrefix),
		FrameworkPrefix: strings.TrimSpace(in.FrameworkPrefix),
	}); err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success":          true,
		"workspace_id":     workspaceID,
		"branch_id":        branchID,
		"pages_project_id": pagesProjectID,
		"next_step": fmt.Sprintf(
			"Binding only injects env — nothing is served until you deploy. Next: deploy_pages_project (pages_project_id=%q, resource_id from upload_pages_resource), then get_pages_binding (workspace_id=%q, branch_id=%q) and open pages_project.preview_domain (one-time token, short-lived).",
			pagesProjectID, workspaceID, branchID),
	})
}

func unbindPagesProject(ctx context.Context, p *policy, in pagesScopedInput) (string, error) {
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	branchID, err := resolvePagesBranch(ctx, client, workspaceID, in.BranchID)
	if err != nil {
		return "", err
	}
	if err := client.UnbindPagesProject(volcengine.UnbindPagesProjectParams{
		WorkspaceID: workspaceID,
		BranchID:    branchID,
	}); err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success":      true,
		"workspace_id": workspaceID,
		"branch_id":    branchID,
	})
}

func syncPagesEnvVars(ctx context.Context, p *policy, in pagesScopedInput) (string, error) {
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	// Gate frontend deployment on the Pages access role (DCDNAccessAIDPRole).
	if err := client.CheckPagesAccessRole(); err != nil {
		return "", err
	}
	branchID, err := resolvePagesBranch(ctx, client, workspaceID, in.BranchID)
	if err != nil {
		return "", err
	}
	binding, err := client.DescribePagesBinding(workspaceID, branchID)
	if err != nil {
		return "", err
	}
	if binding.Binding == nil {
		return "", errors.Errorf("no Pages binding found for workspace %s branch %s; bind a Pages project with bind_pages_project first", workspaceID, branchID)
	}
	pagesProjectID := strings.TrimSpace(binding.Binding.PagesProjectID)
	if pagesProjectID == "" {
		return "", errors.Errorf("Pages binding for workspace %s branch %s has no Pages project id", workspaceID, branchID)
	}
	if err := client.SyncPagesDeployEnvVars(volcengine.SyncPagesDeployEnvVarsParams{
		WorkspaceID:    workspaceID,
		BranchID:       branchID,
		PagesProjectID: pagesProjectID,
	}); err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success":          true,
		"workspace_id":     workspaceID,
		"branch_id":        branchID,
		"pages_project_id": pagesProjectID,
	})
}

type fastCreatePagesInput struct {
	Name            string `json:"name" jsonschema:"Pages project + new workspace name: 2-63 lowercase letters, digits, or hyphens (no leading/trailing hyphen)"`
	FilePath        string `json:"file_path" jsonschema:"local frontend directory (auto-zipped) or a prebuilt deployment archive to upload to Pages"`
	MigrationsInit  string `json:"migrations_init,omitempty" jsonschema:"optional local directory of .sql migration files, executed in filename order against the new workspace before binding"`
	FunctionsInit   string `json:"functions_init,omitempty" jsonschema:"optional local backend root directory; Edge Functions are deployed from <functions_init>/functions/<slug>"`
	CustomPrefix    string `json:"custom_prefix,omitempty" jsonschema:"environment variable prefix, required to bind a Pages project already used by another Supabase branch"`
	FrameworkPrefix string `json:"framework_prefix,omitempty" jsonschema:"framework client prefix for browser-exposed Supabase env vars (e.g. VITE_, NEXT_PUBLIC_)"`
}

// fastCreatePages is the MCP adapter over the shared pages.RunFastCreate orchestration
// (the same core the CLI `pages fast create` command runs). It creates a brand-new
// workspace, so it is rejected under --workspace-ref. Progress lines go to stderr (the
// MCP log stream); the tool blocks until the deployment succeeds and returns the
// resulting ids as JSON.
func fastCreatePages(ctx context.Context, p *policy, in fastCreatePagesInput) (string, error) {
	if p.workspaceRef != "" {
		return "", errors.Errorf("fast_create_pages creates a new workspace and is not available when the server is scoped to a single workspace (%s)", p.workspaceRef)
	}
	if strings.TrimSpace(in.Name) == "" {
		return "", errors.New("name is required")
	}
	if strings.TrimSpace(in.FilePath) == "" {
		return "", errors.New("file_path is required")
	}
	cfg, err := p.config(ctx)
	if err != nil {
		return "", err
	}
	result, err := pages.RunFastCreate(ctx, cfg, pages.FastCreateParams{
		ProjectName:     strings.TrimSpace(in.Name),
		FilePath:        strings.TrimSpace(in.FilePath),
		MigrationsInit:  strings.TrimSpace(in.MigrationsInit),
		FunctionsInit:   strings.TrimSpace(in.FunctionsInit),
		CustomPrefix:    strings.TrimSpace(in.CustomPrefix),
		FrameworkPrefix: strings.TrimSpace(in.FrameworkPrefix),
	}, os.Stderr)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success": true,
		"result":  result,
	})
}

// pagesWorkspaceFilter resolves the optional workspace filter for list_pages_projects.
// Under --workspace-ref the scoped workspace wins (and a mismatched caller value is
// rejected); otherwise the caller-supplied value is passed through verbatim (may be empty,
// which lists across the whole account).
func pagesWorkspaceFilter(p *policy, input string) (string, error) {
	input = strings.TrimSpace(input)
	if p.workspaceRef != "" {
		if input != "" && input != p.workspaceRef {
			return "", errors.Errorf("workspace %q is not allowed; this server is scoped to %s", input, p.workspaceRef)
		}
		return p.workspaceRef, nil
	}
	return input, nil
}

// resolvePagesBranch returns the explicit branch id, or the workspace's default branch
// when the caller omits it (mirrors the CLI pages commands' branch resolution).
func resolvePagesBranch(ctx context.Context, client *volcengine.Client, workspaceID, branchID string) (string, error) {
	branchID = strings.TrimSpace(branchID)
	if branchID != "" {
		return branchID, nil
	}
	result, err := client.DescribeDefaultBranch(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	if result.Branch.BranchID == "" {
		return "", errors.Errorf("failed to resolve default branch for workspace %s", workspaceID)
	}
	return result.Branch.BranchID, nil
}
