// Copyright (c) 2021 Supabase, Inc. and contributors
// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT
//
// This file has been modified by ByteDance Ltd. and/or its affiliates.
//
// Original file was released under MIT License, with the full license text
// available at https://github.com/supabase/cli/blob/main/LICENSE.
//
// This modified file is released under the same license.

package new

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/functions/deploy"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func TestNewCommand(t *testing.T) {
	t.Run("creates new function", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Run test
		assert.NoError(t, Run(context.Background(), "test-func", "", fsys))
		// Validate output
		funcPath := filepath.Join(utils.FunctionsDir, "test-func", "index.ts")
		content, err := afero.ReadFile(fsys, funcPath)
		assert.NoError(t, err)
		assert.Contains(t, string(content),
			"curl -i --location --request POST 'http://127.0.0.1:54321/functions/v1/test-func'",
		)

		// Verify config.toml is updated
		configContent, err := afero.ReadFile(fsys, utils.ConfigPath)
		assert.NoError(t, err, "config.toml should be created")
		assert.Contains(t, string(configContent), `runtime = "native-node20/v1"`)
		assert.Contains(t, string(configContent), `entrypoint = "./functions/test-func/index.ts"`)

		// Volcengine runtime uses a read-only function directory, so new Deno
		// templates must not trigger lockfile writes during cold start.
		denoPath := filepath.Join(utils.FunctionsDir, "test-func", "deno.json")
		_, err = afero.ReadFile(fsys, denoPath)
		assert.Error(t, err, "deno.json should not be created by default")

		npmrcPath := filepath.Join(utils.FunctionsDir, "test-func", ".npmrc")
		_, err = afero.ReadFile(fsys, npmrcPath)
		assert.Error(t, err, ".npmrc should not be created by default")
	})

	t.Run("creates python function", func(t *testing.T) {
		fsys := afero.NewMemMapFs()

		assert.NoError(t, Run(context.Background(), "python-func", deploy.RuntimePython39, fsys))

		appContent, err := afero.ReadFile(fsys, filepath.Join(utils.FunctionsDir, "python-func", "app.py"))
		assert.NoError(t, err)
		assert.Contains(t, string(appContent), "FastAPI")

		runInfo, err := fsys.Stat(filepath.Join(utils.FunctionsDir, "python-func", "run.sh"))
		assert.NoError(t, err)
		assert.Equal(t, uint32(0755), uint32(runInfo.Mode().Perm()))

		requirements, err := afero.ReadFile(fsys, filepath.Join(utils.FunctionsDir, "python-func", "requirements.txt"))
		assert.NoError(t, err)
		assert.Contains(t, string(requirements), "uvicorn[standard]")

		_, err = afero.ReadFile(fsys, filepath.Join(utils.FunctionsDir, "python-func", "index.ts"))
		assert.Error(t, err, "python template should not create index.ts")

		configContent, err := afero.ReadFile(fsys, utils.ConfigPath)
		assert.NoError(t, err)
		assert.Contains(t, string(configContent), `runtime = "native-python3.9/v1"`)
		assert.Contains(t, string(configContent), `entrypoint = "./functions/python-func/app.py"`)
	})

	t.Run("throws error on malformed slug", func(t *testing.T) {
		assert.Error(t, Run(context.Background(), "@", "", afero.NewMemMapFs()))
	})

	t.Run("throws error on duplicate slug", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		funcPath := filepath.Join(utils.FunctionsDir, "test-func", "index.ts")
		require.NoError(t, afero.WriteFile(fsys, funcPath, []byte{}, 0644))
		// Run test
		assert.Error(t, Run(context.Background(), "test-func", "", fsys))
	})

	t.Run("throws error on permission denied", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewReadOnlyFs(afero.NewMemMapFs())
		// Run test
		assert.Error(t, Run(context.Background(), "test-func", "", fsys))
	})
}
