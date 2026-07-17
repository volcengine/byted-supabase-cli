// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package distribution

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRoot mimics the shape of the upstream tree: branding, the console auth
// surface, a Volcengine-only command, and a nested group.
func fakeRoot() *cobra.Command {
	root := &cobra.Command{
		Use:     "byted-supabase-cli",
		Short:   "Byted Supabase CLI 1.2.3",
		Version: "1.2.3",
	}
	endpoints := &cobra.Command{Use: "endpoints"}
	endpoints.AddCommand(
		&cobra.Command{Use: "eips"},
		&cobra.Command{Use: "list"},
	)
	root.AddCommand(
		&cobra.Command{Use: "login", Short: "Log in to the Volcengine console"},
		&cobra.Command{Use: "logout", Short: "Log out of the Volcengine console"},
		&cobra.Command{Use: "configure", Short: "Manage AK/SK profiles"},
		&cobra.Command{Use: "pages", Short: "Volcengine-only"},
		&cobra.Command{Use: "link", Short: "Link to a Supabase project"},
		endpoints,
	)
	return root
}

func commandByName(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func install(t *testing.T, d Distribution) {
	t.Helper()
	Set(d)
	t.Cleanup(func() { Set(nil) })
}

func TestApplyWithoutDistributionIsNoOp(t *testing.T) {
	Set(nil)
	root := fakeRoot()

	Apply(root)

	assert.Equal(t, "byted-supabase-cli", root.Use)
	assert.NotNil(t, commandByName(root, "configure"), "the plain upstream tree must be untouched")
	assert.Empty(t, root.Annotations, "an unapplied root must carry no marker")
}

func TestApplyNilRootSafe(t *testing.T) {
	install(t, Base{})
	assert.NotPanics(t, func() { Apply(nil) })
}

// rebrandOnly exercises CLIName/Brand in isolation.
type rebrandOnly struct{ Base }

func (rebrandOnly) CLIName() string { return "bytecloud-supabase-cli" }
func (rebrandOnly) Brand() string   { return "ByteCloud Supabase CLI" }

func TestApplyRebrands(t *testing.T) {
	install(t, rebrandOnly{})
	root := fakeRoot()

	Apply(root)

	assert.Equal(t, "bytecloud-supabase-cli", root.Use)
	assert.Equal(t, "ByteCloud Supabase CLI 1.2.3", root.Short)
}

// authSwap exercises AuthCommands in isolation.
type authSwap struct{ Base }

func (authSwap) AuthCommands() []*cobra.Command {
	return []*cobra.Command{
		{Use: "login", Short: "Log in to ByteCloud"},
		{Use: "logout", Short: "Log out of ByteCloud"},
	}
}

func TestApplyReplacesAuthSurface(t *testing.T) {
	install(t, authSwap{})
	root := fakeRoot()
	volcLogin := commandByName(root, "login")

	Apply(root)

	login := commandByName(root, "login")
	require.NotNil(t, login)
	assert.NotSame(t, volcLogin, login, "the console login must be replaced, not kept")
	assert.Contains(t, login.Short, "ByteCloud")
	assert.Nil(t, commandByName(root, "configure"),
		"a replaced auth surface must drop every built-in auth command, replacement or not")
	assert.NotNil(t, commandByName(root, "link"), "non-auth commands must survive")
}

// filtering exercises IncludeCommand in isolation, at both depths.
type filtering struct{ Base }

func (filtering) IncludeCommand(path ...string) bool {
	p := strings.Join(path, " ")
	return p != "pages" && p != "endpoints eips"
}

func TestApplyFiltersCommands(t *testing.T) {
	install(t, filtering{})
	root := fakeRoot()

	Apply(root)

	assert.Nil(t, commandByName(root, "pages"), "excluded root command must be unmounted")
	endpoints := commandByName(root, "endpoints")
	require.NotNil(t, endpoints, "included parent must survive")
	assert.Nil(t, commandByName(endpoints, "eips"), "excluded nested command must be unmounted")
	assert.NotNil(t, commandByName(endpoints, "list"), "included sibling must survive")
}

// extending exercises ExtraCommands in isolation.
type extending struct{ Base }

func (extending) ExtraCommands() []*cobra.Command {
	return []*cobra.Command{{Use: "aidap"}, {Use: "status"}}
}

func TestApplyMountsExtraCommands(t *testing.T) {
	install(t, extending{})
	root := fakeRoot()

	Apply(root)

	assert.NotNil(t, commandByName(root, "aidap"))
	assert.NotNil(t, commandByName(root, "status"))
}

// overriding exercises OverrideCommand in isolation, recording the visit order
// and decorating one command.
type overriding struct {
	Base
	visited *[]string
}

func (o overriding) OverrideCommand(path []string, cmd *cobra.Command) {
	*o.visited = append(*o.visited, strings.Join(path, " "))
	if strings.Join(path, " ") == "link" {
		cmd.Flags().String("project", "", "decorated")
	}
}

func TestApplyOverridesEveryCommand(t *testing.T) {
	visited := []string{}
	install(t, overriding{visited: &visited})
	root := fakeRoot()

	Apply(root)

	assert.Contains(t, visited, "", "the root itself must be offered for override")
	assert.Contains(t, visited, "link")
	assert.Contains(t, visited, "endpoints eips", "nested commands must be offered for override")
	link := commandByName(root, "link")
	require.NotNil(t, link)
	assert.NotNil(t, link.Flags().Lookup("project"), "override decorations must stick")
}

type extendingAgain struct{ extending }

func TestApplyIsIdempotentPerRoot(t *testing.T) {
	install(t, extendingAgain{})
	root := fakeRoot()

	Apply(root)
	Apply(root)

	var aidaps int
	for _, c := range root.Commands() {
		if c.Name() == "aidap" {
			aidaps++
		}
	}
	assert.Equal(t, 1, aidaps, "re-applying must not mount duplicates")
}

// byteCloudLike combines all five customizations the internal ByteCloud
// distribution needs, proving the seam covers its whole surgery surface:
// rebrand, auth swap, prune, extend, and a link takeover.
type byteCloudLike struct{ Base }

func (byteCloudLike) CLIName() string { return "bytecloud-supabase-cli" }
func (byteCloudLike) Brand() string   { return "ByteCloud Supabase CLI" }
func (byteCloudLike) AuthCommands() []*cobra.Command {
	return []*cobra.Command{{Use: "login", Short: "Log in to ByteCloud"}}
}
func (byteCloudLike) IncludeCommand(path ...string) bool {
	return strings.Join(path, " ") != "pages"
}
func (byteCloudLike) ExtraCommands() []*cobra.Command {
	return []*cobra.Command{{Use: "aidap", Short: "Call ByteAIDAP gateway APIs"}}
}
func (byteCloudLike) OverrideCommand(path []string, cmd *cobra.Command) {
	if strings.Join(path, " ") == "link" {
		cmd.Flags().String("project", "", "ByteAIDAP project ID to link")
	}
}

func TestApplyByteCloudLikeDistribution(t *testing.T) {
	install(t, byteCloudLike{})
	root := fakeRoot()

	Apply(root)

	assert.Equal(t, "bytecloud-supabase-cli", root.Use)
	assert.Equal(t, "ByteCloud Supabase CLI 1.2.3", root.Short)
	login := commandByName(root, "login")
	require.NotNil(t, login)
	assert.Contains(t, login.Short, "ByteCloud")
	assert.Nil(t, commandByName(root, "configure"))
	assert.Nil(t, commandByName(root, "pages"))
	assert.NotNil(t, commandByName(root, "aidap"))
	link := commandByName(root, "link")
	require.NotNil(t, link)
	assert.NotNil(t, link.Flags().Lookup("project"))
}
