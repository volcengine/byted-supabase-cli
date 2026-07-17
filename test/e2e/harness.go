// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

//go:build live_e2e

// Package e2e drives the compiled CLI binary end-to-end against a real
// Volcengine account: it provisions one ephemeral workspace per run, hands its
// coordinates to the tests, and deletes it on teardown. See README.md for the
// environment contract. Guarded by the live_e2e build tag so `go test ./...`
// never touches the cloud.
package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// envBin points at the CLI binary under test. Defaults to the
	// `make build-cli` output (dist/byted-supabase-cli).
	envBin = "BYTED_SUPABASE_E2E_BIN"
	// envWorkspace reuses an existing workspace instead of provisioning one;
	// the harness then never deletes it. For local debugging.
	envWorkspace = "BYTED_SUPABASE_E2E_WORKSPACE"
	// envDelete opts into automatic teardown: the harness deletes the
	// workspace it provisioned (lifting the default deletion protection
	// first). Off by default — the harness never deletes anything unless
	// explicitly asked; locally you clean up by hand, CI sets this.
	envDelete = "BYTED_SUPABASE_E2E_DELETE"
	// envCreateArgs appends extra whitespace-separated args to
	// `projects create` (e.g. "--is-agent-plan --suspend-timeout-seconds 300").
	envCreateArgs = "BYTED_SUPABASE_E2E_CREATE_ARGS"
	// envUseProfile opts into authenticating via the local
	// ~/.volcengine/config.json profile instead of explicit env AK/SK.
	envUseProfile = "BYTED_SUPABASE_E2E_USE_PROFILE"
	// envReadyTimeout overrides how long to wait for Running (Go duration).
	envReadyTimeout = "BYTED_SUPABASE_E2E_READY_TIMEOUT"
	// envDataPlane opts into the data-plane suite (db query, storage,
	// functions, secrets, gen types). It enables public endpoint access on
	// the test workspace's default branch, so it is off by default.
	envDataPlane = "BYTED_SUPABASE_E2E_DATA_PLANE"

	createTimeout     = 3 * time.Minute
	queryTimeout      = 1 * time.Minute
	deleteTimeout     = 5 * time.Minute
	defaultReadyWait  = 15 * time.Minute
	readyPollInterval = 10 * time.Second
	statusRunning     = "Running"
	workspaceNameStem = "cli-e2e-live"
)

// Provisioning never recovers from these workspace statuses — fail fast
// instead of polling to the timeout.
var terminalBadStatuses = map[string]bool{
	"CreateFailed": true,
	"Error":        true,
	"Deleted":      true,
	"Closed":       true,
}

// harness is the per-run state shared by TestMain and the tests.
type harness struct {
	bin         string
	workDir     string // scratch cwd so the CLI never picks up this repo's supabase/ config
	workspaceID string
	provisioned bool // false when reusing BYTED_SUPABASE_E2E_WORKSPACE
	autoDelete  bool // deletion is strictly opt-in via BYTED_SUPABASE_E2E_DELETE
}

var h harness

// cliResult carries one CLI invocation's outcome. JSON goes to stdout,
// progress and nags to stderr, so tests parse stdout only.
type cliResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// createdWorkspace is the `projects create --output json` payload: the raw
// Workspace struct with PascalCase fields (internal/projects/create).
type createdWorkspace struct {
	WorkspaceID string `json:"WorkspaceId"`
	Status      string `json:"WorkspaceStatus"`
}

// workspaceDetail is the `projects list --workspace-id X --detail --output
// json` payload: the snake_case detail projection (internal/projects/list).
type workspaceDetail struct {
	ReferenceID              string `json:"reference_id"`
	Name                     string `json:"name"`
	Region                   string `json:"region"`
	Status                   string `json:"status"`
	DeletionProtectionStatus string `json:"deletion_protection_status"`
}

// apiKey is one entry of the `projects api-keys --output json` array.
type apiKey struct {
	Name string `json:"name"`
	Key  string `json:"api_key"`
	Type string `json:"type"`
}

