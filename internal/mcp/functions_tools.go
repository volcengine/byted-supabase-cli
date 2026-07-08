// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/functions/deploy"
	"github.com/volcengine/byted-supabase-cli/internal/functions/gateway"
	functionlist "github.com/volcengine/byted-supabase-cli/internal/functions/list"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// functionsTools registers the functions (Edge Functions) group.
//
// Edge Functions do not go through the aidap management API — they sit behind the
// branch gateway. All gateway calls reuse internal/functions/gateway (the same package
// used by CLI commands); here we only assemble string parameters into requests. Supported
// runtimes match the legacy Python server.
func functionsTools() []toolSpec {
	return []toolSpec{
		defineTool(meta{
			name:        "list_edge_functions",
			title:       "List Edge Functions",
			feature:     featureFunctions,
			description: "List Edge Functions deployed to a Supabase branch.",
		}, listEdgeFunctions),
		defineTool(meta{
			name:        "get_edge_function",
			title:       "Get Edge Function",
			feature:     featureFunctions,
			description: "Get a single Edge Function's metadata and source code from a Supabase branch.",
		}, getEdgeFunction),
		defineTool(meta{
			name:        "deploy_edge_function",
			title:       "Deploy Edge Function",
			feature:     featureFunctions,
			mutating:    true,
			description: "Deploy (create or update) an Edge Function from source code provided as a string. Supported runtimes: native-node20/v1 (the TypeScript/Deno runtime, entrypoint index.ts; default), native-python3.9/v1, native-python3.10/v1, native-python3.12/v1. For Python runtimes the code is deployed as app.py and a run.sh launcher plus requirements.txt are injected automatically (override deps via the requirements field; defaults to fastapi + uvicorn[standard]).",
		}, deployEdgeFunction),
		defineTool(meta{
			name:        "delete_edge_function",
			title:       "Delete Edge Function",
			feature:     featureFunctions,
			mutating:    true,
			description: "Delete an Edge Function from a Supabase branch.",
		}, deleteEdgeFunction),
	}
}

// Supported runtime → entrypoint mapping reuses the CLI deploy package as single source of truth.
const defaultFunctionRuntime = deploy.RuntimeNativeNode20

type listEdgeFunctionsInput struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func listEdgeFunctions(ctx context.Context, p *policy, in listEdgeFunctionsInput) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	functions, err := functionlist.GetVolcengineFunctions(ctx, client, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"functions": functions,
		"count":     len(functions),
	})
}

