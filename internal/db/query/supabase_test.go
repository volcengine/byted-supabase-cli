package query

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunSupabaseHTTPUsesSQLDataPlane(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/postgres/query", request.URL.Path)
		require.Empty(t, request.URL.RawQuery)
		require.Equal(t, "Bearer service-key", request.Header.Get("Authorization"))
		require.Equal(t, "service-key", request.Header.Get("apikey"))

		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		var payload supabaseHTTPQueryRequest
		require.NoError(t, json.Unmarshal(body, &payload))
		require.Equal(t, "select 1", payload.Query)

		_, _ = io.WriteString(writer, `[{"value":1,"label":"ok"}]`)
	}))
	defer server.Close()

	var output strings.Builder
	require.NoError(t, RunSupabaseHTTP(context.Background(), SupabaseHTTPParams{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-key",
		SQL:            "select 1",
		Format:         "json",
	}, &output))
	require.Contains(t, output.String(), `"value": 1`)
	require.Contains(t, output.String(), `"label": "ok"`)
}

func TestRunSupabaseHTTPRequiresServiceRoleKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Fatal("request should not be sent without service role key")
	}))
	defer server.Close()

	err := RunSupabaseHTTP(context.Background(), SupabaseHTTPParams{
		SupabaseURL: server.URL,
		SQL:         "select 1",
		Format:      "table",
	}, io.Discard)
	require.ErrorContains(t, err, "requires service_role authentication")
}

func TestRunSupabaseHTTPReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"invalid query"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	err := RunSupabaseHTTP(context.Background(), SupabaseHTTPParams{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-key",
		SQL:            "select invalid",
		Format:         "json",
	}, io.Discard)
	require.ErrorContains(t, err, "query request failed")
	require.ErrorContains(t, err, "invalid query")
}
