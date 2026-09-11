package solo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndpointsClientListsDerivedSupabaseEndpoints(t *testing.T) {
	client, err := NewEndpointsClient(Profile{
		SupabaseURL:    "https://example.supabase.co/",
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)

	endpoints := client.List()
	require.Len(t, endpoints, 4)
	require.Equal(t, "https://example.supabase.co/auth/v1", endpoints[0].URL)
	require.Equal(t, "https://example.supabase.co/functions/v1", endpoints[3].URL)
}

func TestNewEndpointsClientUsesRequestTimeout(t *testing.T) {
	client, err := NewEndpointsClient(Profile{
		SupabaseURL:    "https://example.supabase.co",
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	require.Equal(t, endpointsRequestTimeout, client.httpClient.Timeout)
}

func TestEndpointsClientCheckUsesDataPlaneHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "service-role-key", request.Header.Get("apikey"))
		require.Equal(t, "Bearer service-role-key", request.Header.Get("Authorization"))
		require.Equal(t, "byted-supabase-cli", request.Header.Get("X-Request-From"))
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := NewEndpointsClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)

	results := client.Check(context.Background())
	require.Len(t, results, 4)
	for _, result := range results {
		require.Equal(t, http.StatusUnauthorized, result.Status)
		require.True(t, result.OK)
	}
}
