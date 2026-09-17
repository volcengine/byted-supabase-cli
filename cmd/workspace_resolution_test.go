package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func TestWorkspaceResolverPrecedence(t *testing.T) {
	previousProjectRef := flags.ProjectRef
	previousRegion := volcengine.RegionSetting()
	flags.ProjectRef = "wrong-workspace"
	volcengine.SetRegionOverride("cn-beijing")
	t.Cleanup(func() {
		flags.ProjectRef = previousProjectRef
		volcengine.SetRegionOverride(previousRegion)
	})

	fsys := afero.NewMemMapFs()
	require.NoError(t, utils.WriteFile(
		filepath.Join(utils.TempDir, volcengine.LinkedWorkspaceFile),
		[]byte("linked-workspace"),
		fsys,
	))

	t.Run("explicit workspace wins for every preparer", func(t *testing.T) {
		functions := &functionsOptions{WorkspaceID: "functions-workspace", Linked: true}
		secrets := &secretsOptions{WorkspaceID: "secrets-workspace", Linked: true}
		storage := &storageOptions{WorkspaceID: "storage-workspace", Linked: true}
		seed := &seedOptions{WorkspaceID: "seed-workspace", Linked: true}
		restrictions := &networkRestrictionsOptions{WorkspaceID: "restrictions-workspace"}

		functionsCmd := &cobra.Command{Use: "functions"}
		newFunctionsWorkspaceFlags(functionsCmd, functions)
		require.NoError(t, functionsCmd.Flags().Set("workspace-id", functions.WorkspaceID))
		require.NoError(t, prepareFunctionsWorkspace(functionsCmd, fsys, functions, ""))

		secretsCmd := &cobra.Command{Use: "secrets"}
		addSecretsWorkspaceFlags(secretsCmd, secrets)
		require.NoError(t, secretsCmd.Flags().Set("workspace-id", secrets.WorkspaceID))
		require.NoError(t, prepareSecretsWorkspace(secretsCmd, fsys, secrets, ""))

		storageCmd := &cobra.Command{Use: "storage"}
		addStorageWorkspaceFlags(storageCmd, storage)
		require.NoError(t, storageCmd.Flags().Set("workspace-id", storage.WorkspaceID))
		require.NoError(t, prepareStorageWorkspace(storageCmd, fsys, storage, ""))

		seedCmd := &cobra.Command{Use: "seed"}
		addSeedWorkspaceFlags(seedCmd, seed)
		require.NoError(t, seedCmd.Flags().Set("workspace-id", seed.WorkspaceID))
		require.NoError(t, prepareSeedWorkspace(seedCmd, fsys, seed))

		restrictionsCmd := &cobra.Command{Use: "restrictions"}
		addNetworkRestrictionsWorkspaceFlags(restrictionsCmd, restrictions)
		require.NoError(t, restrictionsCmd.Flags().Set("workspace-id", restrictions.WorkspaceID))
		require.NoError(t, prepareNetworkRestrictionsWorkspace(restrictionsCmd, fsys, restrictions, ""))
		require.Equal(t, "wrong-workspace", flags.ProjectRef)
	})

	t.Run("linked fallback returns linked workspace", func(t *testing.T) {
		functions := &functionsOptions{Linked: true}
		functionsCmd := &cobra.Command{Use: "functions"}
		newFunctionsWorkspaceFlags(functionsCmd, functions)
		require.NoError(t, prepareFunctionsWorkspace(functionsCmd, fsys, functions, ""))
		require.Equal(t, "linked-workspace", functions.WorkspaceID)

		secrets := &secretsOptions{Linked: true}
		secretsCmd := &cobra.Command{Use: "secrets"}
		addSecretsWorkspaceFlags(secretsCmd, secrets)
		require.NoError(t, prepareSecretsWorkspace(secretsCmd, fsys, secrets, ""))
		require.Equal(t, "linked-workspace", secrets.WorkspaceID)

		storage := &storageOptions{Linked: true}
		storageCmd := &cobra.Command{Use: "storage"}
		addStorageWorkspaceFlags(storageCmd, storage)
		require.NoError(t, prepareStorageWorkspace(storageCmd, fsys, storage, ""))
		require.Equal(t, "linked-workspace", storage.WorkspaceID)

		seed := &seedOptions{Linked: true}
		seedCmd := &cobra.Command{Use: "seed"}
		addSeedWorkspaceFlags(seedCmd, seed)
		require.NoError(t, prepareSeedWorkspace(seedCmd, fsys, seed))
		require.Equal(t, "linked-workspace", seed.WorkspaceID)

		restrictions := &networkRestrictionsOptions{}
		restrictionsCmd := &cobra.Command{Use: "restrictions"}
		addNetworkRestrictionsWorkspaceFlags(restrictionsCmd, restrictions)
		require.NoError(t, prepareNetworkRestrictionsWorkspace(restrictionsCmd, fsys, restrictions, ""))
		require.Equal(t, "linked-workspace", restrictions.WorkspaceID)
	})

	t.Run("linked false does not return linked workspace", func(t *testing.T) {
		_, err := (WorkspaceResolver{FS: fsys}).Resolve(t.Context(), "", false)
		require.Error(t, err)
		require.Equal(t, "wrong-workspace", flags.ProjectRef)
	})
}

