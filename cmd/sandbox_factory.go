// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/cobra"
	sandboxclient "github.com/volcengine/byted-supabase-cli/internal/sandbox"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type sandboxWorkspaceOptions struct {
	WorkspaceID string
	BranchID    string
	AccessToken string
}

const (
	sandboxGroupLifecycle     = "sandbox-lifecycle"
	sandboxGroupExecution     = "sandbox-execution"
	sandboxGroupFiles         = "sandbox-files"
	sandboxGroupObservability = "sandbox-observability"
	sandboxGroupQuota         = "sandbox-quota"
)

func sandboxCommandGroup() CommandGroup {
	return CommandGroup{Factories: []CommandFactory{newSandboxCommand}}
}

func newSandboxCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "sandbox",
		Short:   "Manage Supabase Sandboxes (Beta)",
	}
	SetAuthPolicy(cmd, AuthPolicyVolcengine)
	cmd.AddGroup(
		&cobra.Group{ID: sandboxGroupLifecycle, Title: "Lifecycle:"},
		&cobra.Group{ID: sandboxGroupExecution, Title: "Execution:"},
		&cobra.Group{ID: sandboxGroupFiles, Title: "Files and Code:"},
		&cobra.Group{ID: sandboxGroupObservability, Title: "Observability and Networking:"},
		&cobra.Group{ID: sandboxGroupQuota, Title: "Quota:"},
	)
	cmd.AddCommand(
		newSandboxListCommand(ctx),
		newSandboxCreateCommand(ctx),
		newSandboxConnectCommand(ctx),
		newSandboxInfoCommand(ctx),
		newSandboxKillCommand(ctx),
		newSandboxLogsCommand(ctx),
		newSandboxMetricsCommand(ctx),
		newSandboxHealthCommand(ctx),
		newSandboxExecCommand(ctx),
		newSandboxTimeoutCommand(ctx),
		newSandboxFilesCommand(ctx),
		newSandboxCodeCommand(ctx),
		newSandboxPortCommand(ctx),
		newSandboxProcessCommand(ctx),
		newSandboxQuotaCommand(ctx),
	)
	return cmd
}

func newSandboxCreateCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var templateID, templateName string
	var timeout int
	var secure, allowInternet bool
	var metadata, envVars []string
	var detach bool
	cmd := &cobra.Command{
		GroupID: sandboxGroupLifecycle,
		Use:     "create",
		Short:   "Create a Sandbox from a template",
		Args:    cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(templateID) == "" && strings.TrimSpace(templateName) == "" {
				return errors.New("one of --template-id or --template-name is required")
			}
			if strings.TrimSpace(templateID) != "" && strings.TrimSpace(templateName) != "" {
				return errors.New("--template-id and --template-name cannot be combined")
			}
			if timeout < 0 {
				return errors.New("--timeout must be non-negative")
			}
			if _, err := parseSandboxKeyValues(metadata, "--metadata"); err != nil {
				return err
			}
			if _, err := parseSandboxKeyValues(envVars, "--env"); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			metadataValues, _ := parseSandboxKeyValues(metadata, "--metadata")
			envValues, _ := parseSandboxKeyValues(envVars, "--env")
			resolvedTemplateName, err := resolveSandboxTemplateName(cmd, options, templateID, templateName)
			if err != nil {
				return err
			}
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			result, err := client.Create(cmd.Context(), sandboxclient.CreateRequest{
				TemplateID: resolvedTemplateName, Timeout: timeout,
				Secure: &secure, AllowInternetAccess: &allowInternet,
				Metadata: metadataValues, EnvVars: envValues,
			})
			if err != nil {
				return err
			}
			if !detach && utils.OutputFormat.Value == utils.OutputPretty {
				fmt.Fprintln(ctx.Output, "Sandbox created. Use the returned sandbox ID with `sandbox connect` or `sandbox exec`.")
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&templateID, "template-id", "", "Sandbox template ID.")
	cmd.Flags().StringVar(&templateName, "template-name", "", "Sandbox template name.")
	cmd.Flags().IntVar(&timeout, "timeout", 900, "Sandbox lifetime in seconds.")
	cmd.Flags().BoolVar(&secure, "secure", true, "Secure system communication with the Sandbox.")
	cmd.Flags().BoolVar(&allowInternet, "allow-internet", true, "Allow the Sandbox to access the internet.")
	cmd.Flags().StringArrayVar(&metadata, "metadata", nil, "Sandbox metadata entry in key=value format; repeatable.")
	cmd.Flags().StringArrayVar(&envVars, "env", nil, "Sandbox environment entry in key=value format; repeatable.")
	cmd.Flags().BoolVar(&detach, "detach", false, "Create the Sandbox without connecting to it.")
	return cmd
}

func newSandboxConnectCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID string
	var timeout int
	cmd := &cobra.Command{
		GroupID: sandboxGroupExecution,
		Use:     "connect",
		Short:   "Connect to a running or paused Sandbox",
		Args:    cobra.ArbitraryArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(sandboxID) == "" {
				return errors.New("missing required flag: --sandbox-id")
			}
			if timeout < 0 {
				return errors.New("--timeout must be non-negative")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			body, err := json.Marshal(map[string]int{"timeout": timeout})
			if err != nil {
				return err
			}
			result, err := client.Request(
				cmd.Context(),
				http.MethodPost,
				"/sandbox/v1/sandboxes/"+url.PathEscape(sandboxID)+"/connect",
				sandboxID,
				body,
			)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	cmd.Flags().IntVar(&timeout, "timeout", 900, "Sandbox lifetime in seconds.")
	return cmd
}

func newSandboxInfoCommand(ctx RuntimeContext) *cobra.Command {
	return newSandboxDataPlaneActionCommand(ctx, "info", "Show Sandbox information", http.MethodGet, "", sandboxGroupLifecycle)
}

func newSandboxKillCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxIDs []string
	var killAll bool
	var states []string
	var metadata []string
	cmd := &cobra.Command{
		GroupID: sandboxGroupLifecycle,
		Use:     "kill [sandbox-id...]",
		Short:   "Stop one or more Sandboxes",
		Args:    cobra.ArbitraryArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sandboxIDs = append(sandboxIDs, args...)
			if !killAll && len(sandboxIDs) == 0 {
				return errors.New("provide sandbox IDs or --all")
			}
			if killAll && len(sandboxIDs) > 0 {
				return errors.New("sandbox IDs cannot be combined with --all")
			}
			if !killAll && (len(states) > 0 || len(metadata) > 0) {
				return errors.New("--state and --metadata require --all")
			}
			if _, err := parseSandboxKeyValues(metadata, "--metadata"); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireSandboxConfirmation(cmd); err != nil {
				return err
			}
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			if killAll {
				metadataValues, _ := parseSandboxKeyValues(metadata, "--metadata")
				listed, err := client.ListSandboxes(cmd.Context(), sandboxclient.SandboxListOptions{
					Metadata: metadataValues,
					States:   states,
					Limit:    100,
				})
				if err != nil {
					return err
				}
				var rows []struct {
					SandboxID string `json:"sandboxID"`
				}
				if err := json.Unmarshal(listed, &rows); err != nil {
					return errors.Errorf("decode Sandbox list for --all: %w", err)
				}
				for _, row := range rows {
					if strings.TrimSpace(row.SandboxID) != "" {
						sandboxIDs = append(sandboxIDs, row.SandboxID)
					}
				}
			}
			items := make([]map[string]any, 0, len(sandboxIDs))
			failed := 0
			for _, sandboxID := range sandboxIDs {
				result, killErr := client.Request(cmd.Context(), http.MethodDelete, "/sandbox/v1/sandboxes/"+url.PathEscape(sandboxID), sandboxID, nil)
				item := map[string]any{"sandbox_id": sandboxID, "success": killErr == nil}
				if killErr != nil {
					failed++
					item["error"] = killErr.Error()
				} else if len(bytes.TrimSpace(result)) > 0 {
					item["result"] = json.RawMessage(result)
				}
				items = append(items, item)
			}
			summary := map[string]any{
				"requested": len(sandboxIDs),
				"killed":    len(sandboxIDs) - failed,
				"failed":    failed,
				"items":     items,
			}
			if err := utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, summary); err != nil {
				return err
			}
			if failed > 0 {
				return errors.Errorf("%d Sandbox kill operations failed", failed)
			}
			return nil
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringSliceVar(&sandboxIDs, "sandbox-id", nil, "Sandbox ID; repeatable. Positional IDs are also accepted.")
	cmd.Flags().BoolVar(&killAll, "all", false, "Stop all Sandboxes matching the filters in the workspace and branch.")
	cmd.Flags().StringSliceVar(&states, "state", nil, "Filter states for --all, such as running,paused.")
	cmd.Flags().StringArrayVar(&metadata, "metadata", nil, "Filter metadata for --all in key=value format; repeatable.")
	return cmd
}

