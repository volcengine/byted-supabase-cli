package commandtree

import (
	"io"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type RuntimeContext struct {
	Input  io.Reader
	FS     afero.Fs
	Output io.Writer
	Stderr io.Writer
	Mode   string
}

type CommandFactory func(RuntimeContext) *cobra.Command

type CommandGroup struct {
	Group     *cobra.Group
	Factories []CommandFactory
}

func DefaultRuntimeContext() RuntimeContext {
	return RuntimeContext{
		Input:  os.Stdin,
		FS:     afero.NewOsFs(),
		Output: os.Stdout,
		Stderr: os.Stderr,
	}
}
