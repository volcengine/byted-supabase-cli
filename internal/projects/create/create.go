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

package create

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

type RunVolcengineParams struct {
	WorkspaceName         string
	ProjectName           string
	IsAgentPlan           *bool
	AgentPlanSeatID       string
	Region                string
	SuspendTimeoutSeconds *int
}

func Run(ctx context.Context, params api.V1CreateProjectBody, fsys afero.Fs) error {
	if err := promptMissingParams(ctx, &params); err != nil {
		return err
	}

	resp, err := utils.GetSupabase().V1CreateAProjectWithResponse(ctx, params)
	if err != nil {
		return errors.Errorf("failed to create project: %w", err)
	}
	if resp.JSON201 == nil {
		return errors.New("Unexpected error creating project: " + string(resp.Body))
	}

	flags.ProjectRef = resp.JSON201.Id
	viper.Set("DB_PASSWORD", params.DbPass)

	projectUrl := fmt.Sprintf("%s/project/%s", utils.GetSupabaseDashboardURL(), resp.JSON201.Id)
	fmt.Fprintf(os.Stderr, "Created a new project at %s\n", utils.Bold(projectUrl))
	if utils.OutputFormat.Value == utils.OutputPretty {
		table := `|ORG ID|REFERENCE ID|NAME|REGION|CREATED AT (UTC)|
|-|-|-|-|-|
`
		table += fmt.Sprintf(
			"|`%s`|`%s`|`%s`|`%s`|`%s`|\n",
			resp.JSON201.OrganizationSlug,
			resp.JSON201.Id,
			strings.ReplaceAll(resp.JSON201.Name, "|", "\\|"),
			utils.FormatRegion(resp.JSON201.Region),
			utils.FormatTimestamp(resp.JSON201.CreatedAt),
		)
		return utils.RenderTable(table)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, resp.JSON201)
}

func RunVolcengine(ctx context.Context, params RunVolcengineParams) error {
	if err := volcengine.RequireAccessKeysEnv(); err != nil {
		return err
	}
	if err := promptMissingVolcengineParams(ctx, &params); err != nil {
		return err
	}
	params.WorkspaceName = strings.TrimSpace(params.WorkspaceName)
	params.ProjectName = strings.TrimSpace(params.ProjectName)
	params.Region = strings.TrimSpace(params.Region)
	if params.Region == "" {
		return errors.New("region cannot be empty")
	}
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	cfg := cfgSet.ConfigForRegion(params.Region)
	// Gate creation on the AIDAP service-linked role: without it the account cannot
	// use AIDAP, so block with the authorization guidance instead of creating.
	if err := volcengine.NewClient(cfg).CheckAIDAPServiceLinkedRole(); err != nil {
		return err
	}
	createParams := volcengine.CreateWorkspaceParams{
		WorkspaceName:         params.WorkspaceName,
		ProjectName:           params.ProjectName,
		IsAgentPlan:           params.IsAgentPlan,
		AgentPlanSeatID:       strings.TrimSpace(params.AgentPlanSeatID),
		SuspendTimeoutSeconds: params.SuspendTimeoutSeconds,
	}
	if err := applyAgentPlanSuspendDefault(volcengine.NewClient(cfg), &createParams); err != nil {
		return err
	}
	result, err := volcengine.NewWriteClient(cfg).CreateSupabaseWorkspace(ctx, createParams)
	if err != nil {
		return err
	}
	workspace := result.Workspace
	if workspace.WorkspaceID == "" {
		workspace.WorkspaceID = result.WorkspaceID
	}
	if workspace.RegionID == "" {
		workspace.RegionID = params.Region
	}
	if workspace.WorkspaceName == "" {
		workspace.WorkspaceName = params.WorkspaceName
	}

	fmt.Fprintf(os.Stderr, "Created a new project: %s\n", utils.Bold(workspace.WorkspaceID))
	if utils.OutputFormat.Value == utils.OutputPretty {
		table := `REFERENCE ID|NAME|PROJECT NAME|REGION|STATUS|ENGINE|ENGINE VERSION|CREATED AT (UTC)
|-|-|-|-|-|-|-|-|
`
		table += fmt.Sprintf(
			"|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|\n",
			workspace.WorkspaceID,
			strings.ReplaceAll(workspace.WorkspaceName, "|", "\\|"),
			strings.ReplaceAll(workspace.ProjectName, "|", "\\|"),
			workspace.RegionID,
			workspace.WorkspaceStatus,
			workspace.EngineType,
			workspace.EngineVersion,
			utils.FormatTimestamp(workspace.CreateTime),
		)
		return utils.RenderTable(table)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, workspace)
}

