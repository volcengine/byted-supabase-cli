// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/functions/deploy"
	"github.com/volcengine/byted-supabase-cli/internal/functions/gateway"
)

func scaffoldFileByName(files []gateway.File, name string) (gateway.File, bool) {
	for _, f := range files {
		if f.Name == name {
			return f, true
		}
	}
	return gateway.File{}, false
}

func TestEnsurePythonScaffold(t *testing.T) {
	t.Run("injects run.sh and requirements for python", func(t *testing.T) {
		files := []gateway.File{{Name: "app.py", Content: "print('hi')"}}
		got := ensurePythonScaffold(files, deploy.RuntimePython312, "")

		run, ok := scaffoldFileByName(got, "run.sh")
		require.True(t, ok, "run.sh should be injected")
		assert.Contains(t, run.Content, "uvicorn app:app")

		req, ok := scaffoldFileByName(got, "requirements.txt")
		require.True(t, ok, "requirements.txt should be injected")
		assert.Contains(t, req.Content, "uvicorn[standard]")
	})

	t.Run("leaves non-python runtimes untouched", func(t *testing.T) {
		files := []gateway.File{{Name: "index.ts", Content: "export default {}"}}
		got := ensurePythonScaffold(files, deploy.RuntimeNativeNode20, "")
		assert.Len(t, got, 1)
		_, ok := scaffoldFileByName(got, "run.sh")
		assert.False(t, ok, "run.sh must not be injected for node runtime")
	})

	t.Run("honours caller-supplied requirements override", func(t *testing.T) {
		files := []gateway.File{{Name: "app.py", Content: "print('hi')"}}
		got := ensurePythonScaffold(files, deploy.RuntimePython310, "flask\nrequests")

		req, ok := scaffoldFileByName(got, "requirements.txt")
		require.True(t, ok)
		assert.Equal(t, "flask\nrequests\n", req.Content)
		assert.NotContains(t, req.Content, "uvicorn")
	})

	t.Run("does not overwrite caller-supplied scaffold files", func(t *testing.T) {
		files := []gateway.File{
			{Name: "app.py", Content: "print('hi')"},
			{Name: "run.sh", Content: "custom launcher"},
			{Name: "requirements.txt", Content: "numpy"},
		}
		got := ensurePythonScaffold(files, deploy.RuntimePython39, "ignored")
		assert.Len(t, got, 3)

		run, _ := scaffoldFileByName(got, "run.sh")
		assert.Equal(t, "custom launcher", run.Content)
		req, _ := scaffoldFileByName(got, "requirements.txt")
		assert.Equal(t, "numpy", req.Content)
	})
}
