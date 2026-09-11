// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package solo

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
)

// The help text of a directly linked branch must not leak the internal
// "Solo" codename, the "data plane" layering vocabulary, or "instance" where
// the product term is "branch".
func TestCommandTreeHelpAvoidsInternalVocabulary(t *testing.T) {
	root := NewCommandTree(commandtree.RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})

	banned := []string{"Solo", "solo", "data plane", "data-plane", "Data Plane", "instance"}
	var walk func(command *cobra.Command)
	walk = func(command *cobra.Command) {
		for _, word := range banned {
			require.NotContains(t, command.Short, word, "%q Short leaks %q", command.CommandPath(), word)
			require.NotContains(t, command.Long, word, "%q Long leaks %q", command.CommandPath(), word)
		}
		command.Flags().VisitAll(func(flag *pflag.Flag) {
			for _, word := range banned {
				require.NotContains(t, flag.Usage, word, "%q --%s leaks %q", command.CommandPath(), flag.Name, word)
			}
		})
		for _, child := range command.Commands() {
			walk(child)
		}
	}
	walk(root)
	require.NotEmpty(t, strings.TrimSpace(root.Short))
}