// applyAgentPlanSuspendDefault defaults the auto-suspend idle timeout to 60 minutes for a
// small Agent Plan workspace created without an explicit --suspend-timeout-seconds. It
// resolves the Agent Plan edition (honouring profile defaults), then looks up the plan
// size — GetPersonalPlan for the personal edition, or GetSeatAFPUsage for the enterprise
// edition (by seat ID). Only small plans (Small/Medium) receive the default; larger plans
// (Large/Max) are left to the backend default. The lookup failing never blocks creation:
// the backend default applies and the user can still pass --suspend-timeout-seconds.
func applyAgentPlanSuspendDefault(client *volcengine.Client, params *volcengine.CreateWorkspaceParams) error {
	// The gates are checked in order so the cheap, local conditions short-circuit before
	// AgentPlanType (a network call). Any gate that fails leaves the backend default in
	// place rather than blocking creation.
	if userSetTimeout := params.SuspendTimeoutSeconds != nil; userSetTimeout {
		return nil
	}
	resolved, err := volcengine.ResolveCreateWorkspaceAgentPlan(*params)
	if err != nil {
		return err
	}
	// Persist the resolved edition for every path (even non-Agent-Plan) so the subsequent
	// create reuses the same decision (CreateSupabaseWorkspace re-resolves idempotently).
	*params = resolved
	if isAgentPlan := volcengine.IsAgentPlan(resolved); !isAgentPlan {
		return nil
	}
	planType, err := client.AgentPlanType(resolved)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: skipping auto-suspend default; failed to look up Agent Plan size: %v\n", err)
		return nil
	}
	if isSmallPlan := volcengine.IsSmallAgentPlanType(planType); !isSmallPlan {
		return nil
	}
	timeout := volcengine.SmallAgentPlanSuspendTimeoutSeconds
	params.SuspendTimeoutSeconds = &timeout
	fmt.Fprintln(os.Stderr, printKeyValue("Auto-suspend timeout", fmt.Sprintf("%ds (60 min) [Agent Plan %s default]", timeout, planType)))
	return nil
}

func printKeyValue(key, value string) string {
	// Always keep at least one space so long keys (len >= 20) don't collide with the
	// value, and never pass a negative count to strings.Repeat (which would panic).
	indent := 20 - len(key)
	if indent < 1 {
		indent = 1
	}
	spaces := strings.Repeat(" ", indent)
	return key + ":" + spaces + value
}

func promptMissingVolcengineParams(ctx context.Context, params *RunVolcengineParams) error {
	var err error
	params.WorkspaceName = strings.TrimSpace(params.WorkspaceName)
	if len(params.WorkspaceName) == 0 {
		if params.WorkspaceName, err = promptProjectName(ctx); err != nil {
			return err
		}
		if params.WorkspaceName == "" {
			fmt.Fprintln(os.Stderr, "Project name is empty. Volcengine will generate one automatically.")
		}
	} else {
		fmt.Fprintln(os.Stderr, printKeyValue("Creating project", params.WorkspaceName))
	}
	if len(params.Region) == 0 {
		if params.Region, err = promptVolcengineRegion(ctx); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stderr, printKeyValue("Selected region", params.Region))
	return nil
}

func promptMissingParams(ctx context.Context, body *api.V1CreateProjectBody) error {
	var err error
	if len(body.Name) == 0 {
		if body.Name, err = promptProjectName(ctx); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(os.Stderr, printKeyValue("Creating project", body.Name))
	}
	if len(body.OrganizationSlug) == 0 {
		if body.OrganizationSlug, err = promptOrgId(ctx); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, printKeyValue("Selected org-id", body.OrganizationSlug))
	}
	if body.Region == nil || len(*body.Region) == 0 {
		region, err := promptProjectRegion(ctx)
		if err != nil {
			return err
		}
		body.Region = &region
		fmt.Fprintln(os.Stderr, printKeyValue("Selected region", string(region)))
	}
	if len(body.DbPass) == 0 {
		body.DbPass = flags.PromptPassword(os.Stdin)
	}
	return nil
}

func promptVolcengineRegion(ctx context.Context) (string, error) {
	title := "Which region do you want to host the project in?"
	regions := volcengine.ConfiguredRegions()
	if len(regions) == 0 {
		return volcengine.DefaultRegion, nil
	}
	items := make([]utils.PromptItem, len(regions))
	for i, region := range regions {
		items[i] = utils.PromptItem{
			Summary: region,
			Details: region,
		}
	}
	choice, err := utils.PromptChoice(ctx, title, items)
	if err != nil {
		return "", err
	}
	return choice.Summary, nil
}

func promptProjectName(ctx context.Context) (string, error) {
	title := "Enter your project name: "
	if name, err := utils.NewConsole().PromptText(ctx, title); err != nil {
		return "", err
	} else if name = strings.TrimSpace(name); len(name) > 0 {
		return name, nil
	}
	return "", nil
}

func promptOrgId(ctx context.Context) (string, error) {
	title := "Which organisation do you want to create the project for?"
	resp, err := utils.GetSupabase().V1ListAllOrganizationsWithResponse(ctx)
	if err != nil {
		return "", err
	}
	if resp.JSON200 == nil {
		return "", errors.New("Unexpected error retrieving organizations: " + string(resp.Body))
	}
	items := make([]utils.PromptItem, len(*resp.JSON200))
	for i, org := range *resp.JSON200 {
		items[i] = utils.PromptItem{Summary: org.Name, Details: org.Id}
	}
	choice, err := utils.PromptChoice(ctx, title, items)
	if err != nil {
		return "", err
	}
	return choice.Details, nil
}

func promptProjectRegion(ctx context.Context) (api.V1CreateProjectBodyRegion, error) {
	title := "Which region do you want to host the project in?"
	items := make([]utils.PromptItem, len(utils.CurrentProfile.ProjectRegions))
	for i, region := range utils.CurrentProfile.ProjectRegions {
		items[i] = utils.PromptItem{
			Summary: string(region),
			Details: utils.FormatRegion(string(region)),
		}
	}
	choice, err := utils.PromptChoice(ctx, title, items)
	if err != nil {
		return "", err
	}
	return api.V1CreateProjectBodyRegion(choice.Summary), nil
}
