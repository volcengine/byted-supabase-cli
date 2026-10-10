// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package sandbox

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateUsesE2BCompatibleRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/sandbox/v1/sandboxes", request.URL.Path)
		require.Equal(t, "Bearer service-role", request.Header.Get("Authorization"))
		require.Equal(t, "service-role", request.Header.Get("apikey"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		require.Equal(t, "codex", body["templateID"])
		require.Equal(t, float64(900), body["timeout"])
		require.Equal(t, map[string]any{"ownerUserId": "user-1"}, body["metadata"])
		require.Equal(t, map[string]any{"MODE": "test"}, body["envVars"])
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"sandboxID":"sandbox-1"}`)
	}))
	defer server.Close()

	result, err := (Client{BaseURL: server.URL, Token: "service-role", HTTP: server.Client()}).Create(
		context.Background(),
		CreateRequest{
			TemplateID: "codex",
			Timeout:    900,
			Metadata:   map[string]string{"ownerUserId": "user-1"},
			EnvVars:    map[string]string{"MODE": "test"},
		},
	)
	require.NoError(t, err)
	require.JSONEq(t, `{"sandboxID":"sandbox-1"}`, string(result))
}

func TestListSandboxesUsesE2BFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/sandbox/v1/v2/sandboxes", request.URL.Path)
		require.Equal(t, "owner=user-1&purpose=demo", request.URL.Query().Get("metadata"))
		require.Equal(t, "running,paused", request.URL.Query().Get("state"))
		require.Equal(t, "100", request.URL.Query().Get("limit"))
		_, _ = io.WriteString(writer, `[]`)
	}))
	defer server.Close()

	result, err := (Client{
		BaseURL: server.URL,
		Token:   "service-role",
		HTTP:    server.Client(),
	}).ListSandboxes(context.Background(), SandboxListOptions{
		Metadata: map[string]string{"purpose": "demo", "owner": "user-1"},
		States:   []string{"running", "paused"},
		Limit:    100,
	})
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(result))
}

func TestStartProcessIncludesSandboxIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/sandbox/v1/process.Process/Start", request.URL.Path)
		require.Equal(t, "sandbox-1", request.Header.Get("E2b-Sandbox-Id"))
		require.Equal(t, "Basic dXNlcjo=", request.Header.Get("Authorization"))
		require.Equal(t, "application/connect+json", request.Header.Get("Content-Type"))
		var body map[string]any
		frame, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.NotEmpty(t, decodeConnectFrames(frame))
		require.NoError(t, json.Unmarshal(decodeConnectFrames(frame)[0], &body))
		process, ok := body["process"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "/bin/echo", process["cmd"])
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(encodeConnectFrame([]byte(`{"event":{"start":{"pid":1}}}`)))
		_, _ = writer.Write(encodeConnectFrame([]byte(`{"event":{"data":{"stdout":"aGVsbG8K"}}}`)))
		_, _ = writer.Write(encodeConnectFrame([]byte(`{"event":{"end":{"exitCode":0,"exited":true,"status":"exit status 0"}}}`)))
	}))
	defer server.Close()

	result, err := (Client{BaseURL: server.URL, Token: "service-role", HTTP: server.Client()}).StartProcess(
		context.Background(), "sandbox-1", "/bin/echo", []string{"ok"}, "",
	)
	require.NoError(t, err)
	var decoded struct {
		Stdout   string            `json:"stdout"`
		ExitCode int               `json:"exit_code"`
		Status   string            `json:"status"`
		Events   []json.RawMessage `json:"events"`
	}
	require.NoError(t, json.Unmarshal(result, &decoded))
	require.Equal(t, "hello\n", decoded.Stdout)
	require.Equal(t, 0, decoded.ExitCode)
	require.Equal(t, "exit status 0", decoded.Status)
	require.Len(t, decoded.Events, 3)
}

func TestStartProcessSurfacesTerminalConnectError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(encodeConnectFrame([]byte(`{"error":{"code":"invalid_argument","message":"cwd does not exist"}}`)))
	}))
	defer server.Close()

	result, err := (Client{
		BaseURL:     server.URL,
		AccessToken: "v1-access",
		HTTP:        server.Client(),
	}).StartProcess(context.Background(), "sandbox-1", "echo", []string{"hello"}, "/missing")
	require.NoError(t, err)
	var decoded struct {
		ExitCode int    `json:"exit_code"`
		Status   string `json:"status"`
		Error    string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(result, &decoded))
	require.Equal(t, 1, decoded.ExitCode)
	require.Equal(t, "invalid_argument: cwd does not exist", decoded.Status)
	require.Equal(t, "invalid_argument: cwd does not exist", decoded.Error)
}

func TestFileExistsTreatsNotFoundAsFalse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/sandbox/v1/files", request.URL.Path)
		require.Equal(t, "sandbox-1", request.Header.Get("E2b-Sandbox-Id"))
		http.NotFound(writer, request)
	}))
	defer server.Close()

	exists, err := (Client{BaseURL: server.URL, Token: "service-role", HTTP: server.Client()}).FileExists(
		context.Background(), "sandbox-1", "/missing.txt",
	)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestListFilesUsesFilesystemListDirRPC(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/sandbox/v1/filesystem.Filesystem/ListDir", request.URL.Path)
		require.Equal(t, "sandbox-1", request.Header.Get("E2b-Sandbox-Id"))
		require.Equal(t, "49983", request.Header.Get("E2b-Sandbox-Port"))
		require.Equal(t, "v1-access", request.Header.Get("X-Access-Token"))
		require.Equal(t, "Basic dXNlcjo=", request.Header.Get("Authorization"))
		require.Equal(t, "application/json", request.Header.Get("Content-Type"))
		var payload map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		require.Equal(t, "", payload["path"])
		require.Equal(t, float64(2), payload["depth"])
		_, _ = writer.Write([]byte(`{"entries":[]}`))
	}))
	defer server.Close()

	_, err := (Client{
		BaseURL:     server.URL,
		Token:       "service-role",
		AccessToken: "v1-access",
		HTTP:        server.Client(),
	}).ListFiles(context.Background(), "sandbox-1", "", 2)
	require.NoError(t, err)
}

func TestWriteFileUsesMultipartForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/sandbox/v1/files", request.URL.Path)
		require.Equal(t, "sandbox-1", request.Header.Get("E2b-Sandbox-Id"))
		require.Equal(t, "v1-access", request.Header.Get("X-Access-Token"))
		require.True(t, strings.HasPrefix(request.Header.Get("Content-Type"), "multipart/form-data; boundary="))
		reader := multipart.NewReader(request.Body, strings.TrimPrefix(request.Header.Get("Content-Type"), "multipart/form-data; boundary="))
		part, err := reader.NextPart()
		require.NoError(t, err)
		require.Equal(t, "file", part.FormName())
		require.Equal(t, "hello.txt", part.FileName())
		body, err := io.ReadAll(part)
		require.NoError(t, err)
		require.Equal(t, "hello", string(body))
		_, _ = io.WriteString(writer, `[{"path":"/home/user/hello.txt"}]`)
	}))
	defer server.Close()

	_, err := (Client{
		BaseURL:     server.URL,
		Token:       "service-role",
		AccessToken: "v1-access",
		HTTP:        server.Client(),
	}).WriteFile(context.Background(), "sandbox-1", "hello.txt", []byte("hello"))
	require.NoError(t, err)
}

func mustReadBody(t *testing.T, request *http.Request) []byte {
	t.Helper()
	body, err := io.ReadAll(request.Body)
	require.NoError(t, err)
	return body
}
