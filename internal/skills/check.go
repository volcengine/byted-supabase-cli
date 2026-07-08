// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package skills

import (
	"os"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

// releaseChannelPattern matches the prerelease suffixes we still treat as real
// releases worth notifying about (published npm prereleases). A git-describe
// build like "1.2.3-5-gabcdef" does NOT match and is treated as a dev build.
var releaseChannelPattern = regexp.MustCompile(`^(alpha|beta|rc|pre)(\.\d+)?$`)

// Init runs the cheap, local, network-free skill drift check. It compares the
// version recorded in skills-state.json against the running binary version and,
// on mismatch, stores a pending StaleNotice for Execute to print. Safe to call
// once near the end of cmd.Execute.
//
// Skip rules (shouldSkip): CI envs, dev/empty builds, non-release versions, and
// the BYTED_SUPABASE_CLI_NO_SKILLS_NOTIFIER opt-out. Nothing is emitted on a
// cold start (no state file yet) — the skill notice is reserved for genuine
// drift, not "never installed".
func Init(currentVersion string) {
	SetPending(nil)
	if shouldSkip(currentVersion) {
		return
	}
	recorded, ok := ReadSyncedVersion()
	if !ok {
		return
	}
	if normalizeVersion(recorded) == normalizeVersion(currentVersion) {
		return
	}
	SetPending(&StaleNotice{Current: recorded, Target: currentVersion})
}

// shouldSkip suppresses the skill notice in environments where it would be noise
// or wrong: opt-out, CI, dev builds (empty/"dev" version), and anything that is
// not a clean release semver (e.g. a local git-describe build).
func shouldSkip(version string) bool {
	if os.Getenv(envNoNotifier) != "" {
		return true
	}
	if isCIEnv() {
		return true
	}
	if version == "" || version == "dev" || version == "DEV" {
		return true
	}
	v := "v" + normalizeVersion(version)
	if !semver.IsValid(v) {
		return true
	}
	pre := strings.TrimPrefix(semver.Prerelease(v), "-")
	if pre == "" {
		return false // clean release
	}
	return !releaseChannelPattern.MatchString(pre)
}

func isCIEnv() bool {
	for _, key := range []string{"CI", "BUILD_NUMBER", "RUN_ID"} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}
