package solo

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

func TestAuthClientGetConfigUsesDataPlaneCredentials(t *testing.T) {
	var gotPath string
	var gotAPIKey string
	var gotAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotPath = request.URL.Path
		gotAPIKey = request.Header.Get("apikey")
		gotAuthorization = request.Header.Get("Authorization")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"SITE_URL":"https://example.com","DISABLE_SIGNUP":false}`))
	}))
	defer server.Close()

	client, err := NewAuthClient(Profile{
		SupabaseURL:    server.URL + "/",
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)

	config, err := client.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "/auth/v1/config", gotPath)
	require.Equal(t, "service-role-key", gotAPIKey)
	require.Equal(t, "Bearer service-role-key", gotAuthorization)
	require.Equal(t, false, config["DISABLE_SIGNUP"])
}

func TestNewAuthClientUsesRequestTimeout(t *testing.T) {
	client, err := NewAuthClient(Profile{
		SupabaseURL:    "https://example.com",
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	require.Equal(t, authRequestTimeout, client.httpClient.Timeout)
}

func TestAuthClientSetConfigSendsPatchAndReturnsPatchedValues(t *testing.T) {
	var gotMethod string
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotMethod = request.Method
		body, _ := io.ReadAll(request.Body)
		gotBody = string(body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"DISABLE_SIGNUP":true,"SITE_URL":"https://updated.example.com"}`))
	}))
	defer server.Close()

	client, err := NewAuthClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)

	config, err := client.SetConfig(context.Background(), map[string]interface{}{"DISABLE_SIGNUP": false})
	require.NoError(t, err)
	require.Equal(t, http.MethodPatch, gotMethod)
	require.JSONEq(t, `{"DISABLE_SIGNUP":false}`, gotBody)
	require.Equal(t, true, config["DISABLE_SIGNUP"])
}

func TestAuthClientPassesThroughHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"denied"}`, http.StatusForbidden)
	}))
	defer server.Close()

	client, err := NewAuthClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)

	_, err = client.GetConfig(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 403")
	require.Contains(t, err.Error(), "denied")
	require.NotContains(t, strings.ToLower(err.Error()), "workspace")
}

func TestAuthClientHooksGetAndSetUsesDataPlaneEndpoints(t *testing.T) {
	var requests []string
	hooks := map[string]interface{}{
		"hook_send_email_enabled": false,
		"hook_send_email_uri":     "",
		"hook_send_email_secrets": "",
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		switch request.Method {
		case http.MethodGet:
			_ = json.NewEncoder(writer).Encode(hooks)
		case http.MethodPatch:
			body, readErr := io.ReadAll(request.Body)
			require.NoError(t, readErr)
			require.JSONEq(t, `{"hook_send_email_uri":"pg-functions://send_email"}`, string(body))
			hooks["hook_send_email_uri"] = "pg-functions://send_email"
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ignored": "response"})
		default:
			http.Error(writer, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client, err := NewAuthClient(Profile{SupabaseURL: server.URL, ServiceRoleKey: "service-role-key"})
	require.NoError(t, err)
	result, err := client.SetHooks(context.Background(), map[string]interface{}{
		"hook_send_email_uri": "pg-functions://send_email",
	})
	require.NoError(t, err)
	require.Equal(t, "pg-functions://send_email", result["hook_send_email_uri"])
	require.Equal(t, []string{
		"GET /auth/v1/config/hooks",
		"PATCH /auth/v1/config/hooks",
		"GET /auth/v1/config/hooks",
	}, requests)
}

func TestAuthClientThirdPartyEndpoints(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/auth/v1/config/third-party-auth":
			_, _ = writer.Write([]byte(`[{"id":"provider-1","type":"oidc","oidc_issuer_url":"https://issuer.example.com"}]`))
		case request.Method == http.MethodPost:
			body, readErr := io.ReadAll(request.Body)
			require.NoError(t, readErr)
			require.JSONEq(t, `{"oidc_issuer_url":"https://issuer.example.com"}`, string(body))
			_, _ = writer.Write([]byte(`{"id":"provider-1","type":"oidc"}`))
		case request.Method == http.MethodDelete:
			_, _ = writer.Write([]byte(`{}`))
		case request.Method == http.MethodGet && request.URL.Path == "/auth/v1/config/third-party-auth/sync":
			_, _ = writer.Write([]byte(`{}`))
		default:
			http.Error(writer, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewAuthClient(Profile{SupabaseURL: server.URL, ServiceRoleKey: "service-role-key"})
	require.NoError(t, err)
	providers, err := client.ListThirdParty(context.Background())
	require.NoError(t, err)
	require.Len(t, providers, 1)
	created, err := client.AddThirdParty(context.Background(), AddThirdPartyParams{
		OIDCIssuerURL: "https://issuer.example.com",
	})
	require.NoError(t, err)
	require.Equal(t, "provider-1", created["id"])
	require.NoError(t, client.RemoveThirdParty(context.Background(), "provider-1"))
	require.NoError(t, client.SyncThirdParty(context.Background()))
	require.Equal(t, []string{
		"GET /auth/v1/config/third-party-auth",
		"POST /auth/v1/config/third-party-auth",
		"DELETE /auth/v1/config/third-party-auth/provider-1",
		"GET /auth/v1/config/third-party-auth/sync",
	}, requests)
}
