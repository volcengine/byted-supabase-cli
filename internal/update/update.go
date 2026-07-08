// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package update

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"golang.org/x/mod/semver"

	"github.com/volcengine/byted-supabase-cli/internal/skills"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

const (
	npmPackage        = "@byted-supabase/cli"
	repoURL           = "https://github.com/volcengine/supabase-cli"
	npmInstallTimeout = 10 * time.Minute
	verifyTimeout     = 10 * time.Second
	maxNpmOutput      = 2000
	osWindows         = "windows"
)

// Params controls the update command behaviour.
type Params struct {
	CheckOnly bool
	Force     bool
	JSON      bool
}

// streams bundles the machine-readable (Out, stdout) and human-readable
// (Err, stderr) output sinks. JSON envelopes go to Out; progress to Err.
type streams struct {
	Out io.Writer
	Err io.Writer
}

// --- install detection ---

type installMethod int

const (
	installNpm installMethod = iota
	installManual
)

type detectResult struct {
	method       installMethod
	resolvedPath string
	npmAvailable bool
}

func (d detectResult) canAutoUpdate() bool {
	return d.method == installNpm && d.npmAvailable
}

func (d detectResult) manualReason() string {
	if d.method == installNpm && !d.npmAvailable {
		return "installed via npm, but npm is not available in PATH"
	}
	return "not installed via npm"
}

// --- updater ---

type npmResult struct {
	stdout bytes.Buffer
	stderr bytes.Buffer
	err    error
}

func (r *npmResult) combined() string { return r.stdout.String() + r.stderr.String() }

// Updater manages self-update operations. Platform-specific methods
// (PrepareSelfReplace, CleanupStaleFiles, CanRestorePreviousVersion) live in
// selfreplace_unix.go and selfreplace_windows.go.
type Updater struct {
	backupCreated bool
}

func newUpdater() *Updater { return &Updater{} }

func (u *Updater) resolveExe() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

// detectInstallMethod determines how the CLI was installed (npm vs manual)
// and whether npm is available for an automatic update.
func (u *Updater) detectInstallMethod() detectResult {
	exe, err := u.resolveExe()
	if err != nil {
		return detectResult{method: installManual}
	}
	method := installManual
	if strings.Contains(exe, "node_modules") {
		method = installNpm
	}
	npmAvailable := false
	if method == installNpm {
		if _, err := exec.LookPath("npm"); err == nil {
			npmAvailable = true
		}
	}
	return detectResult{method: method, resolvedPath: exe, npmAvailable: npmAvailable}
}

// runNpmInstall executes `npm install -g @byted-supabase/cli@<version>`.
func (u *Updater) runNpmInstall(ctx context.Context, version string) *npmResult {
	r := &npmResult{}
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		r.err = errors.Errorf("npm not found in PATH: %w", err)
		return r
	}
	ctx, cancel := context.WithTimeout(ctx, npmInstallTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, npmPath, "install", "-g", npmPackage+"@"+version)
	cmd.Stdout = &r.stdout
	cmd.Stderr = &r.stderr
	r.err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		r.err = errors.Errorf("npm install timed out after %s", npmInstallTimeout)
	}
	return r
}

