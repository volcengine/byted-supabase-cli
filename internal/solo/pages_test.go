package solo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPagesClientUsesPagesDataPlanePathsAndHeaders(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		require.Equal(t, "service-role-key", request.Header.Get("apikey"))
		require.Equal(t, "Bearer service-role-key", request.Header.Get("Authorization"))
		require.Equal(t, "byted-supabase-cli", request.Header.Get("X-Request-From"))
		switch request.URL.Path {
		case "/pages/v1/projects":
			require.Empty(t, request.URL.Query().Get("all"))
			require.Empty(t, request.URL.RawQuery)
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(body, &payload))
			require.Equal(t, "demo", payload["Name"])
			require.Equal(t, float64(10), payload["PageSize"])
			require.Equal(t, float64(1), payload["PageNumber"])
			_, _ = writer.Write([]byte(`{"Projects":[{"ProjectID":"pages-1"}]}`))
		case "/pages/v1/binding":
			_, _ = writer.Write([]byte(`{"Binding":{"PagesProjectId":"pages-1"}}`))
		case "/pages/v1/projects/pages-1/deploys":
			_, _ = writer.Write([]byte(`{"Total":0,"Deploys":[]}`))
		default:
			http.Error(writer, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewPagesClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	_, err = client.List(context.Background(), "demo", 10, 0)
	require.NoError(t, err)
	_, err = client.Binding(context.Background())
	require.NoError(t, err)
	_, err = client.DeployList(context.Background(), "pages-1", 20, 20)
	require.NoError(t, err)
	require.Equal(t, []string{
		"GET /pages/v1/projects",
		"GET /pages/v1/binding",
		"GET /pages/v1/projects/pages-1/deploys",
	}, requests)
}

func TestPagesClientBindCreateDeployAndSyncBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		var value map[string]any
		if len(body) > 0 {
			require.NoError(t, json.Unmarshal(body, &value))
		}
		switch request.URL.Path {
		case "/pages/v1/binding":
			require.Equal(t, "PUT", request.Method)
			require.Equal(t, "pages-1", value["PagesProjectId"])
		case "/pages/v1/projects":
			require.Equal(t, "POST", request.Method)
			require.Equal(t, "demo", value["Name"])
			_, _ = writer.Write([]byte(`{"PagesProjectId":"pages-1"}`))
		case "/pages/v1/projects/pages-1/deploys":
			require.Equal(t, "POST", request.Method)
			require.Equal(t, "resource-1", value["ProjectDeployResourceId"])
			_, _ = writer.Write([]byte(`{"DeployId":"deploy-1"}`))
		case "/pages/v1/binding/env-vars/sync":
			require.Equal(t, "POST", request.Method)
			require.Equal(t, "pages-1", value["PagesProjectId"])
		default:
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewPagesClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	require.NoError(t, client.Bind(context.Background(), "pages-1", "", ""))
	_, err = client.Create(context.Background(), map[string]any{"Name": "demo"})
	require.NoError(t, err)
	_, err = client.Deploy(context.Background(), "pages-1", "resource-1")
	require.NoError(t, err)
	require.NoError(t, client.Sync(context.Background(), "pages-1"))
}
