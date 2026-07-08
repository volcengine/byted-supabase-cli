// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package pages

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// captureStdoutStderr swaps os.Stdout/os.Stderr for pipes, runs fn, and returns what each
// stream received. Output is tiny here, so closing the writers before reading cannot
// deadlock on the pipe buffer.
func captureStdoutStderr(t *testing.T, fn func() error) (stdout, stderr string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	errR, errW, err := os.Pipe()
	require.NoError(t, err)
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	runErr := fn()
	require.NoError(t, outW.Close())
	require.NoError(t, errW.Close())
	outBytes, _ := io.ReadAll(outR)
	errBytes, _ := io.ReadAll(errR)
	require.NoError(t, runErr)
	return string(outBytes), string(errBytes)
}

// TestOutputPagesProjectsJSONKeepsStdoutPure reproduces the reported case — an agent runs
// `pages list --name … -o json` and must (a) get parseable JSON on stdout with no hint
// text mixed in, and (b) still see the bare-domain warning, on stderr.
func TestOutputPagesProjectsJSONKeepsStdoutPure(t *testing.T) {
	defer func(v string) { utils.OutputFormat.Value = v }(utils.OutputFormat.Value)
	utils.OutputFormat.Value = utils.OutputJson

	stdout, stderr := captureStdoutStderr(t, func() error {
		return outputPagesProjects(volcengine.ListPagesProjectsResult{
			Total: 1,
			Projects: []volcengine.PagesProjectSummary{{
				PagesProjectID:   "m4m4zc8xsn",
				PagesProjectName: "filebox-demo-20260623",
				PreviewDomain:    "filebox-demo-20260623-m4m4zc8xsn.preview.iga-pages.com",
				SupabaseBindings: []volcengine.PagesBinding{{
					WorkspaceID: "savvy-mango-427643ag",
					BranchID:    "br-kind-erne-f8fb590b",
				}},
			}},
		})
	})

	// stdout is pure JSON — unmarshals cleanly and carries none of the hint text.
	var parsed volcengine.ListPagesProjectsResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &parsed))
	assert.Equal(t, "m4m4zc8xsn", parsed.Projects[0].PagesProjectID)
	assert.NotContains(t, stdout, "pages binding")
	assert.NotContains(t, stdout, "Note")
	// the warning still reaches the agent, on stderr, with workspace/branch pre-filled.
	assert.Contains(t, stderr, "will NOT open on its own")
	assert.Contains(t, stderr, "pages binding --workspace-id savvy-mango-427643ag --branch-id br-kind-erne-f8fb590b")
}

func TestPrintPagesDeployPreviewHint(t *testing.T) {
	var buf bytes.Buffer
	printPagesDeployPreviewHint(&buf)
	out := buf.String()
	// The deploy command returns no URL, so the hint must steer the agent to
	// `pages binding` and name the PreviewDomain field to read.
	assert.Contains(t, out, "does not return a site URL")
	assert.Contains(t, out, "pages binding")
	assert.Contains(t, out, "PreviewDomain")
}

func TestPrintPagesBindNextStepsHint(t *testing.T) {
	var buf bytes.Buffer
	printPagesBindNextStepsHint(&buf, "pages-123", "ws-abc", "br-xyz")
	out := buf.String()
	// After bind, the chain (deploy → binding) must be spelled out with the known
	// project / workspace / branch pre-filled so a weaker agent can continue.
	assert.Contains(t, out, "pages deploy pages-123")
	assert.Contains(t, out, "pages binding --workspace-id ws-abc --branch-id br-xyz")
	assert.Contains(t, out, "PreviewDomain")
}

func TestPrintPagesBareDomainHint(t *testing.T) {
	var buf bytes.Buffer
	printPagesBareDomainHint(&buf, "ws-abc", "br-xyz")
	out := buf.String()
	// The bare domain from list-style output is not openable; the hint must say so
	// and route to `pages binding` (pre-filled) for the real, token-carrying URL.
	assert.Contains(t, out, "will NOT open on its own")
	assert.Contains(t, out, "pages binding --workspace-id ws-abc --branch-id br-xyz")
	assert.Contains(t, out, "PreviewDomain")
}

func TestPrintPagesBareDomainHintFallsBackToPlaceholders(t *testing.T) {
	var buf bytes.Buffer
	printPagesBareDomainHint(&buf, "", "")
	assert.Contains(t, buf.String(), "pages binding --workspace-id <workspace-id> -o json")
}

func TestSinglePagesBinding(t *testing.T) {
	binding := func(ws, br string) volcengine.PagesBinding {
		return volcengine.PagesBinding{WorkspaceID: ws, BranchID: br}
	}

	// Exactly one project bound to exactly one branch → fully resolved.
	ws, br := singlePagesBinding([]volcengine.PagesProjectSummary{
		{SupabaseBindings: []volcengine.PagesBinding{binding("ws-1", "br-1")}},
	})
	assert.Equal(t, "ws-1", ws)
	assert.Equal(t, "br-1", br)

	// Anything ambiguous → empty so the hint uses placeholders.
	cases := [][]volcengine.PagesProjectSummary{
		nil,
		{{}},
		{{SupabaseBindings: []volcengine.PagesBinding{binding("ws-1", "br-1"), binding("ws-2", "br-2")}}},
		{
			{SupabaseBindings: []volcengine.PagesBinding{binding("ws-1", "br-1")}},
			{SupabaseBindings: []volcengine.PagesBinding{binding("ws-2", "br-2")}},
		},
	}
	for _, projects := range cases {
		ws, br := singlePagesBinding(projects)
		assert.Empty(t, ws)
		assert.Empty(t, br)
	}
}

func TestFastCreatePreviewDomain(t *testing.T) {
	result, err := fastCreatePreviewDomain(volcengine.DescribePagesBindingResult{
		Binding: &volcengine.PagesBinding{
			WorkspaceID:    "workspace-1",
			BranchID:       "branch-1",
			PagesProjectID: "pages-1",
		},
		PagesProject: &volcengine.PagesProjectSummary{
			PreviewDomain: " preview.example.com ",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "preview.example.com", result)
}

func TestFastCreatePreviewDomainRequiresBindingDetails(t *testing.T) {
	tests := []struct {
		name   string
		result volcengine.DescribePagesBindingResult
		want   string
	}{
		{
			name: "missing binding",
			want: "binding was not found",
		},
		{
			name: "missing project",
			result: volcengine.DescribePagesBindingResult{
				Binding: &volcengine.PagesBinding{},
			},
			want: "did not include project details",
		},
		{
			name: "missing preview domain",
			result: volcengine.DescribePagesBindingResult{
				Binding:      &volcengine.PagesBinding{},
				PagesProject: &volcengine.PagesProjectSummary{},
			},
			want: "did not include a preview domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fastCreatePreviewDomain(tt.result)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}
