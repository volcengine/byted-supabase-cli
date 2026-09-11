package solo

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func newPagesCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{GroupID: "data-plane", Use: "pages", Short: "Manage Supabase Pages"}
	command.AddCommand(
		newPagesList(ctx), newPagesBinding(ctx), newPagesBind(ctx), newPagesCreate(ctx),
		newPagesUpload(ctx), newPagesDeploy(ctx), newPagesSync(ctx),
		newPagesUnbind(ctx), newPagesFast(ctx),
	)
	return command
}

func pagesClient() (*soloconfig.PagesClient, error) { return soloconfig.LoadPagesClient() }

func pagesRun(ctx commandtree.RuntimeContext, fn func(*soloconfig.PagesClient) (any, error)) error {
	client, err := pagesClient()
	if err != nil {
		return err
	}
	value, err := fn(client)
	if err != nil {
		return err
	}
	return writeJSON(ctx, value)
}

func newPagesList(ctx commandtree.RuntimeContext) *cobra.Command {
	var name string
	var limit, offset int
	command := &cobra.Command{Use: "list", Short: "List IGA Pages projects", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return pagesRun(ctx, func(client *soloconfig.PagesClient) (any, error) {
			return client.List(cmd.Context(), name, limit, offset)
		})
	}}
	command.Flags().StringVar(&name, "name", "", "Pages project name filter.")
	command.Flags().IntVar(&limit, "limit", 10, "Maximum number of projects.")
	command.Flags().IntVar(&offset, "offset", 0, "Number of projects to skip.")
	return command
}

func newPagesBinding(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{Use: "binding", Short: "Show the Pages binding for this Supabase branch", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return pagesRun(ctx, func(client *soloconfig.PagesClient) (any, error) { return client.Binding(cmd.Context()) })
	}}
}

func newPagesBind(ctx commandtree.RuntimeContext) *cobra.Command {
	var customPrefix, frameworkPrefix string
	command := &cobra.Command{Use: "bind <pages-project-id>", Short: "Bind a Pages project to this Supabase branch", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := confirmSoloWrite(cmd, "bind Pages project "+args[0]); err != nil {
			return err
		}
		client, err := pagesClient()
		if err != nil {
			return err
		}
		if err := client.Bind(cmd.Context(), args[0], customPrefix, frameworkPrefix); err != nil {
			return err
		}
		return writeJSON(ctx, map[string]any{"success": true, "pages_project_id": args[0]})
	}}
	command.Flags().StringVar(&customPrefix, "custom-prefix", "", "Environment variable prefix.")
	command.Flags().StringVar(&frameworkPrefix, "framework-prefix", "", "Framework client prefix.")
	return command
}

func newPagesCreate(ctx commandtree.RuntimeContext) *cobra.Command {
	var resourceID, scope, framework, rootDir, outputDir, buildCmd, installCmd, nodeVersion string
	command := &cobra.Command{Use: "create <project-name>", Short: "Create a Pages project", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := confirmSoloWrite(cmd, "create Pages project "+args[0]); err != nil {
			return err
		}
		return pagesRun(ctx, func(client *soloconfig.PagesClient) (any, error) {
			return client.Create(cmd.Context(), map[string]any{"Name": args[0], "Scope": scope, "ProjectDeployResourceId": resourceID, "ProjectParam": map[string]string{"Framework": framework, "RootDir": rootDir, "OutputDir": outputDir, "BuildCmd": buildCmd, "InstallCmd": installCmd, "NodejsVersion": nodeVersion}, "NoDeploy": resourceID == ""})
		})
	}}
	command.Flags().StringVar(&scope, "scope", "domestic", "Pages scope.")
	command.Flags().StringVar(&resourceID, "resource-id", "", "Uploaded resource id.")
	command.Flags().StringVar(&framework, "framework", "", "Framework.")
	command.Flags().StringVar(&rootDir, "root-dir", "", "Root directory.")
	command.Flags().StringVar(&outputDir, "output-dir", "", "Output directory.")
	command.Flags().StringVar(&buildCmd, "build-cmd", "", "Build command.")
	command.Flags().StringVar(&installCmd, "install-cmd", "", "Install command.")
	command.Flags().StringVar(&nodeVersion, "nodejs-version", "", "Node.js version.")
	return command
}

func newPagesUpload(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{Use: "upload <file>", Short: "Upload a Pages deployment resource", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := confirmSoloWrite(cmd, "upload Pages resource "+args[0]); err != nil {
			return err
		}
		return pagesRun(ctx, func(client *soloconfig.PagesClient) (any, error) { return client.Upload(cmd.Context(), args[0]) })
	}}
}

func newPagesDeploy(ctx commandtree.RuntimeContext) *cobra.Command {
	var resourceID string
	command := &cobra.Command{Use: "deploy <pages-project-id>", Short: "Create a Pages deployment (returns without polling)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := confirmSoloWrite(cmd, "deploy Pages project "+args[0]); err != nil {
			return err
		}
		return pagesRun(ctx, func(client *soloconfig.PagesClient) (any, error) {
			return client.Deploy(cmd.Context(), args[0], resourceID)
		})
	}}
	command.Flags().StringVar(&resourceID, "resource-id", "", "Uploaded resource id.")
	var limit, offset int
	deployList := &cobra.Command{
		Use:   "list <pages-project-id>",
		Short: "List deployments for a Pages project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePagesPagination(limit, offset); err != nil {
				return err
			}
			client, err := pagesClient()
			if err != nil {
				return err
			}
			result, err := client.DeployList(cmd.Context(), args[0], limit, offset)
			if err != nil {
				return err
			}
			renderDeployPaginationSummary(ctx.Stderr, result, limit, offset)
			return writeJSON(ctx, result)
		},
	}
	deployList.Flags().IntVar(&limit, "limit", 10, "Maximum number of deployments.")
	deployList.Flags().IntVar(&offset, "offset", 0, "Number of deployments to skip. Must be a multiple of --limit.")
	command.AddCommand(deployList)
	return command
}

