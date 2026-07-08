// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-errors/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// meta holds the static metadata for a tool, driving registration and access policy (feature gating, read-only filtering).
type meta struct {
	name        string
	title       string
	description string
	// aliases are additional, backward-compatible names that route to this tool's
	// canonical name. They are deliberately NOT advertised in tools/list (kept hidden
	// to avoid duplicating the ~47-tool catalog for the model) but remain callable via
	// the alias-rewrite middleware (see aliases.go). Used when a tool's canonical name
	// is realigned with the CLI verb while old callers may still use the previous name
	// (e.g. start_workspace keeps restore_workspace working).
	aliases []string
	// feature is the group this tool belongs to; empty means unconditional (only the denylist can remove it), used for transport-layer tools.
	feature string
	// mutating marks a write tool; hidden in read-only mode.
	mutating bool
}

// handler is the business logic for a tool, returning a text result (usually JSON) or
// an error. Cross-cutting concerns such as input schema, parameter decoding, validation,
// and error mapping are handled centrally by defineTool; the handler is just a thin
// adapter over the fork-specific Run functions and volcengine.Client.
type handler[In any] func(ctx context.Context, p *policy, in In) (string, error)

// toolSpec is a registered tool: metadata plus a closure that adds it to the server
// (the closure captures the concrete In type so the registry stays a flat heterogeneous slice).
type toolSpec struct {
	meta     meta
	register func(s *mcp.Server, p *policy)
}

// defineTool wraps a typed business handler into a toolSpec.
//
// The SDK derives the input schema from In, validates and deserialises parameters
// before the handler runs; any error returned by the handler is packed into the
// result's IsError content (not a protocol-level error, so the model can self-correct).
// Out is an empty interface, so no output schema is produced and the handler's text
// is returned verbatim.
func defineTool[In any](m meta, h handler[In]) toolSpec {
	return toolSpec{
		meta: m,
		register: func(s *mcp.Server, p *policy) {
			mcp.AddTool(s, &mcp.Tool{
				Name:        m.name,
				Title:       m.title,
				Description: m.description,
			}, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
				out, err := h(ctx, p, in)
				if err != nil {
					var loginErr loginRequiredError
					if errors.As(err, &loginErr) {
						return &mcp.CallToolResult{
							IsError: true,
							Content: []mcp.Content{&mcp.TextContent{Text: loginErr.Error()}},
						}, nil, nil
					}
					return nil, nil, annotateRequestID(err)
				}
				return textResult(out), nil, nil
			})
		},
	}
}

// annotateRequestID appends safe Volcengine diagnostics to a failing tool's
// error, so the model (and the human reading the transcript) can quote them when
// reporting an API failure. No-op when the error carries no Volcengine details.
func annotateRequestID(err error) error {
	details := volcengine.VolcengineErrorDetailsFromErr(err)
	if details.Empty() {
		return err
	}
	return errors.Errorf("%w%s", err, formatVolcengineErrorDetails(details))
}

func formatVolcengineErrorDetails(details volcengine.ErrorDetails) string {
	var b strings.Builder
	if details.RequestID != "" {
		fmt.Fprintf(&b, "\nRequest ID: %s", details.RequestID)
	}
	if details.StatusCode != "" {
		fmt.Fprintf(&b, "\nStatus Code: %s", details.StatusCode)
	}
	if details.ErrorCode != "" {
		fmt.Fprintf(&b, "\nError Code: %s", details.ErrorCode)
	}
	if details.Message != "" {
		fmt.Fprintf(&b, "\nMessage: %s", details.Message)
	}
	return b.String()
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

// toJSON serialises a value as indented JSON, the standard output format for tools.
func toJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", errors.Errorf("failed to encode result: %w", err)
	}
	return string(b), nil
}
