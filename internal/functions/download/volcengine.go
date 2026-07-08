// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package download

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
	functionlist "github.com/volcengine/byted-supabase-cli/internal/functions/list"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// RunVolcengine downloads one or all Edge Functions from the selected branch gateway.
func RunVolcengine(ctx context.Context, client *volcengine.Client, workspaceID, branchID, slug string, fsys afero.Fs) error {
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return err
	}
	if slug != "" {
		if err := utils.ValidateFunctionSlug(slug); err != nil {
			return err
		}
		return downloadVolcengineFunction(ctx, access, workspaceID, slug, fsys)
	}
	functions, err := functionlist.GetVolcengineFunctionsWithAccess(ctx, access)
	if err != nil {
		return err
	}
	if len(functions) == 0 {
		fmt.Fprintln(os.Stderr, "No functions found in project", utils.Aqua(workspaceID))
		return nil
	}
	fmt.Fprintf(os.Stderr, "Found %d function(s) to download\n", len(functions))
	for _, function := range functions {
		if err := downloadVolcengineFunction(ctx, access, workspaceID, function.Slug, fsys); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stderr, "Successfully downloaded all functions from project", utils.Aqua(workspaceID))
	return nil
}

func downloadVolcengineFunction(ctx context.Context, access volcengine.PgMetaAccess, workspaceID, slug string, fsys afero.Fs) error {
	fmt.Fprintln(os.Stderr, "Downloading Function:", utils.Bold(slug))
	functionBody, err := gateway.GetBody(ctx, access, slug)
	if err != nil {
		return err
	}
	for _, file := range functionBody.Files {
		dstPath, err := volcengineDownloadPath(slug, file.Name)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Writing file:", dstPath)
		if err := utils.WriteFile(dstPath, []byte(file.Content), fsys); err != nil {
			return errors.Errorf("failed to save function file: %w", err)
		}
	}
	fmt.Fprintf(os.Stderr, "Downloaded Function %s from project %s.\n", utils.Aqua(slug), utils.Aqua(workspaceID))
	return nil
}

func volcengineDownloadPath(slug, name string) (string, error) {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	prefix := path.Join(utils.FunctionsDir, slug) + "/"
	name = strings.TrimPrefix(name, prefix)
	clean := path.Clean(name)
	if clean == "." || strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.Errorf("invalid function file path %q", name)
	}
	return filepath.Join(utils.FunctionsDir, slug, filepath.FromSlash(clean)), nil
}
