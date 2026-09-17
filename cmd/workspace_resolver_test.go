// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/errdefs"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func TestWorkspaceResolverResolve(t *testing.T) {
	tests := []struct {
		name          string
		explicit      string
		linked        string
		allowLinked   bool
		expectedID    string
		expectedError error
	}{
		{
			name:        "explicit workspace takes precedence",
			explicit:    " explicit-workspace ",
			linked:      "linked-workspace",
			allowLinked: true,
			expectedID:  "explicit-workspace",
		},
		{
			name:        "linked workspace is used when explicit workspace is absent",
			linked:      "linked-workspace",
			allowLinked: true,
			expectedID:  "linked-workspace",
		},
		{
			name:          "missing workspace returns the shared error",
			allowLinked:   true,
			expectedError: errdefs.ErrMissingWorkspaceID,
		},
		{
			name:          "linked workspace is ignored when linking is disabled",
			linked:        "linked-workspace",
			allowLinked:   false,
			expectedError: errdefs.ErrMissingWorkspaceID,
		},
	}

	previousRegion := volcengine.RegionSetting()
	volcengine.SetRegionOverride("cn-beijing")
	t.Cleanup(func() {
		volcengine.SetRegionOverride(previousRegion)
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fsys := afero.NewMemMapFs()
			if test.linked != "" {
				require.NoError(t, utils.WriteFile(
					filepath.Join(utils.TempDir, volcengine.LinkedWorkspaceFile),
					[]byte(test.linked),
					fsys,
				))
			}

			resolved, err := (WorkspaceResolver{FS: fsys}).Resolve(
				context.Background(),
				test.explicit,
				test.allowLinked,
			)

			if test.expectedError != nil {
				require.ErrorIs(t, err, test.expectedError)
				require.Empty(t, resolved)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.expectedID, resolved)
		})
	}
}