func TestWorkspacePreparersHonorLinkedFalse(t *testing.T) {
	previousRegion := volcengine.RegionSetting()
	volcengine.SetRegionOverride("cn-beijing")
	t.Cleanup(func() { volcengine.SetRegionOverride(previousRegion) })

	fsys := afero.NewMemMapFs()
	require.NoError(t, utils.WriteFile(
		filepath.Join(utils.TempDir, volcengine.LinkedWorkspaceFile),
		[]byte("linked-workspace"),
		fsys,
	))

	tests := []struct {
		name    string
		options interface{}
		add     func(*cobra.Command)
		prepare func(*cobra.Command, afero.Fs) error
	}{
		{
			name:    "functions",
			options: &functionsOptions{Linked: false},
			add: func(cmd *cobra.Command) {
				newFunctionsWorkspaceFlags(cmd, cmd.Context().Value("options").(*functionsOptions))
			},
			prepare: func(cmd *cobra.Command, fs afero.Fs) error {
				return prepareFunctionsWorkspace(cmd, fs, cmd.Context().Value("options").(*functionsOptions), "")
			},
		},
		{
			name:    "secrets",
			options: &secretsOptions{Linked: false},
			add: func(cmd *cobra.Command) {
				addSecretsWorkspaceFlags(cmd, cmd.Context().Value("options").(*secretsOptions))
			},
			prepare: func(cmd *cobra.Command, fs afero.Fs) error {
				return prepareSecretsWorkspace(cmd, fs, cmd.Context().Value("options").(*secretsOptions), "")
			},
		},
		{
			name:    "storage",
			options: &storageOptions{Linked: false},
			add: func(cmd *cobra.Command) {
				addStorageWorkspaceFlags(cmd, cmd.Context().Value("options").(*storageOptions))
			},
			prepare: func(cmd *cobra.Command, fs afero.Fs) error {
				return prepareStorageWorkspace(cmd, fs, cmd.Context().Value("options").(*storageOptions), "")
			},
		},
		{
			name:    "seed",
			options: &seedOptions{Linked: false},
			add:     func(cmd *cobra.Command) { addSeedWorkspaceFlags(cmd, cmd.Context().Value("options").(*seedOptions)) },
			prepare: func(cmd *cobra.Command, fs afero.Fs) error {
				return prepareSeedWorkspace(cmd, fs, cmd.Context().Value("options").(*seedOptions))
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: test.name}
			cmd.SetContext(context.WithValue(context.Background(), "options", test.options))
			test.add(cmd)
			require.NoError(t, cmd.Flags().Set("linked", "false"))
			require.Error(t, test.prepare(cmd, fsys))
			require.Empty(t, cmd.Flag("workspace-id").Value.String())
		})
	}
}
