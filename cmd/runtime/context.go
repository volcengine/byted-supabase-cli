package runtime

import (
	"context"
	"io"
)

type Context struct {
	Context context.Context
	Input   io.Reader
	Output  io.Writer
	Stderr  io.Writer
}
