// Copyright (c) 2021 Supabase, Inc. and contributors
// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT
//
// This file has been modified by ByteDance Ltd. and/or its affiliates.
//
// Original file was released under MIT License, with the full license text
// available at https://github.com/supabase/cli/blob/main/LICENSE.
//
// This modified file is released under the same license.

package new

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/functions/deploy"
	_init "github.com/volcengine/byted-supabase-cli/internal/init"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

var (
	//go:embed templates/index.ts
	indexEmbed string
	//go:embed templates/app.py
	pythonAppEmbed string
	//go:embed templates/run.sh
	pythonRunEmbed string
	//go:embed templates/requirements.txt
	pythonRequirementsEmbed string
	//go:embed templates/config.toml
	configEmbed string

	indexTemplate  = template.Must(template.New("index").Parse(indexEmbed))
	configTemplate = template.Must(template.New("config").Parse(configEmbed))
)

type indexConfig struct {
	URL        string
	Token      string
	Slug       string
	Runtime    string
	Entrypoint string
}

func Run(ctx context.Context, slug, runtime string, fsys afero.Fs) error {
	// 1. Sanity checks.
	if err := utils.ValidateFunctionSlug(slug); err != nil {
		return err
	}
	runtime, err := normalizeNewRuntime(runtime)
	if err != nil {
		return err
	}
	// Check if this is the first function being created
	existingSlugs, err := deploy.GetFunctionSlugs(fsys)
	if err != nil {
		fmt.Fprintln(utils.GetDebugLogger(), err)
	}
	isFirstFunction := len(existingSlugs) == 0

	// 2. Create new function.
	funcDir := filepath.Join(utils.FunctionsDir, slug)
	if err := utils.MkdirIfNotExistFS(fsys, funcDir); err != nil {
		return err
	}
	// Load config if available
	if err := flags.LoadConfig(fsys); err != nil {
		fmt.Fprintln(utils.GetDebugLogger(), err)
	}
	if err := createFunctionFiles(slug, runtime, fsys); err != nil {
		return err
	}
	if err := appendConfigFile(slug, runtime, fsys); err != nil {
		return err
	}
	fmt.Println("Created new Function at " + utils.Bold(funcDir))

	if isFirstFunction {
		if err := _init.PromptForIDESettings(ctx, fsys); err != nil {
			return err
		}
	}
	return nil
}

func createFunctionFiles(slug, runtime string, fsys afero.Fs) error {
	if isPythonRuntime(runtime) {
		return createPythonFunctionFiles(slug, fsys)
	}
	return createDenoEntrypointFile(slug, fsys)
}

func createDenoEntrypointFile(slug string, fsys afero.Fs) error {
	entrypointPath := filepath.Join(utils.FunctionsDir, slug, "index.ts")
	f, err := fsys.OpenFile(entrypointPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return errors.Errorf("failed to create entrypoint: %w", err)
	}
	defer f.Close()
	if err := indexTemplate.Option("missingkey=error").Execute(f, indexConfig{
		URL:   utils.GetApiUrl("/functions/v1/" + slug),
		Token: utils.Config.Auth.AnonKey.Value,
	}); err != nil {
		return errors.Errorf("failed to write entrypoint: %w", err)
	}
	return nil
}

func createPythonFunctionFiles(slug string, fsys afero.Fs) error {
	funcDir := filepath.Join(utils.FunctionsDir, slug)
	files := map[string]string{
		"app.py":           pythonAppEmbed,
		"run.sh":           pythonRunEmbed,
		"requirements.txt": pythonRequirementsEmbed,
	}
	for name, content := range files {
		perm := os.FileMode(0644)
		if name == "run.sh" {
			perm = 0755
		}
		filePath := filepath.Join(funcDir, name)
		file, err := fsys.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if err != nil {
			return errors.Errorf("failed to create %s: %w", name, err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			_ = file.Close()
			return errors.Errorf("failed to write %s: %w", name, err)
		}
		if err := file.Close(); err != nil {
			return errors.Errorf("failed to close %s: %w", name, err)
		}
	}
	return nil
}

func appendConfigFile(slug, runtime string, fsys afero.Fs) error {
	if _, exists := utils.Config.Functions[slug]; exists {
		fmt.Fprintf(os.Stderr, "[functions.%s] is already declared in %s\n", slug, utils.Bold(utils.ConfigPath))
		return nil
	}
	f, err := fsys.OpenFile(utils.ConfigPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return errors.Errorf("failed to append config: %w", err)
	}
	defer f.Close()
	if err := configTemplate.Option("missingkey=error").Execute(f, indexConfig{
		Slug:       slug,
		Runtime:    runtime,
		Entrypoint: newEntrypoint(runtime),
	}); err != nil {
		return errors.Errorf("failed to append template: %w", err)
	}
	return nil
}

func normalizeNewRuntime(runtime string) (string, error) {
	switch strings.TrimSpace(runtime) {
	case "", deploy.RuntimeDeno, "node", "node20", deploy.RuntimeNativeNode20:
		return deploy.RuntimeNativeNode20, nil
	case "python3.9", "python39", deploy.RuntimePython39:
		return deploy.RuntimePython39, nil
	case "python3.10", "python310", "python", deploy.RuntimePython310:
		return deploy.RuntimePython310, nil
	case "python3.12", "python312", deploy.RuntimePython312:
		return deploy.RuntimePython312, nil
	default:
		return "", errors.Errorf("unsupported runtime %q. Supported values: deno, native-node20/v1, python3.9, python3.10, python3.12", runtime)
	}
}

func isPythonRuntime(runtime string) bool {
	return strings.HasPrefix(runtime, "native-python")
}

func newEntrypoint(runtime string) string {
	if isPythonRuntime(runtime) {
		return "app.py"
	}
	return "index.ts"
}