func renderDeployPaginationSummary(stderr io.Writer, value any, limit, offset int) {
	if utils.OutputFormat.Value != utils.OutputPretty || stderr == nil {
		return
	}
	result, ok := value.(map[string]any)
	if !ok {
		return
	}
	total, ok := numericField(result, "Total", "total")
	if !ok {
		return
	}
	current := collectionLength(result, "Deploys", "deployments", "Deployments")
	if total <= current {
		return
	}
	remaining := total - current
	nextOffset := offset + current
	if nextOffset <= offset {
		nextOffset = offset + limit
	}
	fmt.Fprintf(stderr, "Showing %d of %d deployments (offset=%d, limit=%d).\n", current, total, offset, limit)
	fmt.Fprintf(stderr, "%d remaining; use --offset %d --limit %d to fetch the next page.\n", remaining, nextOffset, limit)
}

func numericField(result map[string]any, names ...string) (int, bool) {
	for _, name := range names {
		switch value := result[name].(type) {
		case float64:
			return int(value), true
		case int:
			return value, true
		case json.Number:
			parsed, err := value.Int64()
			if err == nil {
				return int(parsed), true
			}
		}
	}
	return 0, false
}

func collectionLength(result map[string]any, names ...string) int {
	for _, name := range names {
		switch value := result[name].(type) {
		case []any:
			return len(value)
		case []map[string]any:
			return len(value)
		}
	}
	return 0
}

func validatePagesPagination(limit, offset int) error {
	if limit < 1 {
		return fmt.Errorf("--limit must be greater than 0")
	}
	if offset < 0 {
		return fmt.Errorf("--offset must not be negative")
	}
	if offset%limit != 0 {
		return fmt.Errorf("--offset must be a multiple of --limit")
	}
	return nil
}

func newPagesEnvVars(ctx commandtree.RuntimeContext) *cobra.Command {
	_ = ctx
	return &cobra.Command{
		Use:   "env-vars",
		Short: "List Supabase environment variables available to Pages",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return fmt.Errorf("pages env-vars is not available for a directly linked branch; the Pages API exposes only env-vars sync")
		},
	}
}

func newPagesSync(ctx commandtree.RuntimeContext) *cobra.Command {
	var projectID string
	command := &cobra.Command{Use: "sync", Short: "Sync Supabase environment variables to Pages", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := pagesClient()
		if err != nil {
			return err
		}
		if projectID == "" {
			return fmt.Errorf("missing --pages-project-id")
		}
		if err := confirmSoloWrite(cmd, "sync Supabase environment variables to Pages project "+projectID); err != nil {
			return err
		}
		if err := client.Sync(cmd.Context(), projectID); err != nil {
			return err
		}
		return writeJSON(ctx, map[string]any{"success": true, "pages_project_id": projectID})
	}}
	command.Flags().StringVar(&projectID, "pages-project-id", "", "Pages project id.")
	return command
}

func newPagesUnbind(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{Use: "unbind", Short: "Unbind Pages from this Supabase branch", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if err := confirmSoloWrite(cmd, "unbind Pages from this Supabase branch"); err != nil {
			return err
		}
		client, err := pagesClient()
		if err != nil {
			return err
		}
		if err := client.Unbind(cmd.Context()); err != nil {
			return err
		}
		return writeJSON(ctx, map[string]bool{"success": true})
	}}
}

func newPagesFast(ctx commandtree.RuntimeContext) *cobra.Command {
	var file string
	command := &cobra.Command{Use: "fast", Short: "Run Pages upload/create/deploy workflow"}
	command.AddCommand(&cobra.Command{Use: "create <project-name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if file == "" {
			return fmt.Errorf("missing --file-path")
		}
		if err := confirmSoloWrite(cmd, "run Pages upload/create/deploy workflow for "+args[0]); err != nil {
			return err
		}
		client, err := pagesClient()
		if err != nil {
			return err
		}
		resource, err := client.Upload(cmd.Context(), file)
		if err != nil {
			return err
		}
		resourceMap, ok := resource.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid upload result")
		}
		resourceID, _ := resourceMap["project_deploy_resource_id"].(string)
		created, err := client.Create(cmd.Context(), map[string]any{"Name": args[0], "Scope": "domestic", "ProjectDeployResourceId": resourceID, "NoDeploy": true})
		if err != nil {
			return err
		}
		createdMap, _ := created.(map[string]any)
		projectID, _ := createdMap["PagesProjectId"].(string)
		if projectID == "" {
			projectID, _ = createdMap["ProjectID"].(string)
		}
		deploy, err := client.Deploy(cmd.Context(), projectID, resourceID)
		if err != nil {
			return err
		}
		return writeJSON(ctx, map[string]any{"project": created, "deploy": deploy})
	}})
	command.Commands()[0].Flags().StringVar(&file, "file-path", "", "Frontend archive or file.")
	return command
}
