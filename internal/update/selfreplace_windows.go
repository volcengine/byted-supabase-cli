// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

//go:build windows

package update

import (
	"os"

	"github.com/go-errors/errors"
)

// PrepareSelfReplace renames the running .exe to .old so that npm's postinstall
// script can write the new binary without hitting a file-in-use error. Returns
// a restore function that undoes the rename on failure.
func (u *Updater) PrepareSelfReplace() (restore func(), err error) {
	noop := func() {}

	exe, err := u.resolveExe()
	if err != nil {
		return noop, nil // best-effort; don't block the update
	}
	oldPath := exe + ".old"

	// Clean up any stale .old from a previous upgrade.
	_ = os.Remove(oldPath)

	// Windows allows renaming a locked (running) executable.
	if err := os.Rename(exe, oldPath); err != nil {
		return noop, errors.Errorf("cannot rename binary for update: %w", err)
	}
	u.backupCreated = true

	// Restore: move .old back to the original path. Guard with Stat in case the
	// new install already recovered it; on any failure clear backupCreated so
	// CanRestorePreviousVersion reports the real outcome.
	restore = func() {
		if _, err := os.Stat(oldPath); err != nil {
			u.backupCreated = false
			return
		}
		_ = os.Remove(exe)
		if err := os.Rename(oldPath, exe); err != nil {
			u.backupCreated = false
		}
	}
	return restore, nil
}

// CleanupStaleFiles removes leftover .old files from previous upgrades. If the
// original binary is missing but .old exists (crash mid-update), it restores
// the .old to recover the installation.
func (u *Updater) CleanupStaleFiles() {
	exe, err := u.resolveExe()
	if err != nil {
		return
	}
	oldPath := exe + ".old"

	if _, err := os.Stat(oldPath); err != nil {
		return // no .old file
	}
	if _, err := os.Stat(exe); err != nil {
		// Original missing, .old exists — restore to recover.
		_ = os.Rename(oldPath, exe)
		return
	}
	// Both exist — .old is stale, clean up.
	_ = os.Remove(oldPath)
}

// CanRestorePreviousVersion reports whether PrepareSelfReplace created a
// restorable backup for the current update attempt.
func (u *Updater) CanRestorePreviousVersion() bool {
	return u.backupCreated
}
