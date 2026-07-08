// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package deploy

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/functions/gateway"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

const (
	RuntimeAuto         = "auto"
	RuntimeDeno         = "deno"
	RuntimeNativeNode20 = "native-node20/v1"
	RuntimePython39     = "native-python3.9/v1"
	RuntimePython310    = "native-python3.10/v1"
	RuntimePython312    = "native-python3.12/v1"
)

// NativeRuntimes maps each supported native Edge Function runtime to its default
// entrypoint file. Single source of truth for "which native runtimes exist and
// their entrypoints", shared by the deploy command and the MCP deploy tool.
var NativeRuntimes = map[string]string{
	RuntimeNativeNode20: defaultVolcengineEntrypoint(RuntimeNativeNode20),
	RuntimePython39:     defaultVolcengineEntrypoint(RuntimePython39),
	RuntimePython310:    defaultVolcengineEntrypoint(RuntimePython310),
	RuntimePython312:    defaultVolcengineEntrypoint(RuntimePython312),
}

// RunVolcengine deploys local Functions to the selected branch gateway.
func RunVolcengine(ctx context.Context, client *volcengine.Client, workspaceID, branchID string, slugs []string, runtime string, noVerifyJWT *bool, importMapPath string, fsys afero.Fs) error {
	if err := flags.LoadConfig(fsys); err != nil {
		return err
	}
	if len(slugs) > 0 {
		for _, slug := range slugs {
			if err := utils.ValidateFunctionSlug(slug); err != nil {
				return err
			}
		}
	} else {
		var err error
		slugs, err = getVolcengineFunctionSlugs(fsys)
		if err != nil {
			return err
		}
	}
	if len(slugs) == 0 {
		return errors.Errorf("No Functions specified or found in %s", utils.Bold(utils.FunctionsDir))
	}
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return err
	}
	deployed := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		if isVolcengineFunctionDisabled(slug) {
			fmt.Fprintln(os.Stderr, "Skipping disabled Function:", slug)
			continue
		}
		runtimeValue, err := normalizeVolcengineRuntime(slug, runtime, fsys)
		if err != nil {
			return err
		}
		if err := deployVolcengineFunction(ctx, access, workspaceID, slug, runtimeValue, noVerifyJWT, importMapPath, fsys); err != nil {
			return err
		}
		deployed = append(deployed, slug)
	}
	if len(deployed) == 0 {
		return errors.New("All Functions are disabled.")
	}
	fmt.Printf("Deployed Functions on project %s: %s\n", utils.Aqua(workspaceID), strings.Join(deployed, ", "))
	return nil
}

func deployVolcengineFunction(ctx context.Context, access volcengine.PgMetaAccess, workspaceID, slug, runtime string, noVerifyJWT *bool, importMapPath string, fsys afero.Fs) error {
	fmt.Fprintln(os.Stderr, "Deploying Function:", utils.Bold(slug))
	entrypointPath, importMap, verifyJWT, err := volcengineFunctionMetadata(slug, runtime, noVerifyJWT, importMapPath)
	if err != nil {
		return err
	}
	files, err := collectVolcengineFunctionFiles(slug, importMap, fsys)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.Errorf("No files found in %s", utils.Bold(filepath.Join(utils.FunctionsDir, slug)))
	}
	_, err = gateway.Deploy(ctx, access, gateway.DeployRequest{
		ProjectRef: workspaceID,
		Slug:       slug,
		Metadata: gateway.DeployMetadata{
			EntrypointPath: entrypointPath,
			Name:           slug,
			Runtime:        runtime,
			VerifyJWT:      &verifyJWT,
			ImportMapPath:  importMap,
		},
		Files: files,
	})
	return err
}

func volcengineFunctionMetadata(slug, runtime string, noVerifyJWT *bool, importMapPath string) (string, string, bool, error) {
	functionConfig, hasConfig := utils.Config.Functions[slug]
	if !hasConfig {
		functionConfig.Enabled = true
		functionConfig.VerifyJWT = true
	}
	entrypoint := functionConfig.Entrypoint
	if entrypoint == "" {
		entrypoint = defaultVolcengineEntrypoint(runtime)
	} else {
		entrypoint = trimFunctionPath(slug, filepath.ToSlash(entrypoint))
	}
	importMap := importMapPath
	if importMap == "" {
		importMap = functionConfig.ImportMap
	}
	if importMap != "" {
		if !filepath.IsAbs(importMap) {
			importMap = filepath.Join(utils.CurrentDirAbs, importMap)
		}
		importMap = trimFunctionPath(slug, filepath.ToSlash(importMap))
	}
	verifyJWT := true
	if hasConfig {
		verifyJWT = functionConfig.VerifyJWT
	}
	if noVerifyJWT != nil {
		verifyJWT = !*noVerifyJWT
	}
	return filepath.ToSlash(entrypoint), filepath.ToSlash(importMap), verifyJWT, nil
}

func isVolcengineFunctionDisabled(slug string) bool {
	functionConfig, ok := utils.Config.Functions[slug]
	return ok && !functionConfig.Enabled
}

