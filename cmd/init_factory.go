// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"

	"github.com/go-errors/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	_init "github.com/volcengine/byted-supabase-cli/internal/init"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"golang.org/x/term"
)

type initOptions struct {
	Interactive    bool
	CreateVscode   bool
	CreateIntellij bool
	Params         utils.InitParams
}

func newInitCommand(ctx RuntimeContext) *cobra.Command {
	var options initOptions
	cmd := &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "init",
		Short:   "Initialize a Local Project",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if !viper.IsSet("WORKDIR") {
				viper.Set("WORKDIR", ".")
			}
			return cmd.Root().PersistentPreRunE(cmd, args)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if options.Params.UseOrioleDB && !initExperimentalEnabled(cmd) {
				return errors.New("--use-orioledb requires --experimental")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fsys := ctx.FS
			interactive := options.Interactive && term.IsTerminal(int(os.Stdin.Fd()))
			if err := _init.Run(cmd.Context(), fsys, interactive, options.Params); err != nil {
				return err
			}
			if options.CreateVscode {
				if err := _init.WriteVscodeConfig(fsys); err != nil {
					return err
				}
			}
			if options.CreateIntellij {
				if err := _init.WriteIntelliJConfig(fsys); err != nil {
					return err
				}
			}
			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(ctx.Output, "Finished "+utils.Aqua("byted-supabase-cli init")+".")
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&options.Interactive, "interactive", "i", false, "Enables interactive mode to configure IDE settings.")
	flags.BoolVar(&options.Params.UseOrioleDB, "use-orioledb", false, "Use OrioleDB storage engine for Postgres.")
	flags.BoolVar(&options.Params.Overwrite, "force", false, "Overwrite existing "+utils.ConfigPath+".")
	flags.BoolVar(&options.CreateVscode, "with-vscode-workspace", false, "Generate VS Code workspace.")
	cobra.CheckErr(flags.MarkHidden("with-vscode-workspace"))
	flags.BoolVar(&options.CreateVscode, "with-vscode-settings", false, "Generate VS Code settings for Deno.")
	cobra.CheckErr(flags.MarkHidden("with-vscode-settings"))
	flags.BoolVar(&options.CreateIntellij, "with-intellij-settings", false, "Generate IntelliJ IDEA settings for Deno.")
	cobra.CheckErr(flags.MarkHidden("with-intellij-settings"))
	return cmd
}

func initExperimentalEnabled(cmd *cobra.Command) bool {
	if flag := cmd.Flag("experimental"); flag != nil {
		return flag.Value.String() == "true"
	}
	return viper.GetBool("EXPERIMENTAL")
}
