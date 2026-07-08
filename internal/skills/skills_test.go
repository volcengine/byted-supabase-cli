// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package skills

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// isolate points HOME at a temp dir so the global state file lives there, and
// clears the env that would otherwise suppress drift checks (CI vars, opt-out).
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CI", "")
	t.Setenv("BUILD_NUMBER", "")
	t.Setenv("RUN_ID", "")
	t.Setenv(envNoNotifier, "")
	t.Setenv(envSource, "")
	t.Setenv(envName, "")
}

type fakeRunner struct {
	err      error
	calls    int
	gotForce bool
}

func (f *fakeRunner) Install(_ context.Context, _, _ string, force bool) *CmdResult {
	f.calls++
	f.gotForce = force
	r := &CmdResult{}
	if f.err != nil {
		r.Err = f.err
		r.Stderr.WriteString("install boom")
	}
	return r
}

func fixedNow() time.Time { return time.Unix(1_700_000_000, 0).UTC() }

func TestStateRoundTrip(t *testing.T) {
	isolate(t)

	if _, ok, err := ReadState(); err != nil || ok {
		t.Fatalf("cold start: want (_, false, nil), got ok=%v err=%v", ok, err)
	}

	want := State{Skill: "byted-supabase", Source: Source(), Version: "1.2.3", UpdatedAt: "now"}
	if err := WriteState(want); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	got, ok, err := ReadState()
	if err != nil || !ok {
		t.Fatalf("ReadState after write: ok=%v err=%v", ok, err)
	}
	if got.Version != "1.2.3" || got.Skill != "byted-supabase" {
		t.Fatalf("ReadState mismatch: %+v", got)
	}
}

func TestIsSynced(t *testing.T) {
	isolate(t)

	if IsSynced("1.0.0") {
		t.Fatal("IsSynced should be false with no state")
	}
	if err := WriteState(State{Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if !IsSynced("1.0.0") {
		t.Fatal("IsSynced should be true for matching version")
	}
	if !IsSynced("v1.0.0") {
		t.Fatal("IsSynced should normalize the v prefix")
	}
	if IsSynced("2.0.0") {
		t.Fatal("IsSynced should be false for a different version")
	}
}

func TestSyncSuccessWritesState(t *testing.T) {
	isolate(t)

	r := &fakeRunner{}
	res := Sync(context.Background(), SyncOptions{Version: "1.4.0", Force: true, Runner: r, Now: fixedNow})
	if res.Action != "synced" || res.Err != nil {
		t.Fatalf("want synced, got action=%q err=%v", res.Action, res.Err)
	}
	if r.calls != 1 || !r.gotForce {
		t.Fatalf("runner not called with force: calls=%d force=%v", r.calls, r.gotForce)
	}
	if !IsSynced("1.4.0") {
		t.Fatal("state should record the synced version")
	}
	state, _, _ := ReadState()
	if state.UpdatedAt != fixedNow().Format(time.RFC3339) {
		t.Fatalf("UpdatedAt = %q", state.UpdatedAt)
	}
}

func TestSyncFailureSkipsState(t *testing.T) {
	isolate(t)

	r := &fakeRunner{err: errors.New("npx exploded")}
	res := Sync(context.Background(), SyncOptions{Version: "1.4.0", Runner: r})
	if res.Action != "failed" || res.Err == nil {
		t.Fatalf("want failed, got action=%q err=%v", res.Action, res.Err)
	}
	if res.Detail == "" {
		t.Fatal("failure should carry combined-output detail")
	}
	if _, ok, _ := ReadState(); ok {
		t.Fatal("failed sync must not write state")
	}
}

func TestShouldSkip(t *testing.T) {
	isolate(t)

	cases := []struct {
		name    string
		version string
		setup   func(t *testing.T)
		want    bool
	}{
		{"release", "1.2.3", nil, false},
		{"v-prefixed release", "v1.2.3", nil, false},
		{"empty/dev build", "", nil, true},
		{"literal dev", "dev", nil, true},
		{"git describe build", "1.2.3-5-gabcdef", nil, true},
		{"opt-out env", "1.2.3", func(t *testing.T) { t.Setenv(envNoNotifier, "1") }, true},
		{"CI env", "1.2.3", func(t *testing.T) { t.Setenv("CI", "true") }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolate(t)
			if tc.setup != nil {
				tc.setup(t)
			}
			if got := shouldSkip(tc.version); got != tc.want {
				t.Fatalf("shouldSkip(%q) = %v, want %v", tc.version, got, tc.want)
			}
		})
	}
}

func TestInitDriftSetsPending(t *testing.T) {
	isolate(t)
	if err := WriteState(State{Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}

	Init("1.1.0")
	n := GetPending()
	if n == nil {
		t.Fatal("expected a pending stale notice on drift")
	}
	if n.Current != "1.0.0" || n.Target != "1.1.0" {
		t.Fatalf("notice = %+v", n)
	}

	// No drift clears it.
	Init("1.0.0")
	if GetPending() != nil {
		t.Fatal("matching version should clear the pending notice")
	}

	// Cold start (no state) emits nothing.
	isolate(t)
	Init("1.0.0")
	if GetPending() != nil {
		t.Fatal("cold start should not emit a notice")
	}
}

func TestInitSkipsWhenSuppressed(t *testing.T) {
	isolate(t)
	if err := WriteState(State{Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envNoNotifier, "1")
	Init("1.1.0")
	if GetPending() != nil {
		t.Fatal("opt-out env should suppress the notice despite drift")
	}
}

func TestStaleNoticeMessage(t *testing.T) {
	isolate(t)
	n := &StaleNotice{Current: "v1.0.0", Target: "1.1.0"}
	msg := n.Message()
	for _, want := range []string{"byted-supabase", "1.0.0", "1.1.0", "skills install"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message %q missing %q", msg, want)
		}
	}
}
