// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseBoolEnv(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "on", " On "} {
		assert.True(t, parseBoolEnv(v), v)
	}
	for _, v := range []string{"", "0", "false", "no", "off", "x"} {
		assert.False(t, parseBoolEnv(v), v)
	}
}

func TestSplitCSV(t *testing.T) {
	assert.Nil(t, splitCSV(""))
	assert.Nil(t, splitCSV("   "))
	assert.Equal(t, []string{"a", "b", "c"}, splitCSV("a,b,c"))
	// Per-item trimming/dedup is handled downstream by policy.
	assert.Equal(t, []string{"a", " b"}, splitCSV("a, b"))
}

func TestResolveFlagPrecedence(t *testing.T) {
	// flag set → flag wins, env ignored.
	assert.True(t, resolveBoolFlag(true, true, "off"))
	assert.Equal(t, "fromflag", resolveStringFlag(true, "fromflag", "fromenv"))
	assert.Equal(t, []string{"a"}, resolveSliceFlag(true, []string{"a"}, "x,y"))

	// flag unset → env fallback.
	assert.True(t, resolveBoolFlag(false, false, "true"))
	assert.Equal(t, "fromenv", resolveStringFlag(false, "", "  fromenv  "))
	assert.Equal(t, []string{"x", "y"}, resolveSliceFlag(false, nil, "x,y"))

	// flag unset, env empty → zero value.
	assert.False(t, resolveBoolFlag(false, false, ""))
	assert.Equal(t, "", resolveStringFlag(false, "", ""))
	assert.Nil(t, resolveSliceFlag(false, nil, ""))
}
