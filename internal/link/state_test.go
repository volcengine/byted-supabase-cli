package link

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func TestSaveAndLoadSoloState(t *testing.T) {
	fsys := afero.NewMemMapFs()

	require.NoError(t, SaveSoloState(fsys, "https://example.com/", "link-123"))

	state, err := LoadState(fsys)
	require.NoError(t, err)
	require.Equal(t, linkStateVersion, state.Version)
	require.Equal(t, LinkTypeSolo, state.Type)
	require.Equal(t, "https://example.com", state.SupabaseURL)
	require.Equal(t, "link-123", state.CredentialRef)
	require.True(t, IsSolo(fsys))

	data, err := afero.ReadFile(fsys, StatePath)
	require.NoError(t, err)
	require.NotContains(t, string(data), "service-role")
}

func TestSaveAndLoadManagementState(t *testing.T) {
	fsys := afero.NewMemMapFs()

	require.NoError(t, SaveManagementState(fsys, "ws-123", "br-456"))

	state, err := LoadState(fsys)
	require.NoError(t, err)
	require.Equal(t, LinkTypeManagement, state.Type)
	require.Equal(t, "ws-123", state.WorkspaceID)
	require.Equal(t, "br-456", state.BranchID)
	require.False(t, IsSolo(fsys))
}

func TestClearStateIsIdempotent(t *testing.T) {
	fsys := afero.NewMemMapFs()
	require.NoError(t, ClearState(fsys))
	require.NoError(t, SaveSoloState(fsys, "https://example.com", "link-123"))
	require.NoError(t, ClearState(fsys))
	_, err := LoadState(fsys)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestStatePathIsProjectScoped(t *testing.T) {
	require.Equal(t, filepath.Join(utils.TempDir, "link.json"), StatePath)
}
