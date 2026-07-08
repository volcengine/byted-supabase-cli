// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"strings"

	"github.com/volcengine/byted-supabase-cli/internal/functions/gateway"
	functionsnew "github.com/volcengine/byted-supabase-cli/internal/functions/new"
)

// Python native runtimes boot via a `run.sh` launcher (uvicorn app:app) plus a
// `requirements.txt`. The CLI deploy path uploads these from disk (scaffolded by
// `functions new`), but the MCP tool only receives a single code string — so the
// gateway rejects the deploy with "no run.sh found". We reuse the exact templates
// embedded by `functions new` (single source of truth, no copy) and inject them
// here in the MCP layer.

// isPythonRuntime reports whether runtime is one of the native Python runtimes.
func isPythonRuntime(runtime string) bool {
	return strings.HasPrefix(strings.TrimSpace(runtime), "native-python")
}

// ensurePythonScaffold appends the run.sh launcher and requirements.txt that a
// native Python Edge Function needs to boot, unless the caller already supplied
// them. For non-Python runtimes it returns files unchanged. A non-empty
// requirements override replaces the default fastapi/uvicorn dependency list.
func ensurePythonScaffold(files []gateway.File, runtime, requirements string) []gateway.File {
	if !isPythonRuntime(runtime) {
		return files
	}
	if !hasGatewayFile(files, "run.sh") {
		files = append(files, gateway.File{Name: "run.sh", Content: functionsnew.PythonRunSh()})
	}
	if !hasGatewayFile(files, "requirements.txt") {
		content := strings.TrimSpace(requirements)
		if content == "" {
			content = functionsnew.PythonRequirements()
		} else {
			content += "\n"
		}
		files = append(files, gateway.File{Name: "requirements.txt", Content: content})
	}
	return files
}

func hasGatewayFile(files []gateway.File, name string) bool {
	for _, f := range files {
		if f.Name == name {
			return true
		}
	}
	return false
}
