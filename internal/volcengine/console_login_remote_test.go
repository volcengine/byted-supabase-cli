// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

// panicReader fails the test if anything attempts to read from it, proving that
// non-interactive/agent code paths never block on stdin.
type panicReader struct{ t *testing.T }

func (r panicReader) Read([]byte) (int, error) {
	r.t.Fatal("unexpected read from input in non-interactive/agent mode")
	return 0, nil
}

// syncBuffer is a goroutine-safe io.Writer, needed because remoteAuthorize
// writes output from a background goroutine while the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// parseWaitPath extracts the poll-file path from the stable output line
// "Waiting for authorization code — write it to: <path>". Returns "" until a
// full line (terminated by newline) is present.
func parseWaitPath(s string) string {
	const marker = "write it to: "
	idx := strings.Index(s, marker)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(marker):]
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:nl])
}

func forceAgentMode(t *testing.T) {
	t.Helper()
	prev := utils.AgentMode.Value
	utils.AgentMode.Value = "yes"
	t.Cleanup(func() { utils.AgentMode.Value = prev })
}

func TestResolveConsoleLoginRegionAgentModeUsesDefault(t *testing.T) {
	forceAgentMode(t)
	var out bytes.Buffer
	region, err := resolveConsoleLoginRegion(panicReader{t}, &out, "")
	require.NoError(t, err)
	assert.Equal(t, DefaultConsoleLoginRegion, region)
	assert.Contains(t, out.String(), DefaultConsoleLoginRegion)
}

func TestRemoteAuthorizeAgentModeAutoGeneratesPollFile(t *testing.T) {
	forceAgentMode(t)
	const state = "state-auto"
	out := &syncBuffer{}
	client := newConsoleOAuthClient(DefaultConsoleEndpoint)

	type authResult struct {
		code string
		err  error
	}
	resCh := make(chan authResult, 1)
	go func() {
		code, _, err := remoteAuthorize(context.Background(), panicReader{t}, out, client, consoleClientIDCrossDevice, "challenge", state, DefaultConsoleEndpoint)
		resCh <- authResult{code, err}
	}()

	// Discover the auto-generated poll file from the printed output.
	var path string
	deadline := time.Now().Add(5 * time.Second)
	for path == "" {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for auto-generated poll file path in output")
		}
		path = parseWaitPath(out.String())
		if path == "" {
			time.Sleep(10 * time.Millisecond)
		}
	}
	require.Contains(t, filepath.Base(path), "byted-supabase-login-")

	require.NoError(t, os.WriteFile(path, []byte(encodeRemoteAuthResponse("auto-code", state)+"\n"), 0600))

	select {
	case res := <-resCh:
		require.NoError(t, res.err)
		assert.Equal(t, "auto-code", res.code)
	case <-time.After(5 * time.Second):
		t.Fatal("remoteAuthorize did not return after code was written")
	}

	// The auto-generated temp file must be cleaned up afterwards.
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "auto-generated poll file should be removed, stat err: %v", statErr)
}

func encodeRemoteAuthResponse(code, state string) string {
	q := url.Values{}
	q.Set("code", code)
	q.Set("state", state)
	return base64.StdEncoding.EncodeToString([]byte(q.Encode()))
}

func TestDecodeRemoteAuthResponse(t *testing.T) {
	const state = "state-123"

	t.Run("valid response", func(t *testing.T) {
		raw := encodeRemoteAuthResponse("auth-code-abc", state)
		code, err := decodeRemoteAuthResponse(raw, state)
		require.NoError(t, err)
		assert.Equal(t, "auth-code-abc", code)
	})

	t.Run("all base64 variants", func(t *testing.T) {
		query := url.Values{"code": {"auth-code-abc"}, "state": {state}}.Encode()
		variants := []struct {
			name string
			enc  *base64.Encoding
		}{
			{"StdEncoding", base64.StdEncoding},
			{"RawStdEncoding", base64.RawStdEncoding},
			{"URLEncoding", base64.URLEncoding},
			{"RawURLEncoding", base64.RawURLEncoding},
		}
		for _, v := range variants {
			raw := v.enc.EncodeToString([]byte(query))
			code, err := decodeRemoteAuthResponse(raw, state)
			require.NoErrorf(t, err, "encoding %s should decode", v.name)
			assert.Equalf(t, "auth-code-abc", code, "encoding %s", v.name)
		}
	})

	t.Run("state mismatch", func(t *testing.T) {
		raw := encodeRemoteAuthResponse("auth-code-abc", "other-state")
		_, err := decodeRemoteAuthResponse(raw, state)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "state mismatch")
	})

	t.Run("empty", func(t *testing.T) {
		_, err := decodeRemoteAuthResponse("   ", state)
		require.Error(t, err)
	})

	t.Run("missing code", func(t *testing.T) {
		raw := base64.StdEncoding.EncodeToString([]byte("state=" + state))
		_, err := decodeRemoteAuthResponse(raw, state)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "code parameter")
	})

	t.Run("not base64", func(t *testing.T) {
		_, err := decodeRemoteAuthResponse("!!!notbase64!!!", state)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base64")
	})
}

func TestPollAuthCodeFromFileReadsCode(t *testing.T) {
	const state = "state-xyz"
	dir := t.TempDir()
	codeFile := filepath.Join(dir, "code")

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(codeFile, []byte(encodeRemoteAuthResponse("polled-code", state)+"\n"), 0600)
	}()

	code, err := pollAuthCodeFromFile(context.Background(), os.Stderr, codeFile, state)
	require.NoError(t, err)
	assert.Equal(t, "polled-code", code)
}

func TestPollAuthCodeFromFileRespectsContextCancel(t *testing.T) {
	dir := t.TempDir()
	codeFile := filepath.Join(dir, "code")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := pollAuthCodeFromFile(ctx, os.Stderr, codeFile, "state")
	require.ErrorIs(t, err, context.Canceled)
}

func TestPollAuthCodeFromFileStableInvalidContentErrors(t *testing.T) {
	dir := t.TempDir()
	codeFile := filepath.Join(dir, "code")
	// A stable, fully-written but invalid payload should surface a decode error
	// well before the 10-minute total timeout instead of polling forever.
	require.NoError(t, os.WriteFile(codeFile, []byte("not-valid-base64!!!\n"), 0600))

	// Guard against a regression that would block until remoteCodeFilePollTimeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	_, err := pollAuthCodeFromFile(ctx, os.Stderr, codeFile, "state")
	require.Error(t, err)
	require.NotErrorIs(t, err, context.DeadlineExceeded)
	assert.Contains(t, err.Error(), "invalid")
	assert.Less(t, time.Since(start), 30*time.Second)
}
