package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
)

func TestResolveRuntimeModeUsesSoloLinkState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "supabase", ".temp"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "supabase", ".temp", "link.json"), []byte(`{
		"version": 1,
		"type": "solo",
		"credential_ref": "linked"
	}`), 0600))
	require.NoError(t, writeSoloProfileForTest(t, "linked"))

	profile, err := resolveRuntimeMode(projectDir, afero.NewOsFs())
	require.NoError(t, err)
	require.Equal(t, RuntimeModeSolo, profile.Mode)
	require.Equal(t, projectDir, profile.ProjectDir)
	require.NotNil(t, profile.Solo)
	require.Equal(t, "https://branch.example.com", profile.Solo.Profile.SupabaseURL)
	require.Equal(t, "service-role-key", profile.Solo.Profile.ServiceRoleKey)
	require.Equal(t, "linked", profile.Solo.State.CredentialRef)
	require.Equal(t, soloconfig.ConfigSourceProfile, profile.Solo.Source)
}

func TestResolveRuntimeModeUsesManagementLinkState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "supabase", ".temp"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "supabase", ".temp", "link.json"), []byte(`{
		"version": 1,
		"type": "management",
		"workspace_id": "ws-1",
		"branch_id": "br-1"
	}`), 0600))

	profile, err := resolveRuntimeMode(projectDir, afero.NewOsFs())
	require.NoError(t, err)
	require.Equal(t, RuntimeModeManager, profile.Mode)
}

func TestResolveRuntimeModeUsesSoloEnvironmentWithoutLinkState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUPABASE_URL", "https://branch.example.com")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "service-role-key")
	t.Setenv("SUPABASE_ANON_KEY", "")

	profile, err := resolveRuntimeMode(t.TempDir(), afero.NewOsFs())
	require.NoError(t, err)
	require.Equal(t, RuntimeModeSolo, profile.Mode)
}

func TestResolveRuntimeModeUsesAnonEnvironmentWithoutLinkState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUPABASE_URL", "https://branch.example.com")
	t.Setenv("SUPABASE_ANON_KEY", "anon-key")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")

	profile, err := resolveRuntimeMode(t.TempDir(), afero.NewOsFs())
	require.NoError(t, err)
	require.Equal(t, RuntimeModeSolo, profile.Mode)
}

func TestResolveRuntimeModeDefaultsToManager(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")
	t.Setenv("SUPABASE_ANON_KEY", "")

	profile, err := resolveRuntimeMode(t.TempDir(), afero.NewOsFs())
	require.NoError(t, err)
	require.Equal(t, RuntimeModeManager, profile.Mode)
}

func TestResolveRuntimeModeDoesNotUseGlobalSoloProfileWithoutLink(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, writeSoloProfileForTest(t, "default"))

	profile, err := resolveRuntimeMode(t.TempDir(), afero.NewOsFs())
	require.NoError(t, err)
	require.Equal(t, RuntimeModeManager, profile.Mode)
	require.Nil(t, profile.Solo)
}

func writeSoloProfileForTest(t *testing.T, name string) error {
	t.Helper()
	configDir := filepath.Join(os.Getenv("HOME"), ".volcengine", "byted_supabase_solo")
	require.NoError(t, os.MkdirAll(configDir, 0700))
	data := []byte(`{"current":"` + name + `","profiles":{"` + name + `":{"name":"` + name + `","supabase_url":"https://branch.example.com","service_role_key":"service-role-key"}}}`)
	return os.WriteFile(filepath.Join(configDir, "config.json"), data, 0600)
}
