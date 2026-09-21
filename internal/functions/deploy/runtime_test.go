package deploy

import (
	"path"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/functions/gateway"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/pkg/config"
)

func TestNormalizeVolcengineRuntime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "deno", input: RuntimeDeno, expected: RuntimeNativeNode20},
		{name: "node", input: "node", expected: RuntimeNativeNode20},
		{name: "node20", input: "node20", expected: RuntimeNativeNode20},
		{name: "native node", input: RuntimeNativeNode20, expected: RuntimeNativeNode20},
		{name: "python 3.9", input: "python3.9", expected: RuntimePython39},
		{name: "python 3.10", input: "python310", expected: RuntimePython310},
		{name: "python 3.12", input: "python", expected: RuntimePython310},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime, err := NormalizeVolcengineRuntime("hello", test.input, afero.NewMemMapFs())
			require.NoError(t, err)
			require.Equal(t, test.expected, runtime)
		})
	}
}

func TestNormalizeVolcengineRuntimeUsesConfigBeforeDetection(t *testing.T) {
	original := utils.Config.Functions
	t.Cleanup(func() { utils.Config.Functions = original })
	utils.Config.Functions = config.FunctionConfig{
		"hello": {Runtime: RuntimePython39},
	}

	fsys := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fsys, filepath.Join(utils.FunctionsDir, "hello", "run.sh"), []byte("#!/bin/sh"), 0755))
	runtime, err := NormalizeVolcengineRuntime("hello", RuntimeAuto, fsys)
	require.NoError(t, err)
	require.Equal(t, RuntimePython39, runtime)
}

func TestNormalizeVolcengineRuntimeDetectsPythonWhenConfigMissing(t *testing.T) {
	original := utils.Config.Functions
	t.Cleanup(func() { utils.Config.Functions = original })
	utils.Config.Functions = config.FunctionConfig{}

	fsys := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fsys, filepath.Join(utils.FunctionsDir, "hello-python39", "run.sh"), []byte("#!/bin/sh"), 0755))
	runtime, err := NormalizeVolcengineRuntime("hello-python39", RuntimeAuto, fsys)
	require.NoError(t, err)
	require.Equal(t, RuntimePython39, runtime)
}

func TestDefaultVolcengineEntrypoint(t *testing.T) {
	require.Equal(t, "index.ts", DefaultVolcengineEntrypoint(RuntimeNativeNode20))
	require.Equal(t, "app.py", DefaultVolcengineEntrypoint(RuntimePython312))
}

func TestCollectVolcengineFunctionFilesIncludesSharedFiles(t *testing.T) {
	fsys := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fsys,
		filepath.Join(utils.FunctionsDir, "hello", "index.ts"),
		[]byte(`import { sharedValue } from "../_shared/shared.ts";`), 0644))
	require.NoError(t, afero.WriteFile(fsys,
		filepath.Join(utils.FunctionsDir, "_shared", "shared.ts"),
		[]byte(`import { nestedValue } from "./nested.ts";`), 0644))
	require.NoError(t, afero.WriteFile(fsys,
		filepath.Join(utils.FunctionsDir, "_shared", "nested.ts"),
		[]byte(`export const nestedValue = "ok";`), 0644))
	require.NoError(t, afero.WriteFile(fsys,
		filepath.Join(utils.FunctionsDir, "_shared", "unused.ts"),
		[]byte(`export const unusedValue = "unused";`), 0644))
	require.NoError(t, afero.WriteFile(fsys,
		filepath.Join(utils.FunctionsDir, "other", "index.ts"),
		[]byte(`Deno.serve(() => new Response("other"));`), 0644))

	files, err := collectVolcengineFunctionFiles("hello", "", fsys)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"supabase/functions/hello/index.ts":    `import { sharedValue } from "../_shared/shared.ts";`,
		"supabase/functions/_shared/shared.ts": `import { nestedValue } from "./nested.ts";`,
		"supabase/functions/_shared/nested.ts": `export const nestedValue = "ok";`,
	}, functionFilesByName(files))
}

func TestVolcengineFunctionPathsNormalizeWindowsSeparators(t *testing.T) {
	original := utils.CurrentDirAbs
	t.Cleanup(func() { utils.CurrentDirAbs = original })
	utils.CurrentDirAbs = `C:\work\project`

	require.Equal(t,
		"supabase/functions/hello/index.ts",
		volcengineFunctionProjectPath("hello", `C:\work\project\supabase\functions\hello\index.ts`))
	require.Equal(t,
		"supabase/functions/hello/deno.json",
		volcengineFunctionProjectPath("hello", `supabase\functions\hello\deno.json`))
	require.Equal(t,
		"supabase/functions/_shared/shared.ts",
		pathFromWindowsImport(`supabase/functions/hello/index.ts`, `../_shared/shared.ts`))
}

func pathFromWindowsImport(current, module string) string {
	current = normalizeVolcenginePath(current)
	module = normalizeVolcenginePath(module)
	return path.Clean(path.Join(path.Dir(current), module))
}

func functionFilesByName(files []gateway.File) map[string]string {
	result := make(map[string]string, len(files))
	for _, file := range files {
		result[file.Name] = file.Content
	}
	return result
}
