// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"io"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type RuntimeContext struct {
	Input  io.Reader
	FS     afero.Fs
	Output io.Writer
	Stderr io.Writer
}

type CommandFactory func(RuntimeContext) *cobra.Command

type CommandGroup struct {
	Group     *cobra.Group
	Factories []CommandFactory
}

func defaultRuntimeContext() RuntimeContext {
	return RuntimeContext{
		Input:  os.Stdin,
		FS:     afero.NewOsFs(),
		Output: os.Stdout,
		Stderr: os.Stderr,
	}
}

func installCommandGroup(root *cobra.Command, group CommandGroup, ctx RuntimeContext) {
	if group.Group != nil {
		root.AddGroup(group.Group)
	}
	for _, factory := range group.Factories {
		if command := factory(ctx); command != nil {
			root.AddCommand(command)
		}
	}
}
