// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

const serverName = "byted-supabase-cli"

// Serve builds the MCP server and runs it over stdio until the context is cancelled
// (Ctrl-C) or the client disconnects.
//
// stdout carries the JSON-RPC stream only; all logs go to stderr.
//
// A client disconnect closes stdin, causing the transport to return io.EOF; both that
// and a context cancellation are treated as clean shutdowns — the function returns nil
// so the process exits with status 0 rather than surfacing a spurious error.
func Serve(ctx context.Context, opts Options) error {
	if err := validateOptions(opts); err != nil {
		return err
	}
	// Tag every backend request from this process as MCP traffic so the backend's
	// X-Request-From attribution distinguishes it from interactive CLI traffic.
	// All MCP tools reuse the shared volcengine client layer, which reads this value.
	volcengine.SetRequestSource(volcengine.RequestSourceMCP)
	err := newServer(opts, os.Stderr).Run(ctx, &mcp.StdioTransport{})
	if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// newServer assembles an MCP server from the given options. logw receives all structured logs (never written to stdout to keep the protocol stream clean).
func newServer(opts Options, logw io.Writer) *mcp.Server {
	level := slog.LevelInfo
	if opts.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(logw, &slog.HandlerOptions{Level: level}))

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Title:   "Byted Supabase CLI",
		Version: version(),
	}, &mcp.ServerOptions{
		Logger: logger,
		Instructions: "Tools for managing Volcengine Supabase (aidap) workspaces, branches, " +
			"database, edge functions and storage.",
	})

	p := newPolicy(opts)
	registerTools(srv, &p)
	return srv
}

// version returns the build-time CLI version; falls back to "dev" when running from source (ldflags only inject utils.Version in release builds).
func version() string {
	if utils.Version == "" {
		return "dev"
	}
	return utils.Version
}
