// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

//go:build !windows

package update

// PrepareSelfReplace is a no-op on Unix, which allows overwriting a running
// executable via inode semantics.
func (u *Updater) PrepareSelfReplace() (restore func(), err error) {
	return func() {}, nil
}

// CleanupStaleFiles is a no-op on Unix (no .old files are created).
func (u *Updater) CleanupStaleFiles() {}

// CanRestorePreviousVersion reports whether PrepareSelfReplace created a
// restorable backup for the current update attempt.
func (u *Updater) CanRestorePreviousVersion() bool {
	return u.backupCreated
}
