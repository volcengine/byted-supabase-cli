package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	"github.com/volcengine/byted-supabase-cli/internal/link"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
)

func TestAssembleRootCommandSelectsOneTree(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	projectDir := t.TempDir()
	require.NoError(t, os.Chdir(projectDir))
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	ctx := commandtree.RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}

	managementRoot := newRootCommand()
	managementProfile, err := ResolveRuntimeMode(ctx)
	require.NoError(t, err)
	configureRootCommand(managementRoot, managementProfile)
	assembleRootCommand(managementRoot, managementProfile, ctx)
	require.NotNil(t, directCommand(managementRoot, "projects"))
	managementConfigure := directCommand(managementRoot, "configure")
	require.NotNil(t, managementConfigure)
	require.NotNil(t, directCommand(managementConfigure, "set"))

	require.NoError(t, soloconfig.SaveProfile(soloconfig.Profile{
		Name:        "default",
		SupabaseURL: "https://example.supabase.co",
		AnonKey:     "anon",
	}))
	require.NoError(t, os.MkdirAll(filepath.Dir(link.StatePath), 0755))
	require.NoError(t, os.WriteFile(link.StatePath, []byte(`{"version":1,"type":"solo","supabase_url":"https://example.supabase.co","credential_ref":"default"}`), 0600))

	soloRoot := newRootCommand()
	soloProfile, err := ResolveRuntimeMode(ctx)
	require.NoError(t, err)
	configureRootCommand(soloRoot, soloProfile)
	assembleRootCommand(soloRoot, soloProfile, ctx)
	require.Nil(t, directCommand(soloRoot, "projects"))
	require.NotNil(t, directCommand(soloRoot, "auth"))
	soloConfigure := directCommand(soloRoot, "configure")
	require.NotNil(t, soloConfigure)
	require.Nil(t, directCommand(soloConfigure, "set"))
	require.Nil(t, directCommand(soloConfigure, "unset"))
	require.NotNil(t, directCommand(soloRoot, "link"))
	require.NotNil(t, directCommand(soloRoot, "unlink"))
	soloLink := directCommand(soloRoot, "link")
	require.NotNil(t, soloLink)
	require.NotNil(t, directCommand(soloLink, "status"))
	require.Nil(t, soloLink.Flags().Lookup("workspace-id"))
	require.Nil(t, soloLink.Flags().Lookup("project-ref"))
	require.Nil(t, soloLink.Flags().Lookup("volc-project-name"))
	require.Nil(t, soloLink.Flags().Lookup("database-url"))
	require.Equal(t, 1, strings.Count(soloRoot.UsageString(), "Supabase Services:"))
	require.Contains(t, soloRoot.UsageString(), "Configuration:")
	require.Contains(t, soloRoot.UsageString(), "Tools:")
	require.NotContains(t, soloRoot.UsageString(), "Local Development:")
	require.Contains(t, soloRoot.Long, "Linked directly to a Supabase branch with the URL and keys you supplied.")
	require.Contains(t, soloRoot.Long, "byted-supabase-cli unlink")
	require.NotContains(t, soloRoot.Long, "Mode")
}

func TestConfigureRootCommandUsesExplicitWorkdirForRuntimeMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	projectDir := t.TempDir()
	originalArgs := os.Args
	os.Args = []string{"byted-supabase-cli", "--workdir", projectDir, "--help"}
	t.Cleanup(func() { os.Args = originalArgs })

	require.NoError(t, writeSoloProfileForTest(t, "default"))

	root := newRootCommand()
	ctx := commandtree.RuntimeContext{
		FS:     afero.NewOsFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	profile, err := ResolveRuntimeMode(ctx)
	require.NoError(t, err)
	configureRootCommand(root, profile)
	assembleRootCommand(root, profile, ctx)

	require.NotNil(t, directCommand(root, "projects"))
	require.NotNil(t, directCommand(root, "ai-gateway"))
	require.NotNil(t, root.Flag("region"))
	require.NotNil(t, root.Flag("experimental"))
}

func TestBuildRootCommandUsesOneRuntimeProfileForConfigurationAndAssembly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	projectDir := t.TempDir()
	originalArgs := os.Args
	os.Args = []string{"byted-supabase-cli", "--workdir", projectDir, "--help"}
	t.Cleanup(func() { os.Args = originalArgs })

	require.NoError(t, writeSoloProfileForTest(t, "default"))

	root, err := buildRootCommand()
	require.NoError(t, err)
	require.NotNil(t, directCommand(root, "projects"))
	require.NotNil(t, directCommand(root, "ai-gateway"))
	require.NotNil(t, root.Flag("region"))
	require.Contains(t, root.UsageString(), "Management APIs:")
	require.NotContains(t, root.UsageString(), "Supabase Services:")
}

func TestBuildRootCommandAssemblesExpectedTreeForRuntimeModes(t *testing.T) {
	tests := []struct {
		name             string
		linkType         string
		withEnvironment  bool
		wantSoloCommand  bool
		wantManagerGroup bool
	}{
		{
			name:            "solo link",
			linkType:        "solo",
			wantSoloCommand: true,
		},
		{
			name:             "management link",
			linkType:         "management",
			wantManagerGroup: true,
		},
		{
			name:            "solo environment",
			withEnvironment: true,
			wantSoloCommand: true,
		},
		{
			name:             "manager default",
			wantManagerGroup: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("SUPABASE_URL", "")
			t.Setenv("SUPABASE_ANON_KEY", "")
			t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")
			projectDir := t.TempDir()
			originalArgs := os.Args
			os.Args = []string{"byted-supabase-cli", "--workdir", projectDir, "--help"}
			t.Cleanup(func() { os.Args = originalArgs })

			if test.linkType != "" {
				linkDir := filepath.Join(projectDir, "supabase", ".temp")
				require.NoError(t, os.MkdirAll(linkDir, 0700))
				state := `{"version":1,"type":"` + test.linkType + `"}`
				if test.linkType == "solo" {
					state = `{"version":1,"type":"solo","credential_ref":"default"}`
					require.NoError(t, writeSoloProfileForTest(t, "default"))
				} else {
					state = `{"version":1,"type":"management","workspace_id":"ws-1","branch_id":"br-1"}`
				}
				require.NoError(t, os.WriteFile(filepath.Join(linkDir, "link.json"), []byte(state), 0600))
			}
			if test.withEnvironment {
				t.Setenv("SUPABASE_URL", "https://branch.example.com")
				t.Setenv("SUPABASE_ANON_KEY", "anon-key")
			} else {
				t.Setenv("SUPABASE_URL", "")
				t.Setenv("SUPABASE_ANON_KEY", "")
				t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")
			}

			root, err := buildRootCommand()
			require.NoError(t, err)
			require.Equal(t, test.wantSoloCommand, directCommand(root, "projects") == nil)
			require.Equal(t, test.wantManagerGroup, strings.Contains(root.UsageString(), "Management APIs:"))
			require.NotContains(t, root.UsageString(), "group_id")
		})
	}
}

func directCommand(root *cobra.Command, name string) *cobra.Command {
	for _, command := range root.Commands() {
		if command.Name() == name {
			return command
		}
	}
	return nil
}