func newSandboxLogsCommand(ctx RuntimeContext) *cobra.Command {
	return newSandboxDataPlaneActionCommand(ctx, "logs", "Show Sandbox logs", http.MethodGet, "/v2/sandboxes/{id}/logs", sandboxGroupObservability)
}

func newSandboxMetricsCommand(ctx RuntimeContext) *cobra.Command {
	return newSandboxDataPlaneActionCommand(ctx, "metrics", "Show Sandbox metrics", http.MethodGet, "/metrics", sandboxGroupObservability)
}

func newSandboxHealthCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID string
	cmd := &cobra.Command{
		GroupID: sandboxGroupObservability,
		Use:     "health",
		Short:   "Check Sandbox health",
		Args:    cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(sandboxID) == "" {
				return errors.New("missing required flag: --sandbox-id")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			result, err := client.Health(cmd.Context(), sandboxID)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	return cmd
}

func newSandboxTimeoutCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID string
	var timeout int
	cmd := &cobra.Command{GroupID: sandboxGroupLifecycle, Use: "timeout", Short: "Set Sandbox timeout", Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(sandboxID) == "" {
				return errors.New("missing required flag: --sandbox-id")
			}
			if timeout < 0 {
				return errors.New("--timeout must be non-negative")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			body, _ := json.Marshal(map[string]int{"timeout": timeout})
			result, err := client.Request(cmd.Context(), http.MethodPost, "/sandbox/v1/sandboxes/"+url.PathEscape(sandboxID)+"/timeout", sandboxID, body)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		}}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	cmd.Flags().IntVar(&timeout, "timeout", 900, "Sandbox lifetime in seconds.")
	return cmd
}

func newSandboxExecCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID, command, cwd, user string
	var commandArgs []string
	var envVars []string
	var background bool
	cmd := &cobra.Command{
		GroupID: sandboxGroupExecution,
		Use:     "exec",
		Short:   "Execute a command in a Sandbox",
		Args:    cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(sandboxID) == "" {
				return errors.New("missing required flag: --sandbox-id")
			}
			if strings.TrimSpace(command) == "" {
				return errors.New("missing required flag: --command")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			envs, err := parseSandboxKeyValues(envVars, "--env")
			if err != nil {
				return err
			}
			result, err := client.StartProcessWithOptions(cmd.Context(), sandboxID, command, commandArgs, cwd, user, envs, background)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	cmd.Flags().StringVar(&command, "command", "", "Executable command.")
	cmd.Flags().StringSliceVar(&commandArgs, "arg", nil, "Command argument; repeat the flag for multiple arguments.")
	cmd.Flags().StringVar(&cwd, "cwd", "", "Working directory inside the Sandbox.")
	cmd.Flags().StringVar(&user, "user", "user", "User to run the command as.")
	cmd.Flags().StringArrayVar(&envVars, "env", nil, "Environment variable in key=value format; repeatable.")
	cmd.Flags().BoolVar(&background, "background", false, "Start the command in the background and return immediately.")
	return cmd
}

func newSandboxFilesCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{GroupID: sandboxGroupFiles, Use: "files", Short: "Manage files in a Sandbox"}
	cmd.AddCommand(
		newSandboxFilesListCommand(ctx),
		newSandboxFilesReadCommand(ctx),
		newSandboxFilesWriteCommand(ctx),
		newSandboxFilesExistsCommand(ctx),
		newSandboxFilesMkdirCommand(ctx),
		newSandboxFilesInfoCommand(ctx),
		newSandboxFilesRemoveCommand(ctx),
		newSandboxFilesRenameCommand(ctx),
	)
	return cmd
}

func newSandboxFilesListCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID, path string
	var depth int
	cmd := &cobra.Command{Use: "list", Short: "List files in a Sandbox", Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if depth < 1 {
				return errors.New("--depth must be at least 1")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			result, err := client.ListFiles(cmd.Context(), sandboxID, path, depth)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		}}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	cmd.Flags().StringVar(&path, "path", "", "Directory path relative to the Sandbox user home.")
	cmd.Flags().IntVar(&depth, "depth", 1, "Directory traversal depth.")
	return cmd
}

func newSandboxFilesReadCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID, path string
	cmd := &cobra.Command{Use: "read", Short: "Read a file from a Sandbox", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			data, err := client.ReadFile(cmd.Context(), sandboxID, path)
			if err != nil {
				return err
			}
			_, err = ctx.Output.Write(data)
			return err
		}}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	cmd.Flags().StringVar(&path, "path", "", "File path.")
	cmd.MarkFlagRequired("path")
	return cmd
}

func newSandboxFilesWriteCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID, path, file string
	var fromStdin bool
	cmd := &cobra.Command{Use: "write", Short: "Write a file to a Sandbox", Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(path) == "" {
				return errors.New("missing required flag: --path")
			}
			if file == "" && !fromStdin {
				return errors.New("provide --file or --stdin")
			}
			if file != "" && fromStdin {
				return errors.New("--file and --stdin cannot be combined")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var data []byte
			var err error
			if fromStdin {
				data, err = io.ReadAll(os.Stdin)
			} else {
				data, err = os.ReadFile(file)
			}
			if err != nil {
				return err
			}
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			result, err := client.WriteFile(cmd.Context(), sandboxID, path, data)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		}}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	cmd.Flags().StringVar(&path, "path", "", "File path.")
	cmd.Flags().StringVar(&file, "file", "", "Local file to upload.")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read file content from stdin.")
	return cmd
}

func newSandboxFilesExistsCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID, path string
	cmd := &cobra.Command{
		Use:   "exists",
		Short: "Check whether a file or directory exists",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			exists, err := client.FileExists(cmd.Context(), sandboxID, path)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, map[string]any{
				"sandbox_id": sandboxID,
				"path":       path,
				"exists":     exists,
			})
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	_ = cmd.MarkFlagRequired("path")
	cmd.Flags().StringVar(&path, "path", "", "File or directory path.")
	return cmd
}

func newSandboxFilesMkdirCommand(ctx RuntimeContext) *cobra.Command {
	return newSandboxFilesystemPathCommand(ctx, "mkdir", "Create a directory in a Sandbox", func(client sandboxclient.Client, cmd *cobra.Command, sandboxID, path string) (json.RawMessage, error) {
		return client.MakeDir(cmd.Context(), sandboxID, path)
	})
}

func newSandboxFilesInfoCommand(ctx RuntimeContext) *cobra.Command {
	return newSandboxFilesystemPathCommand(ctx, "info", "Show file information", func(client sandboxclient.Client, cmd *cobra.Command, sandboxID, path string) (json.RawMessage, error) {
		return client.FileInfo(cmd.Context(), sandboxID, path)
	})
}

func newSandboxFilesRemoveCommand(ctx RuntimeContext) *cobra.Command {
	return newSandboxFilesystemPathCommand(ctx, "remove", "Remove a file or directory", func(client sandboxclient.Client, cmd *cobra.Command, sandboxID, path string) (json.RawMessage, error) {
		if err := requireSandboxConfirmation(cmd); err != nil {
			return nil, err
		}
		return client.RemoveFile(cmd.Context(), sandboxID, path)
	})
}

func newSandboxFilesRenameCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID, source, destination string
	cmd := &cobra.Command{
		Use:   "rename",
		Short: "Rename a file or directory",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(source) == "" || strings.TrimSpace(destination) == "" {
				return errors.New("--source and --destination are required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			result, err := client.RenameFile(cmd.Context(), sandboxID, source, destination)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	cmd.Flags().StringVar(&source, "source", "", "Existing file or directory path.")
	cmd.Flags().StringVar(&destination, "destination", "", "New file or directory path.")
	return cmd
}

type sandboxFilesystemPathAction func(sandboxclient.Client, *cobra.Command, string, string) (json.RawMessage, error)

func newSandboxFilesystemPathCommand(ctx RuntimeContext, use, short string, action sandboxFilesystemPathAction) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID, path string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(path) == "" {
				return errors.New("missing required flag: --path")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			result, err := action(client, cmd, sandboxID, path)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	cmd.Flags().StringVar(&path, "path", "", "File or directory path.")
	return cmd
}

func newSandboxCodeCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID, code, file string
	cmd := &cobra.Command{GroupID: sandboxGroupFiles, Use: "code", Short: "Run code in a code-interpreter Sandbox"}
	run := &cobra.Command{
		Use: "run", Short: "Execute Python code", Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(code) == "" && file == "" {
				return errors.New("provide --code or --file")
			}
			if code != "" && file != "" {
				return errors.New("--code and --file cannot be combined")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				code = string(data)
			}
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			result, err := client.ExecuteCode(cmd.Context(), sandboxID, code)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(run, &options)
	run.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	run.Flags().StringVar(&code, "code", "", "Python source code.")
	run.Flags().StringVar(&file, "file", "", "Local Python file.")
	cmd.AddCommand(run)
	return cmd
}

func newSandboxPortCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID string
	var port int
	cmd := &cobra.Command{GroupID: sandboxGroupObservability, Use: "port", Short: "Manage Sandbox port previews"}
	urlCommand := &cobra.Command{
		Use: "url", Short: "Print the Sandbox port preview URL", Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if port < 1 || port > 65535 {
				return errors.New("--port must be between 1 and 65535")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, map[string]string{
				"sandbox_id": sandboxID,
				"url":        client.PreviewURL(sandboxID, port),
			})
		},
	}
	addSandboxWorkspaceFlags(urlCommand, &options)
	urlCommand.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	urlCommand.Flags().IntVar(&port, "port", 8080, "Port exposed by the Sandbox.")
	cmd.AddCommand(urlCommand)
	return cmd
}

func newSandboxJSONActionCommand(ctx RuntimeContext, use, short, method, suffix string, mutating bool, groupID string) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID string
	cmd := &cobra.Command{GroupID: groupID, Use: use, Short: short, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(sandboxID) == "" {
				return errors.New("missing required flag: --sandbox-id")
			}
			if mutating && method == http.MethodDelete {
				if err := requireSandboxConfirmation(cmd); err != nil {
					return err
				}
			}
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			path := "/sandbox/v1/sandboxes/" + url.PathEscape(sandboxID) + suffix
			result, err := client.Request(cmd.Context(), method, path, sandboxID, nil)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		}}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	return cmd
}

func newSandboxDataPlaneActionCommand(ctx RuntimeContext, use, short, method, suffix, groupID string) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID string
	cmd := &cobra.Command{
		GroupID: groupID,
		Use:     use,
		Short:   short,
		Args:    cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(sandboxID) == "" {
				return errors.New("missing required flag: --sandbox-id")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			path := "/sandbox/v1/sandboxes/" + url.PathEscape(sandboxID) + strings.ReplaceAll(suffix, "{id}", url.PathEscape(sandboxID))
			if strings.HasPrefix(suffix, "/v2/") {
				path = "/sandbox/v1" + strings.ReplaceAll(suffix, "{id}", url.PathEscape(sandboxID))
			}
			result, err := client.Request(cmd.Context(), method, path, sandboxID, nil)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	return cmd
}

func newSandboxProcessCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{GroupID: sandboxGroupExecution, Use: "process", Short: "Manage processes in a Sandbox"}
	cmd.AddCommand(newSandboxProcessStartCommand(ctx))
	return cmd
}

func newSandboxProcessStartCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var sandboxID, command, cwd, user string
	var commandArgs []string
	var envVars []string
	var background bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a process in a Sandbox",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(sandboxID) == "" {
				return errors.New("missing required flag: --sandbox-id")
			}
			if strings.TrimSpace(command) == "" {
				return errors.New("missing required flag: --command")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			envs, err := parseSandboxKeyValues(envVars, "--env")
			if err != nil {
				return err
			}
			result, err := client.StartProcessWithOptions(cmd.Context(), sandboxID, command, commandArgs, cwd, user, envs, background)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&sandboxID, "sandbox-id", "", "Sandbox ID.")
	cmd.Flags().StringVar(&command, "command", "", "Executable command.")
	cmd.Flags().StringSliceVar(&commandArgs, "arg", nil, "Command argument; repeat the flag for multiple arguments.")
	cmd.Flags().StringVar(&cwd, "cwd", "", "Working directory inside the Sandbox.")
	cmd.Flags().StringVar(&user, "user", "user", "User to run the command as.")
	cmd.Flags().StringArrayVar(&envVars, "env", nil, "Environment variable in key=value format; repeatable.")
	cmd.Flags().BoolVar(&background, "background", false, "Start the command in the background and return immediately.")
	return cmd
}

func newSandboxListCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	cmd := &cobra.Command{
		GroupID: sandboxGroupLifecycle,
		Use:     "list",
		Short:   "List Sandboxes",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := sandboxDataPlaneClient(cmd, options)
			if err != nil {
				return err
			}
			result, err := client.Request(cmd.Context(), http.MethodGet, "/sandbox/v1/v2/sandboxes", "", nil)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	return cmd
}

func newSandboxQuotaCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: sandboxGroupQuota,
		Use:     "quota",
		Short:   "View Sandbox permissions and quotas",
	}
	cmd.AddCommand(
		newSandboxGlobalQuotaCommand(ctx),
		newSandboxGlobalQuotaSetCommand(ctx),
		newSandboxUserSummariesCommand(ctx),
	)
	return cmd
}

func newSandboxGlobalQuotaCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show the default daily Sandbox quota",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := sandboxClient(cmd)
			if err != nil {
				return err
			}
			result, err := client.DescribeBaasSandboxGlobalQuota(cmd.Context(), strings.TrimSpace(options.WorkspaceID), strings.TrimSpace(options.BranchID))
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	return cmd
}

func newSandboxGlobalQuotaSetCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var dailyLimit int64
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set the default daily Sandbox quota",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("daily-limit") {
				return errors.New("missing required flag: --daily-limit")
			}
			if dailyLimit < 0 {
				return errors.New("--daily-limit must be greater than or equal to 0")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := volcengineConfigFromAuth(cmd)
			if err != nil {
				return err
			}
			client := volcengine.NewWriteClient(config)
			if err := client.ModifyBaasSandboxGlobalQuota(cmd.Context(), strings.TrimSpace(options.WorkspaceID), strings.TrimSpace(options.BranchID), dailyLimit); err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, map[string]interface{}{
				"success":             true,
				"daily_sandbox_limit": dailyLimit,
				"workspace_id":        strings.TrimSpace(options.WorkspaceID),
			})
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().Int64Var(&dailyLimit, "daily-limit", 0, "Default daily Sandbox quota per user.")
	return cmd
}

func newSandboxUserSummariesCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	var userID, userName, email string
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "users",
		Short: "List Sandbox user permissions, quotas, and usage",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if limit < 0 || offset < 0 {
				return errors.New("--limit and --offset must be non-negative")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := sandboxClient(cmd)
			if err != nil {
				return err
			}
			result, err := client.DescribeBaasSandboxUserSummaries(cmd.Context(), volcengine.SandboxUserSummaryParams{
				WorkspaceID: strings.TrimSpace(options.WorkspaceID),
				BranchID:    strings.TrimSpace(options.BranchID),
				UserID:      strings.TrimSpace(userID),
				UserName:    strings.TrimSpace(userName),
				Email:       strings.TrimSpace(email),
				Limit:       limit,
				Offset:      offset,
			})
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&userID, "user-id", "", "Filter by user ID.")
	cmd.Flags().StringVar(&userName, "user-name", "", "Filter by user name.")
	cmd.Flags().StringVar(&email, "email", "", "Filter by email.")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of users to return.")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of users to skip.")
	return cmd
}

