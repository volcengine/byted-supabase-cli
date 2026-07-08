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

package dump

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/go-errors/errors"
	"github.com/jackc/pgconn"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/pkg/migration"
)

func Run(ctx context.Context, path string, config pgconn.Config, dataOnly, roleOnly, dryRun bool, fsys afero.Fs, opts ...migration.DumpOptionFunc) error {
	// Initialize output stream
	outStream := (io.Writer)(os.Stdout)
	exec := DockerExec
	if dryRun {
		fmt.Fprintln(os.Stderr, "DRY RUN: *only* printing the pg_dump script to console.")
		exec = noExec
	} else if len(path) > 0 {
		f, err := fsys.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return errors.Errorf("failed to open dump file: %w", err)
		}
		defer f.Close()
		outStream = f
	}
	db := "remote"
	if utils.IsLocalDatabase(config) {
		db = "local"
	}
	if dataOnly {
		fmt.Fprintf(os.Stderr, "Dumping data from %s database...\n", db)
		return migration.DumpData(ctx, config, outStream, exec, opts...)
	} else if roleOnly {
		fmt.Fprintf(os.Stderr, "Dumping roles from %s database...\n", db)
		return migration.DumpRole(ctx, config, outStream, exec, opts...)
	}
	fmt.Fprintf(os.Stderr, "Dumping schemas from %s database...\n", db)
	return migration.DumpSchema(ctx, config, outStream, exec, opts...)
}

func noExec(ctx context.Context, script string, env []string, w io.Writer) error {
	envMap := make(map[string]string, len(env))
	for _, e := range env {
		index := strings.IndexByte(e, '=')
		if index < 0 {
			continue
		}
		envMap[e[:index]] = e[index+1:]
	}
	if _, ok := envMap["PGPASSWORD"]; ok {
		envMap["PGPASSWORD"] = "<redacted>"
	}
	expanded := os.Expand(script, func(key string) string {
		// Bash variable expansion is unsupported:
		// https://github.com/golang/go/issues/47187
		parts := strings.Split(key, ":")
		value := envMap[parts[0]]
		// Escape double quotes in env vars
		return strings.ReplaceAll(value, `"`, `\"`)
	})
	fmt.Fprintln(w, expanded)
	return nil
}

func DockerExec(ctx context.Context, script string, env []string, w io.Writer) error {
	return utils.DockerRunOnceWithConfig(
		ctx,
		container.Config{
			Image: utils.Config.Db.Image,
			Env:   env,
			Cmd:   []string{"bash", "-c", script, "--"},
		},
		container.HostConfig{
			NetworkMode: network.NetworkHost,
		},
		network.NetworkingConfig{},
		"",
		w,
		os.Stderr,
	)
}
