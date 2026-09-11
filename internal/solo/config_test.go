package solo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/link"
)

func TestSaveProfileUsesIndependentConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	err := SaveProfile(Profile{
		Name:           "default",
		SupabaseURL:    "https://example.supabase.co/",
		AnonKey:        "anon",
		ServiceRoleKey: "service",
	})
	require.NoError(t, err)

	profile, err := LoadCurrentProfile()
	require.NoError(t, err)
	require.Equal(t, "https://example.supabase.co", profile.SupabaseURL)

	configPath, err := ConfigFilePath()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".volcengine", configDirectory, configFileName), configPath)
	require.FileExists(t, configPath)
}

func TestSaveProfileRejectsInvalidURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	err := SaveProfile(Profile{Name: "default", SupabaseURL: "example.supabase.co", AnonKey: "anon"})
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid Supabase URL")
}

func TestConfigFilesHaveRestrictedPermissions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, SaveProfile(Profile{Name: "default", SupabaseURL: "https://example.com"}))

	configPath, err := ConfigFilePath()
	require.NoError(t, err)
	info, statErr := os.Stat(configPath)
	require.NoError(t, statErr)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestValidateSoloLinkAtDirRequiresReferencedProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	projectDir := t.TempDir()
	linkDir := filepath.Join(projectDir, "supabase", ".temp")
	require.NoError(t, os.MkdirAll(linkDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(linkDir, "link.json"), []byte(`{
		"version": 1,
		"type": "solo",
		"supabase_url": "https://example.supabase.co",
		"credential_ref": "link-missing"
	}`), 0600))

	err := ValidateSoloLinkAtDir(projectDir)
	require.ErrorContains(t, err, `linked-branch profile "link-missing" was not found`)
}

func TestValidateSoloLinkAtDirAcceptsValidProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	projectDir := t.TempDir()
	require.NoError(t, SaveProfile(Profile{
		Name:           "link-valid",
		SupabaseURL:    "https://example.supabase.co",
		ServiceRoleKey: "service-role-key",
	}))
	fsys := afero.NewOsFs()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "supabase", ".temp"), 0700))
	require.NoError(t, afero.WriteFile(fsys, link.StatePathForDir(projectDir), []byte(`{
		"version": 1,
		"type": "solo",
		"supabase_url": "https://example.supabase.co",
		"credential_ref": "link-valid"
	}`), 0600))

	require.NoError(t, ValidateSoloLinkAtDir(projectDir))
}

func TestLoadEnvironmentProfileRequiresURLAndKey(t *testing.T) {
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_ANON_KEY", "")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "service-role-key")
	t.Setenv("SUPABASE_DB_URL", "")
	t.Setenv("SUPABASE_DATABASE_URL", "")

	profile, configured, err := LoadEnvironmentProfile()
	require.True(t, configured)
	require.ErrorContains(t, err, "missing required environment variable(s): SUPABASE_URL")
	require.Empty(t, profile)
}

func TestLoadEnvironmentProfileReturnsValidProfile(t *testing.T) {
	t.Setenv("SUPABASE_URL", "https://example.supabase.co/")
	t.Setenv("SUPABASE_ANON_KEY", "anon-key")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")
	t.Setenv("SUPABASE_DATABASE_URL", "")
	t.Setenv("SUPABASE_DB_URL", "")

	profile, configured, err := LoadEnvironmentProfile()
	require.True(t, configured)
	require.NoError(t, err)
	require.Equal(t, "env", profile.Name)
	require.Equal(t, "https://example.supabase.co/", profile.SupabaseURL)
}

func TestResolverPrefersLinkedProfileOverEnvironment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUPABASE_URL", "https://environment.example.com")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "environment-key")
	t.Setenv("SUPABASE_ANON_KEY", "")
	t.Setenv("SUPABASE_DB_URL", "")
	t.Setenv("SUPABASE_DATABASE_URL", "")

	projectDir := t.TempDir()
	require.NoError(t, SaveProfile(Profile{
		Name:           "link-profile",
		SupabaseURL:    "https://profile.example.com",
		ServiceRoleKey: "profile-key",
	}))
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "supabase", ".temp"), 0700))
	require.NoError(t, os.WriteFile(link.StatePathForDir(projectDir), []byte(`{
		"version": 1,
		"type": "solo",
		"supabase_url": "https://profile.example.com",
		"credential_ref": "link-profile"
	}`), 0600))

	resolution, err := NewResolver(projectDir).Resolve()
	require.NoError(t, err)
	require.Equal(t, ConfigSourceProfile, resolution.Source)
	require.Equal(t, "https://profile.example.com", resolution.Profile.SupabaseURL)
}

func TestDeleteProfileRemovesRequestedProfileOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, SaveProfile(Profile{Name: "link-a", SupabaseURL: "https://a.example.com"}))
	require.NoError(t, SaveProfile(Profile{Name: "link-b", SupabaseURL: "https://b.example.com"}))

	require.NoError(t, DeleteProfile("link-a"))

	cfg, err := LoadConfig()
	require.NoError(t, err)
	_, exists := cfg.Profiles["link-a"]
	require.False(t, exists)
	_, exists = cfg.Profiles["link-b"]
	require.True(t, exists)
}
