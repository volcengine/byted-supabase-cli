package solo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func TestFunctionsClientDeployUsesServiceRoleDataPlaneAPI(t *testing.T) {
	fsys := afero.NewMemMapFs()
	require.NoError(t, utils.WriteConfig(fsys, true))
	functionDir := filepath.Join(utils.FunctionsDir, "hello")
	require.NoError(t, fsys.MkdirAll(functionDir, 0755))
	require.NoError(t, afero.WriteFile(fsys, filepath.Join(functionDir, "index.ts"), []byte("export default 1"), 0644))

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/functions/v1/deploy", request.URL.Path)
		require.Equal(t, "hello", request.URL.Query().Get("slug"))
		require.Equal(t, "service-role-key", request.Header.Get("apikey"))
		require.Equal(t, "Bearer service-role-key", request.Header.Get("Authorization"))
		var payload DeployRequest
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &payload))
		require.Equal(t, "hello", payload.Slug)
		require.Equal(t, "index.ts", payload.Metadata.EntrypointPath)
		require.Equal(t, "native-node20/v1", payload.Metadata.Runtime)
		require.Len(t, payload.Files, 1)
		require.Equal(t, "export default 1", payload.Files[0].Content)
		_, _ = writer.Write([]byte(`{"status":"deployed"}`))
	}))
	defer server.Close()

	client, err := NewFunctionsClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	require.NoError(t, client.DeployLocal(context.Background(), "hello", "deno", nil, fsys))
}

func TestFunctionsClientDeployPythonInjectsRequiredScaffold(t *testing.T) {
	fsys := afero.NewMemMapFs()
	require.NoError(t, utils.WriteConfig(fsys, true))
	functionDir := filepath.Join(utils.FunctionsDir, "python-func")
	require.NoError(t, fsys.MkdirAll(functionDir, 0755))
	require.NoError(t, afero.WriteFile(fsys, filepath.Join(functionDir, "app.py"), []byte("from fastapi import FastAPI\napp = FastAPI()\n"), 0644))

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload DeployRequest
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &payload))
		require.Equal(t, "native-python3.12/v1", payload.Metadata.Runtime)
		require.Equal(t, "app.py", payload.Metadata.EntrypointPath)
		require.Len(t, payload.Files, 3)
		require.Equal(t, "app.py", payload.Files[0].Name)
		require.Equal(t, "run.sh", payload.Files[1].Name)
		require.Equal(t, "requirements.txt", payload.Files[2].Name)
		require.Contains(t, payload.Files[1].Content, "python3 -m uvicorn app:app")
		require.Equal(t, "fastapi\nuvicorn[standard]\n", payload.Files[2].Content)
		_, _ = writer.Write([]byte(`{"status":"deployed"}`))
	}))
	defer server.Close()

	client, err := NewFunctionsClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	require.NoError(t, client.DeployLocal(context.Background(), "python-func", "native-python3.12/v1", nil, fsys))
}

func TestEnsurePythonScaffoldPreservesUserFiles(t *testing.T) {
	files := []FunctionFile{
		{Name: "app.py", Content: "app"},
		{Name: "run.sh", Content: "custom-runner"},
		{Name: "requirements.txt", Content: "custom-dependency\n"},
	}
	result := ensurePythonScaffold(files, "native-python3.12/v1")
	require.Equal(t, files, result)
}

func TestFunctionsClientDeployPythonRequiresApp(t *testing.T) {
	fsys := afero.NewMemMapFs()
	require.NoError(t, utils.WriteConfig(fsys, true))
	functionDir := filepath.Join(utils.FunctionsDir, "python-func")
	require.NoError(t, fsys.MkdirAll(functionDir, 0755))
	require.NoError(t, afero.WriteFile(fsys, filepath.Join(functionDir, "README.md"), []byte("missing app"), 0644))

	client, err := NewFunctionsClient(Profile{
		SupabaseURL:    "https://example.supabase.co",
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	err = client.DeployLocal(context.Background(), "python-func", "native-python3.12/v1", nil, fsys)
	require.ErrorContains(t, err, "Python runtime requires app.py")
}

func TestFunctionsClientDeployRejectsMissingFiles(t *testing.T) {
	client, err := NewFunctionsClient(Profile{
		SupabaseURL:    "https://example.supabase.co",
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	err = client.DeployLocal(context.Background(), "hello", "deno", nil, afero.NewMemMapFs())
	require.ErrorContains(t, err, "failed to collect function files")
}
