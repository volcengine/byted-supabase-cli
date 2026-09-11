package unlink

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	linkstate "github.com/volcengine/byted-supabase-cli/internal/link"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
	"github.com/volcengine/byted-supabase-cli/internal/testing/apitest"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/credentials"
	"github.com/zalando/go-keyring"
)

func TestUnlinkCommand(t *testing.T) {
	keyring.MockInit()
	project := apitest.RandomProjectRef()

	t.Run("unlinks project", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		require.NoError(t, afero.WriteFile(fsys, utils.ProjectRefPath, []byte(project), 0644))
		// Save database password
		require.NoError(t, credentials.StoreProvider.Set(project, "test"))
		// Run test
		err := Run(context.Background(), fsys, true)
		// Check error
		assert.NoError(t, err)
		// Validate file does not exist
		exists, err := afero.Exists(fsys, utils.ProjectRefPath)
		assert.NoError(t, err)
		assert.False(t, exists)
		// Check credentials does not exist
		_, err = credentials.StoreProvider.Get(project)
		assert.ErrorIs(t, err, keyring.ErrNotFound)
	})

	t.Run("unlinks project without credentials", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		require.NoError(t, afero.WriteFile(fsys, utils.ProjectRefPath, []byte(project), 0644))
		// Run test
		err := Run(context.Background(), fsys, true)
		// Check error
		assert.NoError(t, err)
	})

	t.Run("throws error if not linked", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Run test
		err := Run(context.Background(), fsys, true)
		// Check error
		assert.ErrorIs(t, err, utils.ErrNotLinked)
	})

	t.Run("throws error on permission denied", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		require.NoError(t, afero.WriteFile(fsys, utils.ProjectRefPath, []byte(project), 0644))
		// Run test
		err := Run(context.Background(), afero.NewReadOnlyFs(fsys), true)
		// Check error
		assert.ErrorIs(t, err, os.ErrPermission)
	})
}

func TestRunSoloUnlinkKeepsSoloProfiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fsys := afero.NewMemMapFs()
	require.NoError(t, soloconfig.SaveProfile(soloconfig.Profile{
		Name:        "link-current",
		SupabaseURL: "https://current.example.com",
	}))
	require.NoError(t, soloconfig.SaveProfile(soloconfig.Profile{
		Name:        "link-other",
		SupabaseURL: "https://other.example.com",
	}))
	require.NoError(t, linkstate.SaveSoloState(fsys, "https://current.example.com", "link-current"))

	require.NoError(t, Run(context.Background(), fsys, true))

	cfg, err := soloconfig.LoadConfig()
	require.NoError(t, err)
	_, exists := cfg.Profiles["link-current"]
	require.True(t, exists)
	_, exists = cfg.Profiles["link-other"]
	require.True(t, exists)
	_, err = linkstate.LoadState(fsys)
	require.ErrorIs(t, err, os.ErrNotExist)
}
