package solo

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirmSoloWrite(t *testing.T) {
	t.Run("yes skips confirmation", func(t *testing.T) {
		command := &cobra.Command{Use: "test"}
		command.Flags().Bool("yes", true, "")
		assert.NoError(t, confirmSoloWrite(command, "delete test resource"))
	})

	t.Run("non tty requires yes", func(t *testing.T) {
		command := &cobra.Command{Use: "test"}
		command.Flags().Bool("yes", false, "")
		err := confirmSoloWrite(command, "delete test resource")
		assert.ErrorContains(t, err, "confirmation required")
	})

	t.Run("standalone command remains compatible", func(t *testing.T) {
		command := &cobra.Command{Use: "test"}
		err := confirmSoloWrite(command, "write test resource")
		require.NoError(t, err)
	})
}