func addSandboxWorkspaceFlags(cmd *cobra.Command, options *sandboxWorkspaceOptions) {
	cmd.Flags().StringVar(&options.WorkspaceID, "project-ref", "", "Project ref alias of --workspace-id.")
	cmd.Flags().StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Supabase project.")
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Supabase project.")
	cmd.Flags().StringVar(&options.AccessToken, "sandbox-access-token", "", "Sandbox envd access token; defaults to SANDBOX_ACCESS_TOKEN.")
}

func sandboxClient(cmd *cobra.Command) (*volcengine.Client, error) {
	return volcengineClientFromAuth(cmd)
}

func sandboxDataPlaneClient(cmd *cobra.Command, options sandboxWorkspaceOptions) (sandboxclient.Client, volcengine.PgMetaAccess, error) {
	client, err := sandboxClient(cmd)
	if err != nil {
		return sandboxclient.Client{}, volcengine.PgMetaAccess{}, err
	}
	access, err := client.ResolvePgMetaAccess(cmd.Context(), strings.TrimSpace(options.WorkspaceID), strings.TrimSpace(options.BranchID))
	if err != nil {
		return sandboxclient.Client{}, volcengine.PgMetaAccess{}, err
	}
	accessToken := strings.TrimSpace(options.AccessToken)
	if accessToken == "" {
		accessToken = strings.TrimSpace(os.Getenv("SANDBOX_ACCESS_TOKEN"))
	}
	return sandboxclient.Client{BaseURL: access.BaseURL, Token: access.ServiceRoleKey, AccessToken: accessToken}, access, nil
}

func requireSandboxConfirmation(cmd *cobra.Command) error {
	confirmed, err := utils.NewConsole().PromptYesNo(cmd.Context(), "Confirm Sandbox operation?", false)
	if err != nil {
		return err
	}
	if !confirmed {
		return errors.New("operation cancelled")
	}
	return nil
}

func parseSandboxKeyValues(values []string, flagName string) (map[string]string, error) {
	result := make(map[string]string, len(values))
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, errors.Errorf("%s values must use key=value format", flagName)
		}
		result[key] = val
	}
	return result, nil
}

func resolveSandboxTemplateName(cmd *cobra.Command, options sandboxWorkspaceOptions, templateID, templateName string) (string, error) {
	templateID = strings.TrimSpace(templateID)
	templateName = strings.TrimSpace(templateName)
	if templateID == "" {
		return templateName, nil
	}
	client, err := sandboxClient(cmd)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeBaasSandboxTemplates(cmd.Context(), strings.TrimSpace(options.WorkspaceID), strings.TrimSpace(options.BranchID))
	if err != nil {
		return "", err
	}
	rawResult, ok := result["Result"].(map[string]interface{})
	if !ok {
		return "", errors.Errorf("template ID %q was not found", templateID)
	}
	rawTemplates, ok := rawResult["Templates"].([]interface{})
	if !ok {
		return "", errors.Errorf("template ID %q was not found", templateID)
	}
	for _, rawTemplate := range rawTemplates {
		template, ok := rawTemplate.(map[string]interface{})
		if !ok {
			continue
		}
		availableTemplateID, _ := template["TemplateId"].(string)
		availableTemplateName, _ := template["TemplateName"].(string)
		if strings.EqualFold(templateID, strings.TrimSpace(availableTemplateID)) {
			if strings.TrimSpace(availableTemplateName) == "" {
				return "", errors.Errorf("template ID %q has no usable template name", templateID)
			}
			return strings.TrimSpace(availableTemplateName), nil
		}
	}
	return "", errors.Errorf("template ID %q was not found", templateID)
}
