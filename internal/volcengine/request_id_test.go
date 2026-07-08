// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"net/http"
	"testing"

	"github.com/go-errors/errors"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/volcengine-go-sdk/volcengine/volcengineerr"
)

func TestRequestIDFromErr(t *testing.T) {
	// Mirrors the SDK error built with WithSimpleError(true): Error() is just the
	// code, but RequestID() still carries the id.
	simple := true
	sdkErr := volcengineerr.NewRequestFailure(
		volcengineerr.New("InvalidParameter", "workspace not found", nil),
		http.StatusBadRequest, "req-sdk-123", &simple,
	)

	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"data plane StatusError", &StatusError{StatusCode: 500, RequestID: "req-gw-1"}, "req-gw-1"},
		{"wrapped StatusError", errors.Errorf("query failed: %w", &StatusError{RequestID: "req-gw-2"}), "req-gw-2"},
		{"control plane SDK error", sdkErr, "req-sdk-123"},
		{"wrapped SDK error", errors.Errorf("failed to call volcengine X: %w", sdkErr), "req-sdk-123"},
		{"no request id", errors.New("validation: sql is required"), ""},
		{"StatusError without id", &StatusError{StatusCode: 404}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, RequestIDFromErr(tt.err))
		})
	}
}

func TestRequestIDFromHeader(t *testing.T) {
	assert.Equal(t, "", requestIDFromHeader(http.Header{}))

	h := http.Header{}
	h.Set("X-Request-Id", "kong-1")
	assert.Equal(t, "kong-1", requestIDFromHeader(h))

	// X-Tt-Logid takes priority over the gateway headers.
	h.Set("X-Tt-Logid", "logid-1")
	assert.Equal(t, "logid-1", requestIDFromHeader(h))
}

func TestVolcengineErrorDetailsFromErr(t *testing.T) {
	simple := true
	sdkErr := volcengineerr.NewRequestFailure(
		volcengineerr.New("InvalidParameter", "workspace not found", nil),
		http.StatusBadRequest, "req-sdk-123", &simple,
	)

	tests := []struct {
		name string
		err  error
		want ErrorDetails
	}{
		{
			name: "control plane SDK request failure",
			err:  errors.Errorf("failed to call volcengine X: %w", sdkErr),
			want: ErrorDetails{
				RequestID:  "req-sdk-123",
				StatusCode: "400",
				ErrorCode:  "InvalidParameter",
				Message:    "workspace not found",
			},
		},
		{
			name: "data plane status error",
			err:  errors.Errorf("query failed: %w", &StatusError{StatusCode: http.StatusInternalServerError, RequestID: "req-gw-1"}),
			want: ErrorDetails{
				RequestID:  "req-gw-1",
				StatusCode: "500",
			},
		},
		{
			name: "SDK client error without request id",
			err:  errors.Errorf("validation failed: %w", volcengineerr.New("InvalidParameter", "bad input", nil)),
			want: ErrorDetails{
				ErrorCode: "InvalidParameter",
				Message:   "bad input",
			},
		},
		{
			name: "no volcengine details",
			err:  errors.New("plain error"),
			want: ErrorDetails{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, VolcengineErrorDetailsFromErr(tt.err))
		})
	}
}
