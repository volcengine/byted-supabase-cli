// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"net/http"
	"strconv"

	"github.com/go-errors/errors"
	"github.com/volcengine/volcengine-go-sdk/volcengine/volcengineerr"
)

// requestIDHeaders are the response headers, in priority order, that may carry
// the data-plane (branch gateway / pg-meta) request id. X-Tt-Logid is the
// ByteDance infra trace id (the same one the console-login flow reads); the
// others are set by Kong / PostgREST sitting in front of pg-meta.
var requestIDHeaders = []string{"X-Tt-Logid", "X-Request-Id", "X-Kong-Request-Id"}

// requestIDFromHeader returns the first non-empty request-id header, or "".
func requestIDFromHeader(h http.Header) string {
	for _, name := range requestIDHeaders {
		if v := h.Get(name); v != "" {
			return v
		}
	}
	return ""
}

// RequestIDFromErr extracts the Volcengine request id from an error returned by
// any Client method, or "" when none is available. It covers both call paths:
//   - control plane: SDK errors implement volcengineerr.RequestFailure, whose
//     RequestID() holds the OpenAPI request id (WithSimpleError(true) strips it
//     from the error string, so the type assertion is the only way to recover it);
//   - data plane: the branch gateway returns *StatusError, whose RequestID is
//     captured from the response headers.
//
// It unwraps the error chain, so it works through the service layer's
// errors.Errorf("...: %w", err) wrapping. Used by the MCP layer to surface a
// request id for tracing when a tool call fails.
func RequestIDFromErr(err error) string {
	return VolcengineErrorDetailsFromErr(err).RequestID
}

// ErrorDetails is safe-to-print diagnostic metadata for Volcengine API errors.
// It intentionally excludes request payloads, endpoints, credentials, tokens, and
// response bodies because those may contain sensitive information.
type ErrorDetails struct {
	RequestID  string
	StatusCode string
	ErrorCode  string
	Message    string
}

// Empty reports whether no diagnostic metadata was found in the error chain.
func (d ErrorDetails) Empty() bool {
	return d.RequestID == "" && d.StatusCode == "" && d.ErrorCode == "" && d.Message == ""
}

// VolcengineErrorDetailsFromErr extracts printable diagnostic metadata from
// Volcengine control-plane SDK errors and data-plane HTTP errors.
func VolcengineErrorDetailsFromErr(err error) ErrorDetails {
	if err == nil {
		return ErrorDetails{}
	}
	var details ErrorDetails
	var se *StatusError
	if errors.As(err, &se) {
		details.RequestID = se.RequestID
		if se.StatusCode != 0 {
			details.StatusCode = strconv.Itoa(se.StatusCode)
		}
	}
	var sdkErr volcengineerr.Error
	if errors.As(err, &sdkErr) {
		details.ErrorCode = sdkErr.Code()
		details.Message = sdkErr.Message()
	}
	var rf volcengineerr.RequestFailure
	if errors.As(err, &rf) {
		details.RequestID = rf.RequestID()
		if rf.StatusCode() != 0 {
			details.StatusCode = strconv.Itoa(rf.StatusCode())
		}
		details.ErrorCode = rf.Code()
		details.Message = rf.Message()
	}
	return details
}
