package cmd

import (
	"os"
	"path/filepath"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	"github.com/volcengine/byted-supabase-cli/internal/link"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
)

type RuntimeMode string

const (
	RuntimeModeManager RuntimeMode = "manager"
	RuntimeModeSolo    RuntimeMode = "solo"
)

type RuntimeProfile struct {
	Mode       RuntimeMode
	ProjectDir string
	Solo       *soloconfig.Resolution
}

func ResolveRuntimeMode(ctx commandtree.RuntimeContext) (RuntimeProfile, error) {
	return resolveRuntimeMode(runtimeWorkdirFromArgs(), ctx.FS)
}

func resolveRuntimeMode(projectDir string, fsys afero.Fs) (RuntimeProfile, error) {
	projectDir = filepath.Clean(projectDir)
	state, err := link.LoadStateAtDir(projectDir)
	if err == nil {
		switch state.Type {
		case link.LinkTypeSolo:
			resolution, resolveErr := soloconfig.NewResolver(projectDir).Resolve()
			if resolveErr != nil {
				return RuntimeProfile{}, errors.Errorf("failed to resolve Solo runtime mode: %w", resolveErr)
			}
			return RuntimeProfile{
				Mode:       RuntimeModeSolo,
				ProjectDir: projectDir,
				Solo:       &resolution,
			}, nil
		case link.LinkTypeManagement:
			return RuntimeProfile{Mode: RuntimeModeManager, ProjectDir: projectDir}, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return RuntimeProfile{}, err
	}

	if _, configured, envErr := soloconfig.LoadEnvironmentProfile(); configured {
		if envErr != nil {
			return RuntimeProfile{}, envErr
		}
		return RuntimeProfile{Mode: RuntimeModeSolo, ProjectDir: projectDir}, nil
	}

	_ = fsys
	return RuntimeProfile{Mode: RuntimeModeManager, ProjectDir: projectDir}, nil
}