func initHarness() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	h.bin = os.Getenv(envBin)
	if h.bin == "" {
		h.bin = filepath.Join(root, "dist", "byted-supabase-cli")
	}
	if _, err := os.Stat(h.bin); err != nil {
		return fmt.Errorf("CLI binary not found at %s — run `make build-cli` or set %s: %w", h.bin, envBin, err)
	}
	// Refuse to provision (and later delete) cloud resources against implicit
	// credentials: require explicit env AK/SK, or an explicit opt-in to use
	// the local profile. Mirrors upstream's empty-token refusal.
	if os.Getenv("VOLCENGINE_ACCESS_KEY") == "" || os.Getenv("VOLCENGINE_SECRET_KEY") == "" {
		if os.Getenv(envUseProfile) == "" {
			return fmt.Errorf("live e2e requires VOLCENGINE_ACCESS_KEY and VOLCENGINE_SECRET_KEY (or set %s=1 to use the local volcengine profile)", envUseProfile)
		}
	}
	h.workDir, err = os.MkdirTemp("", "byted-supabase-cli-e2e-")
	if err != nil {
		return fmt.Errorf("failed to create scratch workdir: %w", err)
	}
	h.workspaceID = os.Getenv(envWorkspace)
	h.autoDelete = os.Getenv(envDelete) != ""
	return nil
}

// repoRoot resolves the repository root from this source file's location, so
// the default binary path works regardless of the `go test` invocation cwd.
func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot resolve caller path for repo root")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file))), nil
}

// runCLI executes the binary under test with the given args in the scratch
// workdir, inheriting the process environment (AK/SK, region).
func runCLI(ctx context.Context, timeout time.Duration, args ...string) cliResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, h.bin, args...)
	cmd.Dir = h.workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := cliResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		res.ExitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
		}
		if ctx.Err() == context.DeadlineExceeded {
			res.Stderr += fmt.Sprintf("\n(e2e harness: command timed out after %s)", timeout)
		}
	}
	return res
}

func (r cliResult) failureDetail(what string) string {
	return fmt.Sprintf("%s failed (exit %d)\nstdout:\n%s\nstderr:\n%s", what, r.ExitCode, r.Stdout, r.Stderr)
}

// provisionWorkspace creates the ephemeral workspace and waits for Running.
// Any failure after creation deletes the half-provisioned workspace (unless
// keep is set) without masking the original error.
func provisionWorkspace(ctx context.Context) error {
	name := fmt.Sprintf("%s-%s-%s", workspaceNameStem, runID(), randHex(4))
	args := []string{"projects", "create", name, "--output", "json"}
	if extra := strings.Fields(os.Getenv(envCreateArgs)); len(extra) > 0 {
		args = append(args, extra...)
	}
	res := runCLI(ctx, createTimeout, args...)
	if res.ExitCode != 0 {
		return fmt.Errorf("%s", res.failureDetail("projects create"))
	}
	var created createdWorkspace
	if err := json.Unmarshal([]byte(res.Stdout), &created); err != nil {
		return fmt.Errorf("cannot parse `projects create` JSON output: %w\nstdout:\n%s", err, res.Stdout)
	}
	if created.WorkspaceID == "" {
		return fmt.Errorf("`projects create` returned no WorkspaceId\nstdout:\n%s", res.Stdout)
	}
	h.workspaceID = created.WorkspaceID
	h.provisioned = true
	fmt.Fprintf(os.Stderr, "e2e: created workspace %s (%s), waiting for %s ...\n", h.workspaceID, name, statusRunning)

	if err := waitForRunning(ctx); err != nil {
		if h.autoDelete {
			if delErr := deleteWorkspace(ctx); delErr != nil {
				fmt.Fprintf(os.Stderr, "e2e: failed to delete workspace after setup failure: %v\n", delErr)
			}
		} else {
			printManualCleanup()
		}
		return err
	}
	return nil
}

