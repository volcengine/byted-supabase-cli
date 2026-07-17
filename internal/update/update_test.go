// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		latest  string
		current string
		want    bool
	}{
		{"newer release available", "v0.1.21", "0.1.20", true},
		{"same release", "v0.1.20", "0.1.20", false},
		{"older release on registry", "v0.1.19", "0.1.20", false},
		// A git-describe dev build is ahead of its base tag; the registry
		// release with the same core must not count as newer.
		{"dev build ahead of its tag", "v0.1.20", "0.1.20-8-gc45ca44", false},
		{"dev build behind a newer release", "v0.1.21", "0.1.20-8-gc45ca44", true},
		{"v-prefixed current", "v0.1.21", "v0.1.20", true},
		{"empty current counts as outdated", "v0.1.20", "", true},
		{"garbage current counts as outdated", "v0.1.20", "not-a-version", true},
		{"invalid latest never nags", "", "0.1.20", false},
		{"both invalid", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsNewer(tt.latest, tt.current))
		})
	}
}