type getEdgeFunctionInput struct {
	Slug        string `json:"slug" jsonschema:"slug (name) of the Edge Function to fetch"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func getEdgeFunction(ctx context.Context, p *policy, in getEdgeFunctionInput) (string, error) {
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		return "", errors.New("slug is required")
	}
	if err := utils.ValidateFunctionSlug(slug); err != nil {
		return "", err
	}
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	metaBody, err := gateway.GetMetadata(ctx, access, slug)
	if err != nil {
		if gateway.IsNotFound(err) {
			return "", errors.Errorf("Edge function %q not found", slug)
		}
		return "", err
	}
	result := map[string]any{}
	if err := json.Unmarshal(metaBody, &result); err != nil {
		return "", errors.Errorf("failed to decode function response: %w", err)
	}
	// Populate source_code from the entrypoint file content, matching legacy server behaviour; failure is non-fatal.
	if sourceCode := fetchFunctionSourceCode(ctx, access, slug, result); sourceCode != "" {
		result["source_code"] = sourceCode
	}
	return toJSON(result)
}

// fetchFunctionSourceCode fetches the function body and returns the entrypoint file's content (or the first non-empty file).
func fetchFunctionSourceCode(ctx context.Context, access volcengine.PgMetaAccess, slug string, metadata map[string]any) string {
	body, err := gateway.GetBody(ctx, access, slug)
	if err != nil {
		return ""
	}
	entrypoint, _ := metadata["entrypoint_path"].(string)
	var first string
	for _, file := range body.Files {
		if entrypoint != "" && file.Name == entrypoint {
			return file.Content
		}
		if first == "" && file.Content != "" {
			first = file.Content
		}
	}
	return first
}

type deployEdgeFunctionInput struct {
	Slug         string `json:"slug" jsonschema:"slug (name) of the Edge Function to deploy"`
	Code         string `json:"code" jsonschema:"the function source code to deploy"`
	Runtime      string `json:"runtime,omitempty" jsonschema:"runtime; one of native-node20/v1 (TypeScript/Deno, default), native-python3.9/v1, native-python3.10/v1, native-python3.12/v1"`
	VerifyJWT    *bool  `json:"verify_jwt,omitempty" jsonschema:"require a valid JWT to invoke the function (default true)"`
	ImportMap    string `json:"import_map,omitempty" jsonschema:"optional import map JSON; applies to the native-node20/v1 (TypeScript/Deno) runtime only"`
	Requirements string `json:"requirements,omitempty" jsonschema:"python runtimes only: requirements.txt content; defaults to fastapi + uvicorn[standard] when omitted"`
	WorkspaceID  string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID     string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func deployEdgeFunction(ctx context.Context, p *policy, in deployEdgeFunctionInput) (string, error) {
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		return "", errors.New("slug is required")
	}
	if err := utils.ValidateFunctionSlug(slug); err != nil {
		return "", err
	}
	if strings.TrimSpace(in.Code) == "" {
		return "", errors.New("code is required")
	}
	runtime := strings.TrimSpace(in.Runtime)
	if runtime == "" {
		runtime = defaultFunctionRuntime
	}
	entrypoint, ok := deploy.NativeRuntimes[runtime]
	if !ok {
		return "", errors.Errorf("unsupported runtime %q; supported: native-node20/v1, native-python3.9/v1, native-python3.10/v1, native-python3.12/v1", runtime)
	}
	verifyJWT := true
	if in.VerifyJWT != nil {
		verifyJWT = *in.VerifyJWT
	}
	files := []gateway.File{{Name: entrypoint, Content: in.Code}}
	importMapPath := ""
	if importMap := strings.TrimSpace(in.ImportMap); importMap != "" {
		// Validate that the import map is valid JSON before sending.
		if !json.Valid([]byte(importMap)) {
			return "", errors.New("import_map must be valid JSON")
		}
		importMapPath = "import_map.json"
		files = append(files, gateway.File{Name: importMapPath, Content: importMap})
	}
	// MCP only receives a single code string, but the Python native runtime requires a
	// run.sh launcher script (otherwise the gateway returns "no run.sh found") and a
	// requirements.txt. Inject the scaffold here at the MCP layer to match the behaviour
	// of a CLI disk-based deployment.
	files = ensurePythonScaffold(files, runtime, in.Requirements)
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	respBody, err := gateway.Deploy(ctx, access, gateway.DeployRequest{
		ProjectRef: workspaceID,
		Slug:       slug,
		Metadata: gateway.DeployMetadata{
			EntrypointPath: entrypoint,
			Name:           slug,
			Runtime:        runtime,
			VerifyJWT:      &verifyJWT,
			ImportMapPath:  importMapPath,
		},
		Files: files,
	})
	if err != nil {
		return "", err
	}
	result := map[string]any{}
	if len(bytes.TrimSpace(respBody)) > 0 {
		_ = json.Unmarshal(respBody, &result)
	}
	if _, ok := result["runtime"]; !ok {
		result["runtime"] = runtime
	}
	return toJSON(map[string]any{
		"success":      true,
		"slug":         slug,
		"runtime":      runtime,
		"workspace_id": workspaceID,
		"function":     result,
	})
}

type deleteEdgeFunctionInput struct {
	Slug        string `json:"slug" jsonschema:"slug (name) of the Edge Function to delete"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func deleteEdgeFunction(ctx context.Context, p *policy, in deleteEdgeFunctionInput) (string, error) {
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		return "", errors.New("slug is required")
	}
	if err := utils.ValidateFunctionSlug(slug); err != nil {
		return "", err
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	if err := gateway.Delete(ctx, access, slug); err != nil {
		if gateway.IsNotFound(err) {
			return "", errors.Errorf("Edge function %q not found", slug)
		}
		return "", err
	}
	return toJSON(map[string]any{
		"success":      true,
		"slug":         slug,
		"workspace_id": workspaceID,
		"message":      "Edge function deleted successfully",
	})
}
