// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

// Package gateway is the single source of truth for Edge Function branch-gateway
// operations: the endpoint paths, request body structures, and HTTP calls for deploy /
// get / body / delete are defined once here and shared by CLI commands
// (internal/functions/*) and MCP tools. Callers are responsible only for assembling
// the request (CLI reads files from disk; MCP takes code from string parameters);
// all transport details are handled here.
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

const (
	basePath   = "/v1/projects/default/functions/"
	deployPath = "/functions/v1/deploy"
)

// File is a single function source file in a gateway request/response.
type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// DeployMetadata is the metadata for a deploy request.
type DeployMetadata struct {
	EntrypointPath string `json:"entrypoint_path"`
	Name           string `json:"name"`
	Runtime        string `json:"runtime"`
	VerifyJWT      *bool  `json:"verify_jwt,omitempty"`
	ImportMapPath  string `json:"import_map_path,omitempty"`
}

// DeployRequest is the request body for a deploy call.
type DeployRequest struct {
	ProjectRef string         `json:"projectRef,omitempty"`
	Slug       string         `json:"slug,omitempty"`
	Metadata   DeployMetadata `json:"metadata"`
	Files      []File         `json:"files"`
}

// BodyResponse is the set of source files returned by the function body endpoint.
type BodyResponse struct {
	Version int    `json:"version"`
	Files   []File `json:"files"`
}

// Deploy creates or updates an Edge Function and returns the raw gateway response body.
func Deploy(ctx context.Context, access volcengine.PgMetaAccess, req DeployRequest) ([]byte, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Errorf("failed to encode deploy request: %w", err)
	}
	deployURL, err := volcengine.BuildPgMetaURL(access.BaseURL, deployPath)
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(deployURL)
	if err != nil {
		return nil, errors.Errorf("failed to parse deploy URL: %w", err)
	}
	q := parsed.Query()
	q.Set("slug", req.Slug)
	parsed.RawQuery = q.Encode()
	return access.DoRequestRaw(ctx, http.MethodPost, parsed.String(), bytes.NewReader(payload), volcengine.WithTimeout(10*time.Minute))
}

// GetMetadata fetches the metadata of a single Edge Function as raw JSON.
func GetMetadata(ctx context.Context, access volcengine.PgMetaAccess, slug string) ([]byte, error) {
	return access.DoRequest(ctx, http.MethodGet, basePath+url.PathEscape(slug), nil, volcengine.WithTimeout(30*time.Second))
}

// GetBody fetches the source files of an Edge Function.
func GetBody(ctx context.Context, access volcengine.PgMetaAccess, slug string) (BodyResponse, error) {
	body, err := access.DoRequest(ctx, http.MethodGet, basePath+url.PathEscape(slug)+"/body", nil, volcengine.WithTimeout(30*time.Second))
	if err != nil {
		return BodyResponse{}, err
	}
	var parsed BodyResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return BodyResponse{}, errors.Errorf("failed to decode function body response: %w", err)
	}
	return parsed, nil
}

// Delete deletes an Edge Function.
func Delete(ctx context.Context, access volcengine.PgMetaAccess, slug string) error {
	_, err := access.DoRequest(ctx, http.MethodDelete, basePath+url.PathEscape(slug), nil)
	return err
}

// IsNotFound reports whether a gateway error is a 404 (function not found).
func IsNotFound(err error) bool {
	var se *volcengine.StatusError
	return errors.As(err, &se) && se.StatusCode == http.StatusNotFound
}
