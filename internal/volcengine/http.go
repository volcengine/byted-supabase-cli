// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-errors/errors"
)

const (
	DefaultTimeout   = 15 * time.Second
	DBRequestTimeout = time.Minute
	// HeaderFrom is the request-source header the backend reads to attribute traffic.
	HeaderFrom = "X-Request-From"
	// RequestSourceCLI marks interactive CLI traffic (the default).
	RequestSourceCLI = "byted-supabase-cli"
	// RequestSourceMCP marks traffic from the in-CLI MCP server (`mcp serve`).
	RequestSourceMCP = "byted-supabase-mcp"
)

// requestSource is the process-wide X-Request-From value. It defaults to the CLI
// source and is switched once, at MCP server startup (SetRequestSource), so the
// backend can tell MCP traffic apart from interactive CLI traffic. A single
// process only ever plays one role (a CLI command or the MCP server), so a
// package-level value needs no synchronisation.
var requestSource = RequestSourceCLI

// SetRequestSource overrides the X-Request-From value sent on every backend
// request. A blank value is ignored so the header can never be emptied.
func SetRequestSource(source string) {
	if s := strings.TrimSpace(source); s != "" {
		requestSource = s
	}
}

// RequestSource returns the current X-Request-From value.
func RequestSource() string {
	return requestSource
}

// StatusError is returned when the HTTP response has a non-2xx status code.
// Callers can use errors.As to extract the status code for special handling.
// RequestID carries the branch-gateway request id (if the response exposed one)
// for tracing; it is left out of Error() so CLI output is unchanged, and is
// read via RequestIDFromErr (used by the MCP layer).
type StatusError struct {
	StatusCode int
	Body       string
	RequestID  string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected status %d: %s", e.StatusCode, e.Body)
}

// RequestOption configures a single HTTP request issued by PgMetaAccess.
type RequestOption func(*requestConfig)

type requestConfig struct {
	timeout     time.Duration
	contentType string
}

// WithTimeout overrides the default HTTP client timeout for one request.
func WithTimeout(d time.Duration) RequestOption {
	return func(c *requestConfig) {
		c.timeout = d
	}
}

// WithContentType sets a custom Content-Type header.
func WithContentType(ct string) RequestOption {
	return func(c *requestConfig) {
		c.contentType = ct
	}
}

// DoRequest sends an HTTP request through the branch gateway with standard auth
// headers and the CLI tracking header. It returns the response body bytes on
// success (2xx) or a *StatusError for non-2xx responses.
func (a PgMetaAccess) DoRequest(ctx context.Context, method, path string, body io.Reader, opts ...RequestOption) ([]byte, error) {
	cfg := requestConfig{timeout: DefaultTimeout}
	for _, o := range opts {
		o(&cfg)
	}
	reqURL, err := BuildPgMetaURL(a.BaseURL, path)
	if err != nil {
		return nil, err
	}
	return a.doHTTP(ctx, method, reqURL, body, &cfg)
}

// DoRequestRaw is like DoRequest but accepts a fully-formed URL instead of a
// relative path. Use this when the URL has already been constructed (e.g. with
// query parameters appended).
func (a PgMetaAccess) DoRequestRaw(ctx context.Context, method, rawURL string, body io.Reader, opts ...RequestOption) ([]byte, error) {
	cfg := requestConfig{timeout: DefaultTimeout}
	for _, o := range opts {
		o(&cfg)
	}
	return a.doHTTP(ctx, method, rawURL, body, &cfg)
}

func (a PgMetaAccess) doHTTP(ctx context.Context, method, url string, body io.Reader, cfg *requestConfig) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, errors.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("apikey", a.ServiceRoleKey)
	req.Header.Set("Authorization", "Bearer "+a.ServiceRoleKey)
	req.Header.Set(HeaderFrom, requestSource)
	if body != nil && cfg.contentType == "" {
		cfg.contentType = "application/json"
	}
	if cfg.contentType != "" {
		req.Header.Set("Content-Type", cfg.contentType)
	}
	resp, err := (&http.Client{Timeout: cfg.timeout}).Do(req)
	if err != nil {
		return nil, errors.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &StatusError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
			RequestID:  requestIDFromHeader(resp.Header),
		}
	}
	return respBody, nil
}