func collectVolcengineFunctionFiles(slug, importMap string, fsys afero.Fs) ([]gateway.File, error) {
	functionDir := filepath.Join(utils.FunctionsDir, slug)
	files := make([]gateway.File, 0)
	err := afero.Walk(fsys, functionDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(functionDir, filePath)
		if err != nil {
			return err
		}
		return appendVolcengineFunctionFile(fsys, &files, filePath, filepath.ToSlash(rel))
	})
	if err != nil {
		return nil, errors.Errorf("failed to collect function files: %w", err)
	}
	if importMap != "" && !hasVolcengineFile(files, importMap) {
		importPath := filepath.FromSlash(importMap)
		if _, err := fsys.Stat(importPath); err == nil {
			if err := appendVolcengineFunctionFile(fsys, &files, importPath, importMap); err != nil {
				return nil, err
			}
		}
	}
	return files, nil
}

func getVolcengineFunctionSlugs(fsys afero.Fs) ([]string, error) {
	seen := make(map[string]struct{})
	slugs := make([]string, 0)
	entries, err := afero.ReadDir(fsys, utils.FunctionsDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Errorf("failed to read function directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || !utils.FuncSlugPattern.MatchString(entry.Name()) {
			continue
		}
		seen[entry.Name()] = struct{}{}
		slugs = append(slugs, entry.Name())
	}
	for slug := range utils.Config.Functions {
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		slugs = append(slugs, slug)
	}
	return slugs, nil
}

func appendVolcengineFunctionFile(fsys afero.Fs, files *[]gateway.File, filePath, name string) error {
	content, err := afero.ReadFile(fsys, filePath)
	if err != nil {
		return errors.Errorf("failed to read function file %s: %w", filePath, err)
	}
	name = strings.TrimPrefix(filepath.ToSlash(name), "./")
	if name == "" || strings.HasPrefix(name, "../") || path.IsAbs(name) {
		return errors.Errorf("invalid function file path %q", name)
	}
	*files = append(*files, gateway.File{Name: name, Content: string(content)})
	return nil
}

func hasVolcengineFile(files []gateway.File, name string) bool {
	for _, file := range files {
		if file.Name == name {
			return true
		}
	}
	return false
}

func trimFunctionPath(slug, filePath string) string {
	filePath = filepath.ToSlash(filepath.Clean(filePath))
	filePath = strings.TrimPrefix(filePath, "./")
	candidates := []string{
		path.Join(utils.FunctionsDir, slug),
		path.Join("functions", slug),
		path.Join("supabase", "functions", slug),
	}
	if utils.CurrentDirAbs != "" {
		currentDir := filepath.ToSlash(filepath.Clean(utils.CurrentDirAbs))
		candidates = append(candidates,
			path.Join(currentDir, utils.FunctionsDir, slug),
			path.Join(currentDir, "functions", slug),
			path.Join(currentDir, "supabase", "functions", slug),
		)
	}
	for _, candidate := range candidates {
		candidate = filepath.ToSlash(filepath.Clean(candidate))
		if strings.HasPrefix(filePath, candidate+"/") {
			return strings.TrimPrefix(filePath, candidate+"/")
		}
	}
	if idx := strings.LastIndex(filePath, "/functions/"+slug+"/"); idx >= 0 {
		return filePath[idx+len("/functions/"+slug+"/"):]
	}
	return filePath
}

func defaultVolcengineEntrypoint(runtime string) string {
	if strings.HasPrefix(runtime, "native-python") {
		return "app.py"
	}
	return "index.ts"
}

func normalizeVolcengineRuntime(slug, runtime string, fsys afero.Fs) (string, error) {
	configuredRuntime := ""
	if functionConfig, ok := utils.Config.Functions[slug]; ok {
		configuredRuntime = strings.TrimSpace(functionConfig.Runtime)
	}
	if strings.TrimSpace(runtime) == "" || strings.TrimSpace(runtime) == RuntimeAuto {
		runtime = configuredRuntime
	}
	switch strings.TrimSpace(runtime) {
	case "", RuntimeAuto:
		return detectVolcengineRuntime(slug, fsys)
	case RuntimeDeno, "node", "node20", RuntimeNativeNode20:
		return RuntimeNativeNode20, nil
	case "python3.9", "python39", RuntimePython39:
		return RuntimePython39, nil
	case "python3.10", "python310", "python", RuntimePython310:
		return RuntimePython310, nil
	case "python3.12", "python312", RuntimePython312:
		return RuntimePython312, nil
	default:
		return "", errors.Errorf("unsupported runtime %q. Supported values: auto, deno, native-node20/v1, python3.9, python3.10, python3.12", runtime)
	}
}

func detectVolcengineRuntime(slug string, fsys afero.Fs) (string, error) {
	functionDir := filepath.Join(utils.FunctionsDir, slug)
	if exists, err := afero.Exists(fsys, filepath.Join(functionDir, "run.sh")); err != nil {
		return "", errors.Errorf("failed to detect function runtime: %w", err)
	} else if exists {
		return detectVolcenginePythonRuntime(slug), nil
	}
	return RuntimeNativeNode20, nil
}

func detectVolcenginePythonRuntime(slug string) string {
	normalized := strings.ToLower(strings.NewReplacer("_", "-", ".", "-").Replace(slug))
	switch {
	case strings.Contains(normalized, "python-3-9") || strings.Contains(normalized, "python39"):
		return RuntimePython39
	case strings.Contains(normalized, "python-3-10") || strings.Contains(normalized, "python310"):
		return RuntimePython310
	case strings.Contains(normalized, "python-3-12") || strings.Contains(normalized, "python312"):
		return RuntimePython312
	default:
		return RuntimePython312
	}
}
