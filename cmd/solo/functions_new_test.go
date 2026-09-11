package solo

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func TestFunctionsNewCommandIsLocalOnly(t *testing.T) {
	fsys := afero.NewMemMapFs()
	var output discardOutput
	command := newFunctionsNewCommand(commandtree.RuntimeContext{
		FS:     fsys,
		Output: &output,
		Stderr: &output,
	})
	command.SetArgs([]string{"hello", "--runtime", "deno"})

	require.NoError(t, command.Execute())
	_, err := fsys.Stat(filepath.Join(utils.FunctionsDir, "hello", "index.ts"))
	require.NoError(t, err)
}

type discardOutput struct{}

func (*discardOutput) Write(data []byte) (int, error) {
	return len(data), nil
}
