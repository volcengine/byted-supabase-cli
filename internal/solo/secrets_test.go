package solo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

func TestSecretsClientListSetUnsetUsesDataPlaneAPI(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		require.Equal(t, "service-role-key", request.Header.Get("apikey"))
		require.Equal(t, "Bearer service-role-key", request.Header.Get("Authorization"))
		require.Equal(t, "byted-supabase-cli", request.Header.Get("X-Request-From"))
		switch request.Method {
		case http.MethodGet:
			_, _ = writer.Write([]byte(`[{"name":"Z_SECRET","value":"digest-z"},{"name":"A_SECRET","value":"digest-a"}]`))
		case http.MethodPost:
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			require.JSONEq(t, `[{"name":"A_SECRET","value":"value"}]`, string(body))
			writer.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			require.JSONEq(t, `["A_SECRET"]`, string(body))
			writer.WriteHeader(http.StatusOK)
		default:
			http.Error(writer, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client, err := NewSecretsClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)

	secrets, err := client.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"A_SECRET", "Z_SECRET"}, []string{secrets[0].Name, secrets[1].Name})
	require.NoError(t, client.Set(context.Background(), api.CreateSecretBody{{Name: "A_SECRET", Value: "value"}}))
	require.NoError(t, client.Unset(context.Background(), []string{"A_SECRET"}))
	require.Equal(t, []string{
		"GET /v1/projects/default/secrets",
		"POST /v1/projects/default/secrets",
		"DELETE /v1/projects/default/secrets",
	}, requests)
}

func TestParseSecretsSupportsEnvFileAndSkipsSystemPrefix(t *testing.T) {
	fsys := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fsys, "/tmp/.env", []byte("FROM_FILE=file-value\nSUPABASE_INTERNAL=skip\n"), 0600))

	secrets, err := ParseSecrets("/tmp/.env", fsys, "FROM_ARGS=arg-value")
	require.NoError(t, err)
	require.Len(t, secrets, 2)
	values := map[string]string{}
	for _, secret := range secrets {
		values[secret.Name] = secret.Value
	}
	require.Equal(t, "file-value", values["FROM_FILE"])
	require.Equal(t, "arg-value", values["FROM_ARGS"])
	require.NotContains(t, values, "SUPABASE_INTERNAL")
}

func TestSecretsClientPassesThroughHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"denied"}`, http.StatusForbidden)
	}))
	defer server.Close()

	client, err := NewSecretsClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	_, err = client.List(context.Background())
	require.ErrorContains(t, err, "Secrets API returned HTTP 403")
	require.ErrorContains(t, err, "denied")
}

func TestNewSecretsClientRejectsRemoteHTTP(t *testing.T) {
	_, err := NewSecretsClient(Profile{
		SupabaseURL:    "http://api.example.com",
		ServiceRoleKey: "service-role-key",
	})
	require.ErrorContains(t, err, "invalid Supabase URL")
}

func TestNewSecretsClientAllowsLocalHTTP(t *testing.T) {
	client, err := NewSecretsClient(Profile{
		SupabaseURL:    "http://localhost:54321",
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestSecretsJSONShapeDoesNotExposeValueFromClientList(t *testing.T) {
	body, err := json.Marshal([]api.SecretResponse{{Name: "SECRET", Value: "digest"}})
	require.NoError(t, err)
	require.JSONEq(t, `[{"name":"SECRET","value":"digest"}]`, string(body))
}
