package deploy

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
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
