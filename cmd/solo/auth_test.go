package solo

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
)

func TestAuthConfigCommandTreeHasNoWorkspaceFlags(t *testing.T) {
	auth := newAuthCommand(commandtree.RuntimeContext{})
	config, _, err := auth.Find([]string{"config"})
	require.NoError(t, err)
	require.NotNil(t, config)
	for _, command := range config.Commands() {
		require.Nil(t, command.Flags().Lookup("workspace-id"))
		require.Nil(t, command.Flags().Lookup("project-ref"))
		require.Nil(t, command.Flags().Lookup("branch-id"))
	}
}

func TestAuthHooksAndThirdPartyCommandTreesHaveNoWorkspaceFlags(t *testing.T) {
	auth := newAuthCommand(commandtree.RuntimeContext{})
	for _, path := range [][]string{
		{"hooks", "get"},
		{"hooks", "set"},
		{"third-party", "list"},
		{"third-party", "add"},
		{"third-party", "remove"},
		{"third-party", "sync"},
	} {
		command, _, err := auth.Find(path)
		require.NoError(t, err)
		require.NotNil(t, command, "missing Solo Auth command %q", strings.Join(path, " "))
		for _, flag := range []string{"workspace-id", "project-ref", "branch-id", "linked"} {
			require.Nil(t, command.Flags().Lookup(flag), "%s leaked flag %s", command.CommandPath(), flag)
		}
	}
}

func TestAuthConfigGetCommandUsesCurrentSoloProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/auth/v1/config", request.URL.Path)
		require.Equal(t, "service-role-key", request.Header.Get("apikey"))
		_, _ = writer.Write([]byte(`{"SITE_URL":"https://example.com","DISABLE_SIGNUP":false}`))
	}))
	defer server.Close()
	require.NoError(t, soloconfig.SaveProfile(soloconfig.Profile{
		Name:           "default",
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	}))

	var output bytes.Buffer
	command := newAuthConfigGetCommand(commandtree.RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: &output,
	})
	command.SetArgs([]string{"--key", "site_url"})
	_, err := command.ExecuteC()
	require.NoError(t, err)
	require.Contains(t, output.String(), "SITE_URL")
	require.NotContains(t, output.String(), "service-role-key")
}

func TestAuthConfigSetCommandSupportsJSONPatch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPatch, request.Method)
		require.Equal(t, "/auth/v1/config", request.URL.Path)
		_, _ = writer.Write([]byte(`{"DISABLE_SIGNUP":true}`))
	}))
	defer server.Close()
	require.NoError(t, soloconfig.SaveProfile(soloconfig.Profile{
		Name:           "default",
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	}))

	var output bytes.Buffer
	command := newAuthConfigSetCommand(commandtree.RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: &output,
	})
	command.SetArgs([]string{"DISABLE_SIGNUP=false"})
	_, err := command.ExecuteC()
	require.NoError(t, err)
	require.Contains(t, output.String(), "DISABLE_SIGNUP")
}
