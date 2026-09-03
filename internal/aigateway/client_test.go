// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package aigateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatAndModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "Bearer test-token", request.Header.Get("Authorization"))
		switch request.URL.Path {
		case "/v1/models":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, `{"data":[{"id":"model-a","object":"model"}]}`)
		case "/v1/chat/completions":
			require.Equal(t, "application/json", request.Header.Get("Content-Type"))
			_, _ = io.WriteString(writer, `{"choices":[{"message":{"content":"hello"}}]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, Token: "test-token", HTTP: server.Client()}
	var output strings.Builder
	require.NoError(t, client.Chat(context.Background(), ChatRequest{
		Model:    "model-a",
		Messages: []byte(`[{"role":"user","content":"hi"}]`),
	}, &output))
	require.Contains(t, output.String(), "hello")

	models, err := client.Models(context.Background())
	require.NoError(t, err)
	require.Equal(t, "model-a", models.Data[0].ID)
}

func TestChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(writer, "data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n")
		_, _ = io.WriteString(writer, "data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n")
		_, _ = io.WriteString(writer, "data: [DONE]\n\n")
	}))
	defer server.Close()

	var output strings.Builder
	err := (Client{BaseURL: server.URL, Token: "test-token", HTTP: server.Client()}).Chat(
		context.Background(),
		ChatRequest{Messages: []byte(`[{"role":"user","content":"hi"}]`), Stream: true},
		&output,
	)
	require.NoError(t, err)
	require.Equal(t, "hello", output.String())
}

func TestDefaultHTTPClientHasTimeout(t *testing.T) {
	client := Client{}
	require.Equal(t, defaultHTTPTimeout, client.httpClient().Timeout)
}

func TestInjectedHTTPClientIsPreserved(t *testing.T) {
	httpClient := &http.Client{}
	client := Client{HTTP: httpClient}
	require.Same(t, httpClient, client.httpClient())
}