// verifyBinary runs `byted-supabase-cli --version` and checks the reported
// version matches expected (both stripped of any leading "v"). PATH resolution
// is preferred so npm's freshly-linked global bin is picked up.
func (u *Updater) verifyBinary(ctx context.Context, expected string) error {
	exe, err := exec.LookPath("byted-supabase-cli")
	if err != nil {
		if exe, err = u.resolveExe(); err != nil {
			return errors.Errorf("cannot locate binary: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, exe, "--version").Output()
	if ctx.Err() == context.DeadlineExceeded {
		return errors.New("binary verification timed out")
	}
	if err != nil {
		return errors.Errorf("binary not executable: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return errors.New("empty version output")
	}
	actual := strings.TrimPrefix(fields[len(fields)-1], "v")
	want := strings.TrimPrefix(expected, "v")
	if actual != want {
		return errors.Errorf("expected version %s, got %q", expected, actual)
	}
	return nil
}

// --- orchestration ---

// Run is the entry point for the `update` command. The actual update is always
// delegated to npm (never a silent binary swap) so npm keeps owning the version
// bookkeeping; on Windows the running .exe is renamed to .old first and rolled
// back if the install or verification fails.
func Run(ctx context.Context, params Params, stdout, stderr io.Writer) error {
	s := streams{Out: stdout, Err: stderr}
	updater := newUpdater()
	if !params.CheckOnly {
		updater.CleanupStaleFiles()
	}

	latest, err := utils.GetLatestRelease(ctx)
	if err != nil {
		return emitError(s, params, "network", fmt.Sprintf("failed to check latest version: %s", err), nil)
	}
	if !semver.IsValid(latest) {
		return emitError(s, params, "update_error", fmt.Sprintf("invalid version from registry: %q", latest), nil)
	}

	cur := strings.TrimSpace(utils.Version)
	if !params.Force && isUpToDate(latest, cur) {
		return reportAlreadyUpToDate(ctx, s, params, cur, latest)
	}

	detect := updater.detectInstallMethod()

	if params.CheckOnly {
		return reportCheckResult(s, params, cur, latest, detect.canAutoUpdate())
	}

	if !detect.canAutoUpdate() {
		return doManualUpdate(ctx, s, params, cur, latest, detect)
	}
	return doNpmUpdate(ctx, s, params, updater, cur, latest)
}

// maybeSyncSkills installs/updates the byted-supabase skill so it tracks the
// CLI version. It is a no-op for --check (read-only), for dev builds (skill
// sync is opt-in there via `skills install`), and when the skill is already in
// sync (unless --force). The skill is installed via npx and is independent of
// how the CLI binary itself was installed, so this runs on the manual-update
// path too.
func maybeSyncSkills(ctx context.Context, s streams, params Params, version string) *skills.SyncResult {
	if params.CheckOnly || version == "" {
		return nil
	}
	if !params.Force && skills.IsSynced(version) {
		return nil
	}
	if !params.JSON {
		fmt.Fprintf(s.Err, "\nSyncing %s skill ...\n", skills.Name())
	}
	return skills.Sync(ctx, skills.SyncOptions{Version: version, Force: params.Force})
}

func isUpToDate(latest, cur string) bool {
	if cur == "" {
		return false
	}
	cv := "v" + strings.TrimPrefix(cur, "v")
	if !semver.IsValid(cv) {
		return false
	}
	return semver.Compare(latest, cv) <= 0
}

func doNpmUpdate(ctx context.Context, s streams, params Params, updater *Updater, cur, latest string) error {
	restore, err := updater.PrepareSelfReplace()
	if err != nil {
		return emitError(s, params, "update_error", fmt.Sprintf("failed to prepare update: %s", err), nil)
	}

	if !params.JSON {
		fmt.Fprintf(s.Err, "Updating byted-supabase-cli %s %s %s via npm ...\n", displayVersion(cur), symArrow(), latest)
	}

	res := updater.runNpmInstall(ctx, strings.TrimPrefix(latest, "v"))
	if res.err != nil {
		restore()
		combined := res.combined()
		hint := permissionHint(combined)
		if !params.JSON && combined != "" {
			fmt.Fprint(s.Err, combined)
		}
		msg := fmt.Sprintf("npm install failed: %s", res.err)
		if hint != "" {
			msg += "\n  " + hint
		}
		return emitError(s, params, "update_error", msg, map[string]any{
			"detail": truncate(combined, maxNpmOutput),
			"hint":   hint,
		})
	}

	if err := updater.verifyBinary(ctx, latest); err != nil {
		restore()
		hint := verificationFailureHint(updater, latest)
		msg := fmt.Sprintf("new binary verification failed: %s\n  %s", err, hint)
		return emitError(s, params, "update_error", msg, map[string]any{"hint": hint})
	}

	sk := maybeSyncSkills(ctx, s, params, latest)

	if params.JSON {
		env := map[string]any{
			"ok": true, "previous_version": cur, "current_version": latest,
			"latest_version": latest, "action": "updated",
			"message": fmt.Sprintf("byted-supabase-cli updated to %s", latest),
			"url":     releasesURL(),
		}
		applySkills(env, sk)
		return emitJSON(s.Out, env)
	}
	fmt.Fprintf(s.Err, "\n%s Successfully updated byted-supabase-cli %s %s %s\n", symOK(), displayVersion(cur), symArrow(), latest)
	emitSkillsText(s, sk)
	return nil
}

func doManualUpdate(ctx context.Context, s streams, params Params, cur, latest string, detect detectResult) error {
	reason := detect.manualReason()
	// The binary can't auto-update, but the skill installs via npx regardless,
	// so keep it in sync with the running (still-current) binary version.
	sk := maybeSyncSkills(ctx, s, params, cur)
	if params.JSON {
		env := map[string]any{
			"ok": true, "previous_version": cur, "latest_version": latest,
			"action":  "manual_required",
			"message": fmt.Sprintf("automatic update unavailable: %s (path: %s)", reason, detect.resolvedPath),
			"command": fmt.Sprintf("npm install -g %s@%s", npmPackage, strings.TrimPrefix(latest, "v")),
			"url":     releasesURL(),
		}
		applySkills(env, sk)
		return emitJSON(s.Out, env)
	}
	fmt.Fprintf(s.Err, "Automatic update unavailable: %s (path: %s).\n\n", reason, detect.resolvedPath)
	fmt.Fprintf(s.Err, "Update manually:\n  npm install -g %s@%s\n", npmPackage, strings.TrimPrefix(latest, "v"))
	emitSkillsText(s, sk)
	return nil
}

func reportCheckResult(s streams, params Params, cur, latest string, canAutoUpdate bool) error {
	if params.JSON {
		return emitJSON(s.Out, map[string]any{
			"ok": true, "current_version": cur, "latest_version": latest,
			"action": "update_available", "auto_update": canAutoUpdate,
			"message": fmt.Sprintf("update available: %s %s %s", displayVersion(cur), symArrow(), latest),
			"url":     releasesURL(),
		})
	}
	fmt.Fprintf(s.Err, "Update available: %s %s %s\n", displayVersion(cur), symArrow(), latest)
	fmt.Fprintf(s.Err, "  Releases: %s\n", releasesURL())
	if canAutoUpdate {
		fmt.Fprintf(s.Err, "\nRun `byted-supabase-cli update` to install.\n")
	} else {
		fmt.Fprintf(s.Err, "\nNot installed via npm; update manually:\n  npm install -g %s@%s\n", npmPackage, strings.TrimPrefix(latest, "v"))
	}
	return nil
}

func reportAlreadyUpToDate(ctx context.Context, s streams, params Params, cur, latest string) error {
	sk := maybeSyncSkills(ctx, s, params, cur)
	if params.JSON {
		env := map[string]any{
			"ok": true, "current_version": cur, "latest_version": latest,
			"action":  "already_up_to_date",
			"message": fmt.Sprintf("byted-supabase-cli %s is already up to date", displayVersion(cur)),
		}
		applySkills(env, sk)
		return emitJSON(s.Out, env)
	}
	fmt.Fprintf(s.Err, "%s byted-supabase-cli %s is already up to date\n", symOK(), displayVersion(cur))
	emitSkillsText(s, sk)
	return nil
}

// --- output helpers ---

// emitError prints a JSON error envelope (when --json) and returns an error so
// the root command exits non-zero; non-JSON callers rely on the returned error
// being printed by the root error handler.
func emitError(s streams, params Params, errType, msg string, extra map[string]any) error {
	if params.JSON {
		e := map[string]any{"type": errType, "message": msg}
		for k, v := range extra {
			if str, ok := v.(string); ok && str == "" {
				continue
			}
			e[k] = v
		}
		_ = emitJSON(s.Out, map[string]any{"ok": false, "error": e})
	}
	return errors.New(msg)
}

func emitJSON(w io.Writer, payload map[string]any) error {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return errors.Errorf("failed to encode json: %w", err)
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

func permissionHint(npmOutput string) string {
	if runtime.GOOS != osWindows && strings.Contains(npmOutput, "EACCES") {
		return "Permission denied. Try `sudo byted-supabase-cli update`, or fix your npm global prefix: https://docs.npmjs.com/resolving-eacces-permissions-errors"
	}
	return ""
}

func verificationFailureHint(u *Updater, latest string) string {
	if u.CanRestorePreviousVersion() {
		return "the previous version has been restored"
	}
	return fmt.Sprintf("automatic rollback is unavailable; reinstall manually: npm install -g %s@%s", npmPackage, strings.TrimPrefix(latest, "v"))
}

// --- skills output helpers ---

func applySkills(env map[string]any, r *skills.SyncResult) {
	switch {
	case r == nil:
		env["skills"] = map[string]any{"action": "in_sync", "skill": skills.Name()}
	case r.Action == "failed":
		env["skills"] = map[string]any{
			"action": "failed", "skill": r.Skill,
			"warning": fmt.Sprintf("skill sync failed: %s", r.Err),
		}
	case r.Err != nil:
		// Installed, but the state file could not be written.
		env["skills"] = map[string]any{
			"action": "synced", "skill": r.Skill,
			"warning": r.Err.Error(),
		}
	default:
		env["skills"] = map[string]any{"action": "synced", "skill": r.Skill, "version": r.Version}
	}
}

func emitSkillsText(s streams, r *skills.SyncResult) {
	switch {
	case r == nil:
	case r.Action == "failed":
		fmt.Fprintf(s.Err, "%s Skill sync failed: %v\n", symWarn(), r.Err)
		fmt.Fprintf(s.Err, "  Retry with: byted-supabase-cli skills install --force\n")
	case r.Err != nil:
		fmt.Fprintf(s.Err, "%s Skill %s installed (state not saved: %v)\n", symWarn(), r.Skill, r.Err)
	default:
		fmt.Fprintf(s.Err, "%s Skill %s synced\n", symOK(), r.Skill)
	}
}

func displayVersion(v string) string {
	if v == "" {
		return "(dev)"
	}
	return "v" + strings.TrimPrefix(v, "v")
}

func releasesURL() string { return repoURL + "/releases" }

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[len(r)-maxLen:])
}

// --- terminal symbols (ASCII fallback on Windows) ---

func isWindows() bool { return runtime.GOOS == osWindows }

func symOK() string {
	if isWindows() {
		return "[OK]"
	}
	return "✓"
}

func symArrow() string {
	if isWindows() {
		return "->"
	}
	return "→"
}

func symWarn() string {
	if isWindows() {
		return "[WARN]"
	}
	return "⚠"
}