// printManualCleanup tells the operator exactly how to remove the workspace
// this run created (deletion protection is on by default, hence two steps).
func printManualCleanup() {
	fmt.Fprintf(os.Stderr, "e2e: workspace %s left alive (deletion is opt-in via %s) — clean up manually:\n", h.workspaceID, envDelete)
	fmt.Fprintf(os.Stderr, "  byted-supabase-cli projects deletion-protection %s --disable --yes\n", h.workspaceID)
	fmt.Fprintf(os.Stderr, "  byted-supabase-cli projects delete %s --yes\n", h.workspaceID)
}

func waitForRunning(ctx context.Context) error {
	wait := defaultReadyWait
	if v := os.Getenv(envReadyTimeout); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", envReadyTimeout, err)
		}
		wait = d
	}
	deadline := time.Now().Add(wait)
	var lastStatus string
	for time.Now().Before(deadline) {
		detail, err := fetchDetail(ctx)
		if err != nil {
			// Transient list failures are tolerated within the deadline; the
			// control plane can briefly lag right after creation.
			fmt.Fprintf(os.Stderr, "e2e: poll error (will retry): %v\n", err)
		} else {
			lastStatus = detail.Status
			if detail.Status == statusRunning {
				return nil
			}
			if terminalBadStatuses[detail.Status] {
				return fmt.Errorf("workspace %s entered terminal status %s during provisioning", h.workspaceID, detail.Status)
			}
		}
		time.Sleep(readyPollInterval)
	}
	return fmt.Errorf("workspace %s did not reach %s within %s (last status: %q)", h.workspaceID, statusRunning, wait, lastStatus)
}

func fetchDetail(ctx context.Context) (workspaceDetail, error) {
	res := runCLI(ctx, queryTimeout, "projects", "list", "--workspace-id", h.workspaceID, "--detail", "--output", "json")
	if res.ExitCode != 0 {
		return workspaceDetail{}, fmt.Errorf("%s", res.failureDetail("projects list --detail"))
	}
	var detail workspaceDetail
	if err := json.Unmarshal([]byte(res.Stdout), &detail); err != nil {
		return workspaceDetail{}, fmt.Errorf("cannot parse detail JSON: %w\nstdout:\n%s", err, res.Stdout)
	}
	return detail, nil
}

// deleteWorkspace tears the workspace down. New workspaces come with deletion
// protection enabled by default, so always lift it first (best-effort — the
// delete below surfaces any real problem). A failed deletion is returned
// loudly so a leaked workspace fails the run rather than silently accruing
// cost.
func deleteWorkspace(ctx context.Context) error {
	res := runCLI(ctx, queryTimeout, "projects", "deletion-protection", h.workspaceID, "--disable", "--yes")
	if res.ExitCode != 0 {
		fmt.Fprintf(os.Stderr, "e2e: %s\n", res.failureDetail("deletion-protection --disable"))
	}
	res = runCLI(ctx, deleteTimeout, "projects", "delete", h.workspaceID, "--yes")
	if res.ExitCode != 0 {
		return fmt.Errorf("%s", res.failureDetail("projects delete"))
	}
	return nil
}

// runID makes workspace names unique per CI run so a cleanup sweep can target
// exactly this run's leftovers.
func runID() string {
	for _, key := range []string{"GITHUB_RUN_ID", "CI_PIPELINE_ID", "BUILD_ID"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return fmt.Sprintf("%d", time.Now().Unix())
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// runCLIIn is runCLI with an explicit working directory, for tests that need
// their own project dir (init/link).
func runCLIIn(ctx context.Context, dir string, timeout time.Duration, args ...string) cliResult {
	saved := h.workDir
	h.workDir = dir
	defer func() { h.workDir = saved }()
	return runCLI(ctx, timeout, args...)
}

// hasEnvAccessKeys reports whether explicit AK/SK env credentials are present.
// Some commands (link) hard-require them and cannot fall back to the profile.
func hasEnvAccessKeys() bool {
	return os.Getenv("VOLCENGINE_ACCESS_KEY") != "" && os.Getenv("VOLCENGINE_SECRET_KEY") != ""
}
