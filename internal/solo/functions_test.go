package solo

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

func TestFunctionsClientListUsesDataPlaneEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodGet, request.Method)
		require.Equal(t, "/functions/v1/", request.URL.Path)
		require.Equal(t, "service-role-key", request.Header.Get("apikey"))
		require.Equal(t, "Bearer service-role-key", request.Header.Get("Authorization"))
		_, _ = writer.Write([]byte(`[{"id":"function-1","name":"Hello","slug":"hello","status":"ACTIVE","version":2,"created_at":"2026-09-08T00:00:00Z","updated_at":"2026-09-08T01:00:00Z"}]`))
	}))
	defer server.Close()

	client, err := NewFunctionsClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)

	functions, err := client.List(context.Background())
	require.NoError(t, err)
	require.Len(t, functions, 1)
	require.Equal(t, "hello", functions[0].Slug)
	require.Equal(t, api.FunctionResponseStatusACTIVE, functions[0].Status)
	require.Equal(t, int64(1788829200000), functions[0].UpdatedAt)
}

func TestNewFunctionsClientUsesRequestTimeout(t *testing.T) {
	client, err := NewFunctionsClient(Profile{
		SupabaseURL:    "https://example.com",
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	require.Equal(t, functionsRequestTimeout, client.httpClient.Timeout)
}

func TestFunctionsClientDownloadWritesFilesSafely(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/functions/v1/hello/body", request.URL.Path)
		_, _ = writer.Write([]byte(`{"version":1,"files":[{"name":"hello/index.ts","content":"export default 1"},{"name":"utils.ts","content":"export const x = 1"}]}`))
	}))
	defer server.Close()

	client, err := NewFunctionsClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	fsys := afero.NewMemMapFs()

	require.NoError(t, client.Download(context.Background(), "hello", "supabase/functions", fsys))
	data, err := afero.ReadFile(fsys, filepath.Join("supabase/functions", "hello", "index.ts"))
	require.NoError(t, err)
	require.Equal(t, "export default 1", string(data))
	data, err = afero.ReadFile(fsys, filepath.Join("supabase/functions", "hello", "utils.ts"))
	require.NoError(t, err)
	require.Equal(t, "export const x = 1", string(data))
}

func TestFunctionsClientRejectsUnsafeFilePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"version":1,"files":[{"name":"../secret.txt","content":"secret"}]}`))
	}))
	defer server.Close()

	client, err := NewFunctionsClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	err = client.Download(context.Background(), "hello", "supabase/functions", afero.NewMemMapFs())
	require.ErrorContains(t, err, "invalid function file path")
}

func TestFunctionsClientDeleteUsesDataPlaneEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodDelete, request.Method)
		require.Equal(t, "/functions/v1/hello", request.URL.Path)
		_, _ = io.WriteString(writer, `{}`)
	}))
	defer server.Close()

	client, err := NewFunctionsClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	require.NoError(t, client.Delete(context.Background(), "hello"))
}

func TestFunctionsClientPassesThroughHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"denied"}`, http.StatusForbidden)
	}))
	defer server.Close()

	client, err := NewFunctionsClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	_, err = client.List(context.Background())
	require.ErrorContains(t, err, "Functions API returned HTTP 403")
	require.ErrorContains(t, err, "denied")
}
