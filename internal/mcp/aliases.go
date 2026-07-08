// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// methodCallTool is the JSON-RPC method for invoking a tool. The SDK keeps its own
// copy of this constant unexported, so we mirror the literal here (matching the wire
// protocol) for the receiving middleware to key on.
const methodCallTool = "tools/call"

// aliasRewriteMiddleware returns a server receiving-middleware that rewrites the tool
// name on an incoming tools/call request from a backward-compatible alias to its
// canonical name. This keeps a renamed tool callable under its old name without
// listing it twice in tools/list (aliases are hidden — only canonical names are
// advertised). The middleware mutates CallToolParamsRaw.Name in place, which is the
// same value the server's dispatcher reads to look the tool up, so the call is routed
// to the canonical handler. Non-tools/call methods and unmapped names pass through
// unchanged.
func aliasRewriteMiddleware(aliases map[string]string) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == methodCallTool {
				if params, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok {
					if canonical, ok := aliases[params.Name]; ok {
						params.Name = canonical
					}
				}
			}
			return next(ctx, method, req)
		}
	}
}
