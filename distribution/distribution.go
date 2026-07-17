// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

// Package distribution is the command-surface seam for the CLI.
//
// The open-source CLI ships a Volcengine-flavoured command tree. A downstream
// distribution (for example an internal multi-cloud build) reuses that tree
// but needs to rebrand the root, replace the console auth commands with its
// own gateway login, drop commands that only make sense against Volcengine,
// mount extra commands, and decorate individual commands. Historically a
// distribution did this by mutating the assembled cobra tree from the outside,
// keyed on command names — fragile against upstream renames, and invisible to
// this repo. This package inverts control: the distribution registers a
// Distribution via Set during process startup, and the root assembly consults
// it (see Apply).
//
// Like package backend, the dependency direction is strictly one-way: this
// open-source module never imports any distribution-specific package.
// Distributions depend on this package, not the other way around.
package distribution

import (
	"strings"

	"github.com/spf13/cobra"
)

// Distribution customizes the assembled command tree. All methods are
// consulted once, at root assembly time. Embed Base and override only the
// customizations the distribution needs.
type Distribution interface {
	// CLIName returns the command name of the distribution binary; it becomes
	// the root Use, so help, usage lines, and errors print the name users
	// actually type. Empty keeps the upstream name.
	CLIName() string

	// Brand returns the human product name shown in help; the root Short
	// becomes "<Brand> <version>". Empty keeps the upstream brand.
	Brand() string

	// AuthCommands returns the distribution's replacement for the built-in
	// Volcengine console auth surface. Non-nil unmounts every command named in
	// VolcAuthCommandNames and mounts the returned commands in their place.
	// Nil keeps the built-ins.
	AuthCommands() []*cobra.Command

	// IncludeCommand reports whether the command at path — relative to the
	// root, e.g. ("pages") or ("endpoints", "eips") — should stay mounted.
	// Children of an excluded command are never asked about.
	IncludeCommand(path ...string) bool

	// ExtraCommands returns distribution-specific commands to mount on the
	// root, after the auth swap and the IncludeCommand filter have run.
	ExtraCommands() []*cobra.Command

	// OverrideCommand lets the distribution decorate a mounted command in
	// place: wrap its Run hooks, add or hide flags, rewrite help text. It is
	// called for every command left in the tree after the steps above —
	// distribution-supplied commands included — walking parents before
	// children. path is relative to the root and empty for the root itself.
	OverrideCommand(path []string, cmd *cobra.Command)
}

// Base is a no-op Distribution meant for embedding, so implementations only
// spell out what they customize.
type Base struct{}

func (Base) CLIName() string                          { return "" }
func (Base) Brand() string                            { return "" }
func (Base) AuthCommands() []*cobra.Command           { return nil }
func (Base) IncludeCommand(...string) bool            { return true }
func (Base) ExtraCommands() []*cobra.Command          { return nil }
func (Base) OverrideCommand([]string, *cobra.Command) {}

var registered Distribution

// Set installs the distribution. Call it once during process startup, before
// the root command is executed or handed out. Passing nil restores the plain
// upstream tree (for roots not yet applied).
func Set(d Distribution) { registered = d }

// Get returns the registered distribution, or nil when none is installed.
func Get() Distribution { return registered }

// VolcAuthCommandNames are the root commands forming the built-in Volcengine
// console auth surface: `login`/`logout` (console OAuth/STS) and `configure`
// (AK/SK profiles). They are unmounted wholesale when a Distribution supplies
// AuthCommands.
var VolcAuthCommandNames = []string{"login", "logout", "configure"}

// appliedAnnotation marks a root the registered distribution has been applied
// to, making Apply idempotent per root.
const appliedAnnotation = "distribution.applied"

// Apply applies the registered Distribution to root. The CLI calls it during
// root assembly (cmd.Execute / cmd.GetRootCmd); distributions normally never
// call it directly. Nil-safe and idempotent: with no distribution registered
// it is a no-op, and repeated calls on the same root do nothing.
func Apply(root *cobra.Command) {
	d := Get()
	if d == nil || root == nil {
		return
	}
	if root.Annotations[appliedAnnotation] == "true" {
		return
	}
	if root.Annotations == nil {
		root.Annotations = map[string]string{}
	}
	root.Annotations[appliedAnnotation] = "true"

	if name := d.CLIName(); name != "" {
		root.Use = name
	}
	if brand := d.Brand(); brand != "" {
		root.Short = strings.TrimSpace(brand + " " + root.Version)
	}
	if auth := d.AuthCommands(); auth != nil {
		removeByName(root, VolcAuthCommandNames)
		root.AddCommand(auth...)
	}
	filterTree(root, d, nil)
	root.AddCommand(d.ExtraCommands()...)
	d.OverrideCommand(nil, root)
	overrideTree(root, d, nil)
}

func removeByName(parent *cobra.Command, names []string) {
	drop := make(map[string]struct{}, len(names))
	for _, name := range names {
		drop[name] = struct{}{}
	}
	// Snapshot first: RemoveCommand mutates the slice Commands() returns.
	for _, cmd := range append([]*cobra.Command(nil), parent.Commands()...) {
		if _, ok := drop[cmd.Name()]; ok {
			parent.RemoveCommand(cmd)
		}
	}
}

func filterTree(parent *cobra.Command, d Distribution, path []string) {
	for _, cmd := range append([]*cobra.Command(nil), parent.Commands()...) {
		childPath := append(append([]string(nil), path...), cmd.Name())
		if !d.IncludeCommand(childPath...) {
			parent.RemoveCommand(cmd)
			continue
		}
		filterTree(cmd, d, childPath)
	}
}

func overrideTree(parent *cobra.Command, d Distribution, path []string) {
	for _, cmd := range append([]*cobra.Command(nil), parent.Commands()...) {
		childPath := append(append([]string(nil), path...), cmd.Name())
		d.OverrideCommand(childPath, cmd)
		overrideTree(cmd, d, childPath)
	}
}
