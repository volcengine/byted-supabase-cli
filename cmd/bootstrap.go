// Copyright (c) 2021 Supabase, Inc. and contributors
// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT
//
// This file has been modified by ByteDance Ltd. and/or its affiliates.
//
// Original file was released under MIT License, with the full license text
// available at https://github.com/supabase/cli/blob/main/LICENSE.
//
// This modified file is released under the same license.

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/volcengine/byted-supabase-cli/internal/bootstrap"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

var (
	starter = bootstrap.StarterTemplate{
		Name:        "scratch",
		Description: "An empty project from scratch.",
		Start:       "byted-supabase-cli start",
	}

	bootstrapCmd = &cobra.Command{
		GroupID: groupQuickStart,
		Use:     "bootstrap [template]",
		Short:   "Bootstrap a Supabase project from a starter template",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !viper.IsSet("WORKDIR") {
				title := fmt.Sprintf("Enter a directory to bootstrap your project (or leave blank to use %s): ", utils.Bold(utils.CurrentDirAbs))
				if workdir, err := utils.NewConsole().PromptText(ctx, title); err != nil {
					return err
				} else {
					viper.Set("WORKDIR", workdir)
				}
			}
			client := utils.GetGitHubClient(ctx)
			templates, err := bootstrap.ListSamples(ctx, client)
			if err != nil {
				return err
			}
			if len(args) > 0 {
				name := args[0]
				for _, t := range templates {
					if strings.EqualFold(t.Name, name) {
						starter = t
						break
					}
				}
				if !strings.EqualFold(starter.Name, name) {
					return errors.New("Invalid template: " + name)
				}
			} else {
				if err := promptStarterTemplate(ctx, templates); err != nil {
					return err
				}
			}
			return bootstrap.Run(ctx, starter, afero.NewOsFs())
		},
	}
)

func init() {
	bootstrapFlags := bootstrapCmd.Flags()
	bootstrapFlags.StringVarP(&dbPassword, "password", "p", "", "Password to your remote Postgres database.")
	cobra.CheckErr(viper.BindPFlag("DB_PASSWORD", bootstrapFlags.Lookup("password")))
	// Volcengine CLI does not support local-stack starter bootstrap in the first phase.
	// Keep the command definition for future local developer workflow support.
	// rootCmd.AddCommand(bootstrapCmd)
}

func promptStarterTemplate(ctx context.Context, templates []bootstrap.StarterTemplate) error {
	items := make([]utils.PromptItem, len(templates))
	for i, t := range templates {
		items[i] = utils.PromptItem{
			Index:   i,
			Summary: t.Name,
			Details: t.Description,
		}
	}
	items = append(items, utils.PromptItem{
		Index:   len(items),
		Summary: starter.Name,
		Details: starter.Description,
	})
	title := "Which starter template do you want to use?"
	choice, err := utils.PromptChoice(ctx, title, items)
	if err != nil {
		return err
	}
	if choice.Index < len(templates) {
		starter = templates[choice.Index]
	}
	return nil
}
