// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package pages

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/db/query"
	functiondeploy "github.com/volcengine/byted-supabase-cli/internal/functions/deploy"
	"github.com/volcengine/byted-supabase-cli/internal/functions/gateway"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

var pagesProjectNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

// DemoAppArchiveURL is the sample Next.js + Supabase "filebox" frontend archive used to
// demo the one-click `pages fast create` / fast_create_pages flow. Single source of truth
// shared by the CLI command help and the MCP tool description so the two stay aligned.
const DemoAppArchiveURL = "https://lf3-static.bytednsdoc.com/obj/eden-cn/whkph/ljhwZthlaukjlkulzlp/nextjs-supabase-filebox.zip"

type ListParams struct {
	Name        string
	Limit       int
	Offset      int
	WorkspaceID string
	BranchID    string
}

type ScopedParams struct {
	WorkspaceID string
	BranchID    string
	ShowValues  bool
}

type BindParams struct {
	WorkspaceID     string
	BranchID        string
	PagesProjectID  string
	CustomPrefix    string
	FrameworkPrefix string
}

type CreateParams struct {
	Name                    string
	Provider                string
	Scope                   string
	ProjectDeployResourceID string
	Framework               string
	RootDir                 string
	OutputDir               string
	BuildCmd                string
	InstallCmd              string
	NodejsVersion           string
	NoDeploy                bool
}

type SyncParams struct {
	WorkspaceID string
	BranchID    string
}

type DeployParams struct {
	PagesProjectID          string
	ProjectDeployResourceID string
}

type DeployListParams struct {
	PagesProjectID string
	Limit          int
	Offset         int
}

type DeployResult struct {
	PagesProjectID          string `json:"ProjectID" toml:"project_id" yaml:"project_id"`
	DeployID                string `json:"DeployID" toml:"deploy_id" yaml:"deploy_id"`
	Provider                string `json:"Provider" toml:"provider" yaml:"provider"`
	ProjectDeployResourceID string `json:"ProjectDeployResourceID" toml:"project_deploy_resource_id" yaml:"project_deploy_resource_id"`
}

type UploadParams struct {
	FilePath string
}

type FastCreateParams struct {
	ProjectName     string
	FilePath        string
	CustomPrefix    string
	FrameworkPrefix string
	FunctionsInit   string
	MigrationsInit  string
}

type UploadResult struct {
	FileName                string `json:"FileName" toml:"file_name" yaml:"file_name"`
	Size                    int64  `json:"Size" toml:"size" yaml:"size"`
	ProjectDeployResourceID string `json:"ProjectDeployResourceID" toml:"project_deploy_resource_id" yaml:"project_deploy_resource_id"`
}

type CreateResult struct {
	ProjectID               string                       `json:"ProjectID,omitempty" toml:"project_id,omitempty" yaml:"project_id,omitempty"`
	DeployID                string                       `json:"DeployID,omitempty" toml:"deploy_id,omitempty" yaml:"deploy_id,omitempty"`
	PagesProjectName        string                       `json:"Name" toml:"pages_project_name" yaml:"pages_project_name"`
	Scope                   string                       `json:"Scope,omitempty" toml:"scope,omitempty" yaml:"scope,omitempty"`
	Provider                string                       `json:"Provider,omitempty" toml:"provider,omitempty" yaml:"provider,omitempty"`
	Status                  string                       `json:"Status,omitempty" toml:"status,omitempty" yaml:"status,omitempty"`
	LatestDeploy            string                       `json:"LatestDeploy,omitempty" toml:"latest_deploy,omitempty" yaml:"latest_deploy,omitempty"`
	ProjectDeployResourceID string                       `json:"ProjectDeployResourceID,omitempty" toml:"project_deploy_resource_id,omitempty" yaml:"project_deploy_resource_id,omitempty"`
	ProjectParam            volcengine.PagesProjectParam `json:"ProjectParam,omitempty" toml:"project_param,omitempty" yaml:"project_param,omitempty"`
	NoDeploy                bool                         `json:"NoDeploy,omitempty" toml:"no_deploy,omitempty" yaml:"no_deploy,omitempty"`
	CreateAt                string                       `json:"CreateAt,omitempty" toml:"create_at,omitempty" yaml:"create_at,omitempty"`
	UpdateAt                string                       `json:"UpdateAt,omitempty" toml:"update_at,omitempty" yaml:"update_at,omitempty"`
}

type FastCreateResult struct {
	PagesProjectID          string `json:"PagesProjectId" toml:"pages_project_id" yaml:"pages_project_id"`
	PagesProjectName        string `json:"PagesProjectName" toml:"pages_project_name" yaml:"pages_project_name"`
	PreviewDomain           string `json:"PreviewDomain" toml:"preview_domain" yaml:"preview_domain"`
	WorkspaceID             string `json:"WorkspaceId" toml:"workspace_id" yaml:"workspace_id"`
	WorkspaceName           string `json:"WorkspaceName" toml:"workspace_name" yaml:"workspace_name"`
	Region                  string `json:"Region" toml:"region" yaml:"region"`
	BranchID                string `json:"BranchId" toml:"branch_id" yaml:"branch_id"`
	ProjectDeployResourceID string `json:"ProjectDeployResourceID" toml:"project_deploy_resource_id" yaml:"project_deploy_resource_id"`
	DeployID                string `json:"DeployID" toml:"deploy_id" yaml:"deploy_id"`
	FunctionsInit           string `json:"FunctionsInit,omitempty" toml:"functions_init,omitempty" yaml:"functions_init,omitempty"`
	MigrationsInit          string `json:"MigrationsInit,omitempty" toml:"migrations_init,omitempty" yaml:"migrations_init,omitempty"`
}

func List(ctx context.Context, params ListParams) error {
	cfg, err := volcengine.LoadConfigFromEnv()
	if err != nil {
		return err
	}
	result, err := volcengine.NewClient(cfg).ListPagesProjects(volcengine.ListPagesProjectsParams{
		Name:   strings.TrimSpace(params.Name),
		Limit:  params.Limit,
		Offset: params.Offset,
		Filter: volcengine.VolcSupabaseFilter{
			WorkspaceID: strings.TrimSpace(params.WorkspaceID),
			BranchID:    strings.TrimSpace(params.BranchID),
		},
	})
	if err != nil {
		return err
	}
	return outputPagesProjects(result)
}

func EnvVars(ctx context.Context, params ScopedParams) error {
	workspace, client, err := resolveWorkspace(ctx, params.WorkspaceID)
	if err != nil {
		return err
	}
	branchID, err := resolveBranchID(ctx, client, workspace.WorkspaceID, params.BranchID)
	if err != nil {
		return err
	}
	result, err := client.DescribeSupabaseDeployEnvVars(workspace.WorkspaceID, branchID)
	if err != nil {
		return err
	}
	envVars := result.EnvVars
	if !params.ShowValues {
		envVars = maskedEnvVars(envVars)
	}
	return outputEnvVars(envVars)
}

func Binding(ctx context.Context, params ScopedParams) error {
	workspace, client, err := resolveWorkspace(ctx, params.WorkspaceID)
	if err != nil {
		return err
	}
	branchID, err := resolveBranchID(ctx, client, workspace.WorkspaceID, params.BranchID)
	if err != nil {
		return err
	}
	result, err := client.DescribePagesBinding(workspace.WorkspaceID, branchID)
	if err != nil {
		return err
	}
	return outputBinding(result)
}

func Bind(ctx context.Context, params BindParams) error {
	workspace, cfg, client, err := resolveWorkspaceWithConfig(ctx, params.WorkspaceID)
	if err != nil {
		return err
	}
	// Gate frontend deployment on the Pages access role (DCDNAccessAIDPRole).
	if err := client.CheckPagesAccessRole(); err != nil {
		return err
	}
	branchID, err := resolveBranchID(ctx, client, workspace.WorkspaceID, params.BranchID)
	if err != nil {
		return err
	}
	pagesProjectID := strings.TrimSpace(params.PagesProjectID)
	if pagesProjectID == "" {
		return errors.New("missing pages project id")
	}
	if err := confirm(ctx, fmt.Sprintf("bind Pages project %s to branch %s", pagesProjectID, branchID)); err != nil {
		return err
	}
	if err := volcengine.NewWriteClient(cfg).BindPagesProject(volcengine.BindPagesProjectParams{
		WorkspaceID:     workspace.WorkspaceID,
		BranchID:        branchID,
		PagesProjectID:  pagesProjectID,
		CustomPrefix:    strings.TrimSpace(params.CustomPrefix),
		FrameworkPrefix: strings.TrimSpace(params.FrameworkPrefix),
	}); err != nil {
		return err
	}
	result := struct {
		WorkspaceID     string `json:"WorkspaceId" toml:"workspace_id" yaml:"workspace_id"`
		BranchID        string `json:"BranchId" toml:"branch_id" yaml:"branch_id"`
		PagesProjectID  string `json:"PagesProjectId" toml:"pages_project_id" yaml:"pages_project_id"`
		CustomPrefix    string `json:"CustomPrefix,omitempty" toml:"custom_prefix,omitempty" yaml:"custom_prefix,omitempty"`
		FrameworkPrefix string `json:"FrameworkPrefix,omitempty" toml:"framework_prefix,omitempty" yaml:"framework_prefix,omitempty"`
	}{
		WorkspaceID:     workspace.WorkspaceID,
		BranchID:        branchID,
		PagesProjectID:  pagesProjectID,
		CustomPrefix:    strings.TrimSpace(params.CustomPrefix),
		FrameworkPrefix: strings.TrimSpace(params.FrameworkPrefix),
	}
	if utils.OutputFormat.Value == utils.OutputPretty {
		fmt.Fprintf(os.Stderr, "Bound Pages project %s to branch %s.\n", utils.Aqua(pagesProjectID), utils.Aqua(branchID))
	} else if err := utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result); err != nil {
		return err
	}
	printPagesBindNextStepsHint(os.Stderr, pagesProjectID, workspace.WorkspaceID, branchID)
	return nil
}

func Create(ctx context.Context, params CreateParams) error {
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return errors.New("missing pages project name")
	}
	if err := ValidateProjectName(name); err != nil {
		return err
	}
	provider := strings.TrimSpace(params.Provider)
	if provider == "" {
		provider = "upload_v2"
	}
	if provider != "upload_v2" {
		return errors.New("only upload_v2 provider is currently supported")
	}
	scope := strings.TrimSpace(params.Scope)
	if scope == "" {
		scope = "domestic"
	}
	switch scope {
	case "domestic", "overseas", "global":
	default:
		return errors.New("invalid --scope. Expected domestic, overseas, or global.")
	}
	resourceID := strings.TrimSpace(params.ProjectDeployResourceID)
	if provider == "upload_v2" && resourceID == "" {
		return errors.New("missing required flag: --resource-id")
	}
	cfg, err := volcengine.LoadConfigFromEnv()
	if err != nil {
		return err
	}
	// Gate frontend deployment on the Pages access role (DCDNAccessAIDPRole).
	if err := volcengine.NewClient(cfg).CheckPagesAccessRole(); err != nil {
		return err
	}
	action := fmt.Sprintf("create Pages project %s", name)
	if params.NoDeploy {
		action += fmt.Sprintf(" without deployment using resource %s", resourceID)
	} else {
		action += fmt.Sprintf(" with resource %s", resourceID)
	}
	if err := confirm(ctx, action); err != nil {
		return err
	}
	projectParam := volcengine.PagesProjectParam{
		Framework:     strings.TrimSpace(params.Framework),
		RootDir:       strings.TrimSpace(params.RootDir),
		OutputDir:     strings.TrimSpace(params.OutputDir),
		BuildCmd:      strings.TrimSpace(params.BuildCmd),
		InstallCmd:    strings.TrimSpace(params.InstallCmd),
		NodejsVersion: strings.TrimSpace(params.NodejsVersion),
	}
	result, err := volcengine.NewWriteClient(cfg).CreatePagesProject(volcengine.CreatePagesProjectParams{
		Name:                    name,
		Provider:                provider,
		Scope:                   scope,
		ProjectDeployResourceID: resourceID,
		ProjectParam:            projectParam,
		NoDeploy:                params.NoDeploy,
	})
	if err != nil {
		return err
	}
	output := CreateResult{
		ProjectID:               result.ProjectID,
		DeployID:                result.DeployID,
		PagesProjectName:        result.PagesProjectName,
		Scope:                   result.Scope,
		Provider:                result.Provider,
		Status:                  result.Status,
		LatestDeploy:            result.LatestDeploy,
		ProjectDeployResourceID: result.ProjectDeployResourceID,
		ProjectParam:            result.ProjectParam,
		NoDeploy:                result.NoDeploy,
		CreateAt:                result.CreateAt,
		UpdateAt:                result.UpdateAt,
	}
	if utils.OutputFormat.Value == utils.OutputPretty {
		if output.ProjectID == "" {
			fmt.Fprintf(os.Stderr, "Created Pages project %s.\n", utils.Aqua(output.PagesProjectName))
		} else {
			fmt.Fprintf(os.Stderr, "Created Pages project %s [%s].\n", utils.Aqua(output.PagesProjectName), utils.Aqua(output.ProjectID))
		}
		if output.NoDeploy {
			fmt.Fprintln(os.Stderr, "NoDeploy: true")
		}
		if output.DeployID != "" {
			fmt.Fprintf(os.Stderr, "DeployID: %s\n", utils.Aqua(output.DeployID))
		}
		if output.LatestDeploy != "" {
			fmt.Fprintf(os.Stderr, "LatestDeploy: %s\n", utils.Aqua(output.LatestDeploy))
		}
		return nil
	}
	if utils.OutputFormat.Value == utils.OutputEnv {
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, output)
}

func Upload(ctx context.Context, params UploadParams) error {
	cfg, err := volcengine.LoadConfigFromEnv()
	if err != nil {
		return err
	}
	// Gate frontend deployment on the Pages access role (DCDNAccessAIDPRole).
	if err := volcengine.NewClient(cfg).CheckPagesAccessRole(); err != nil {
		return err
	}
	result, err := UploadResourceWithClient(ctx, volcengine.NewWriteClient(cfg), params.FilePath)
	if err != nil {
		return err
	}
	if utils.OutputFormat.Value == utils.OutputPretty {
		fmt.Fprintf(os.Stderr, "Uploaded Pages resource %s.\nProjectDeployResourceID: %s\n", utils.Aqua(result.FileName), utils.Aqua(result.ProjectDeployResourceID))
		return nil
	}
	if utils.OutputFormat.Value == utils.OutputEnv {
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
}

func FastCreate(ctx context.Context, params FastCreateParams) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	if len(cfgSet.Regions) == 0 {
		return errors.New("missing Volcengine region")
	}
	cfg := cfgSet.ConfigForRegion(cfgSet.Regions[0])
	if err := confirm(ctx, fmt.Sprintf("fast create Pages project and Supabase workspace %s", strings.TrimSpace(params.ProjectName))); err != nil {
		return err
	}
	result, err := RunFastCreate(ctx, cfg, params, os.Stderr)
	if err != nil {
		return err
	}
	return outputFastCreateResult(result)
}

// RunFastCreate runs the end-to-end Pages + Supabase fast-create chain: stage the
// frontend (a directory is auto-zipped, a prebuilt archive is uploaded as-is), create
// the Pages project, create a new Supabase workspace, wait for it to become ready,
// optionally apply SQL migrations and deploy Edge Functions, bind Pages to the
// workspace, deploy, and wait for the deployment to succeed.
//
// It performs no confirmation prompt and no stdout/output side effects (progress lines
// are written to progress, which may be nil → discarded), so it is shared by the CLI
// `pages fast create` command and the MCP fast_create_pages tool. It blocks until the
// deployment succeeds, ctx is cancelled, or a step fails.
func RunFastCreate(ctx context.Context, cfg volcengine.Config, params FastCreateParams, progress io.Writer) (FastCreateResult, error) {
	if progress == nil {
		progress = io.Discard
	}
	projectName := strings.TrimSpace(params.ProjectName)
	if projectName == "" {
		return FastCreateResult{}, errors.New("missing pages project name")
	}
	if err := ValidateProjectName(projectName); err != nil {
		return FastCreateResult{}, err
	}
	resourceFile, cleanup, err := preparePagesResourceFile(strings.TrimSpace(params.FilePath))
	if err != nil {
		return FastCreateResult{}, err
	}
	defer cleanup()
	functionsInit, err := validateOptionalDir(params.FunctionsInit, "functions init")
	if err != nil {
		return FastCreateResult{}, err
	}
	migrationsInit, err := validateOptionalDir(params.MigrationsInit, "migrations init")
	if err != nil {
		return FastCreateResult{}, err
	}
	client := volcengine.NewClient(cfg)
	writeClient := volcengine.NewWriteClient(cfg)

	// fast create provisions a brand-new workspace and deploys a Pages frontend; gate on
	// both required roles first so we fail before the upload / Pages-project side effects
	// rather than after, when the account is not yet authorized.
	if err := client.CheckAIDAPServiceLinkedRole(); err != nil {
		return FastCreateResult{}, err
	}
	if err := client.CheckPagesAccessRole(); err != nil {
		return FastCreateResult{}, err
	}

	fmt.Fprintln(progress, "Uploading Pages resource...")
	upload, err := UploadResourceWithClient(ctx, writeClient, resourceFile)
	if err != nil {
		return FastCreateResult{}, err
	}
	fmt.Fprintf(progress, "Uploaded Pages resource. ProjectDeployResourceID: %s\n", utils.Aqua(upload.ProjectDeployResourceID))

	fmt.Fprintln(progress, "Creating Pages project...")
	pagesProject, err := writeClient.CreatePagesProject(volcengine.CreatePagesProjectParams{
		Name:                    projectName,
		Provider:                "upload_v2",
		Scope:                   "domestic",
		ProjectDeployResourceID: upload.ProjectDeployResourceID,
		NoDeploy:                true,
	})
	if err != nil {
		return FastCreateResult{}, err
	}
	if pagesProject.ProjectID == "" {
		return FastCreateResult{}, errors.New("CreatePagesProject returned empty ProjectID")
	}
	fmt.Fprintf(progress, "Created Pages project. PagesProjectID: %s\n", utils.Aqua(pagesProject.ProjectID))

	fmt.Fprintln(progress, "Creating Supabase workspace...")
	workspaceResult, err := writeClient.CreateSupabaseWorkspace(ctx, volcengine.CreateWorkspaceParams{
		WorkspaceName: projectName,
	})
	if err != nil {
		return FastCreateResult{}, err
	}
	workspaceID := strings.TrimSpace(workspaceResult.WorkspaceID)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(workspaceResult.Workspace.WorkspaceID)
	}
	if workspaceID == "" {
		return FastCreateResult{}, errors.New("CreateWorkspace returned empty WorkspaceId")
	}
	fmt.Fprintf(progress, "Created Supabase workspace. WorkspaceID: %s\n", utils.Aqua(workspaceID))

	workspace, branch, err := waitForFastCreateWorkspaceReady(ctx, client, workspaceID, progress)
	if err != nil {
		return FastCreateResult{}, err
	}
	branchID := strings.TrimSpace(branch.BranchID)
	if branchID == "" {
		return FastCreateResult{}, errors.Errorf("failed to resolve default branch for workspace %s", workspaceID)
	}
	fmt.Fprintf(progress, "Resolved default branch. BranchID: %s\n", utils.Aqua(branchID))

	if migrationsInit != "" {
		if err := runFastCreateMigrations(ctx, client, workspaceID, branchID, migrationsInit, progress); err != nil {
			return FastCreateResult{}, err
		}
	}
	if functionsInit != "" {
		if err := deployFastCreateFunctions(ctx, client, workspaceID, branchID, functionsInit, progress); err != nil {
			return FastCreateResult{}, err
		}
	}

	fmt.Fprintln(progress, "Binding Pages project to Supabase workspace...")
	if err := writeClient.BindPagesProject(volcengine.BindPagesProjectParams{
		WorkspaceID:     workspaceID,
		BranchID:        branchID,
		PagesProjectID:  pagesProject.ProjectID,
		CustomPrefix:    strings.TrimSpace(params.CustomPrefix),
		FrameworkPrefix: strings.TrimSpace(params.FrameworkPrefix),
	}); err != nil {
		return FastCreateResult{}, err
	}

	fmt.Fprintln(progress, "Deploying Pages project...")
	deploy, err := writeClient.CreatePagesDeploy(volcengine.CreatePagesDeployParams{
		PagesProjectID:          pagesProject.ProjectID,
		ProjectDeployResourceID: upload.ProjectDeployResourceID,
	})
	if err != nil {
		return FastCreateResult{}, err
	}
	fmt.Fprintf(progress, "Created Pages deployment. DeployID: %s\n", utils.Aqua(deploy.DeployID))
	if _, err := waitForPagesDeploySuccess(ctx, client, pagesProject.ProjectID, deploy.DeployID, progress); err != nil {
		return FastCreateResult{}, err
	}
	fmt.Fprintln(progress, "Resolving Pages preview domain...")
	binding, err := client.DescribePagesBinding(workspaceID, branchID)
	if err != nil {
		return FastCreateResult{}, err
	}
	previewDomain, err := fastCreatePreviewDomain(binding)
	if err != nil {
		return FastCreateResult{}, err
	}
	return FastCreateResult{
		PagesProjectID:          pagesProject.ProjectID,
		PagesProjectName:        projectName,
		PreviewDomain:           previewDomain,
		WorkspaceID:             workspaceID,
		WorkspaceName:           workspace.WorkspaceName,
		Region:                  cfg.Region,
		BranchID:                branchID,
		ProjectDeployResourceID: upload.ProjectDeployResourceID,
		DeployID:                deploy.DeployID,
		FunctionsInit:           functionsInit,
		MigrationsInit:          migrationsInit,
	}, nil
}

func fastCreatePreviewDomain(result volcengine.DescribePagesBindingResult) (string, error) {
	if result.Binding == nil {
		return "", errors.New("Pages binding was not found after deployment")
	}
	if result.PagesProject == nil {
		return "", errors.New("Pages binding response did not include project details")
	}
	previewDomain := strings.TrimSpace(result.PagesProject.PreviewDomain)
	if previewDomain == "" {
		return "", errors.New("Pages binding response did not include a preview domain")
	}
	return previewDomain, nil
}

// ValidateProjectName checks a Pages project name against the platform naming rules:
// 2-63 characters, lowercase letters, digits and hyphens only, no leading or trailing
// hyphen. Shared by the CLI `pages create` command and the MCP create_pages_project tool.
func ValidateProjectName(name string) error {
	if len(name) < 2 || len(name) > 63 || !pagesProjectNamePattern.MatchString(name) {
		return errors.New("invalid pages project name. Project name must be 2-63 characters and contain only lowercase letters, numbers, and hyphens; it cannot start or end with a hyphen.")
	}
	return nil
}

// UploadResourceWithClient stages a single deployment resource file (a built static-site
// archive) using the given client and returns the resource id to reference from create or
// deploy. It performs no output/formatting side effects, so it is shared by the CLI
// `pages upload` command and the MCP upload_pages_resource tool.
func UploadResourceWithClient(ctx context.Context, client *volcengine.Client, filePath string) (UploadResult, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return UploadResult{}, errors.New("missing pages resource file path")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return UploadResult{}, errors.Errorf("failed to open pages resource file: %w", err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return UploadResult{}, errors.Errorf("failed to stat pages resource file: %w", err)
	}
	if stat.IsDir() {
		return UploadResult{}, errors.Errorf("pages resource path %q is a directory", filePath)
	}
	fileName := filepath.Base(filePath)
	uploadURL, err := client.UploadPagesResourcesPostURL(volcengine.UploadPagesResourcesPostURLParams{
		FileName: fileName,
	})
	if err != nil {
		return UploadResult{}, err
	}
	if err := uploadResource(ctx, uploadURL.TosUploadURL, file, stat.Size()); err != nil {
		return UploadResult{}, err
	}
	return UploadResult{
		FileName:                fileName,
		Size:                    stat.Size(),
		ProjectDeployResourceID: uploadURL.ProjectDeployResourceID,
	}, nil
}

func preparePagesResourceFile(filePath string) (string, func(), error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", func() {}, errors.New("missing frontend path")
	}
	stat, err := os.Stat(filePath)
	if err != nil {
		return "", func() {}, errors.Errorf("failed to stat frontend path: %w", err)
	}
	if !stat.IsDir() {
		if abs, err := filepath.Abs(filePath); err == nil {
			filePath = abs
		}
		return filePath, func() {}, nil
	}
	archive, err := os.CreateTemp("", "byted-supabase-pages-frontend-*.zip")
	if err != nil {
		return "", func() {}, errors.Errorf("failed to create temporary frontend archive: %w", err)
	}
	archivePath := archive.Name()
	if err := archive.Close(); err != nil {
		_ = os.Remove(archivePath)
		return "", func() {}, errors.Errorf("failed to close temporary frontend archive: %w", err)
	}
	if err := zipDirectory(filePath, archivePath); err != nil {
		_ = os.Remove(archivePath)
		return "", func() {}, err
	}
	return archivePath, func() { _ = os.Remove(archivePath) }, nil
}

func zipDirectory(root, archivePath string) error {
	archive, err := os.Create(archivePath)
	if err != nil {
		return errors.Errorf("failed to create frontend archive: %w", err)
	}
	defer archive.Close()
	zw := zip.NewWriter(archive)
	defer zw.Close()
	root, err = filepath.Abs(root)
	if err != nil {
		return errors.Errorf("failed to resolve frontend path: %w", err)
	}
	return filepath.WalkDir(root, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if filePath == root {
			return nil
		}
		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		if shouldSkipFrontendArchivePath(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(w, file)
		return err
	})
}

func shouldSkipFrontendArchivePath(rel string, d fs.DirEntry) bool {
	return shouldSkipGeneratedFile(rel, d, map[string]struct{}{".next": {}})
}

func shouldSkipGeneratedFile(rel string, d fs.DirEntry, extraDirs map[string]struct{}) bool {
	name := d.Name()
	if name == ".DS_Store" || strings.HasPrefix(name, "._") {
		return true
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		switch part {
		case ".git", "node_modules":
			return true
		}
		if _, ok := extraDirs[part]; ok {
			return true
		}
	}
	return false
}

func validateOptionalDir(input, label string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}
	stat, err := os.Stat(input)
	if err != nil {
		return "", errors.Errorf("failed to stat %s directory: %w", label, err)
	}
	if !stat.IsDir() {
		return "", errors.Errorf("%s path %q is not a directory", label, input)
	}
	if abs, err := filepath.Abs(input); err == nil {
		input = abs
	}
	return input, nil
}

func validateRegularFile(path, label string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.Errorf("missing %s path", label)
	}
	stat, err := os.Stat(path)
	if err != nil {
		return errors.Errorf("failed to stat %s: %w", label, err)
	}
	if stat.IsDir() {
		return errors.Errorf("%s path %q is a directory", label, path)
	}
	return nil
}

func waitForFastCreateWorkspaceReady(ctx context.Context, client *volcengine.Client, workspaceID string, progress io.Writer) (volcengine.Workspace, volcengine.Branch, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		workspaceResult, err := client.DescribeWorkspaceDetail(ctx, workspaceID)
		if err != nil {
			return volcengine.Workspace{}, volcengine.Branch{}, err
		}
		workspace := workspaceResult.Workspace
		if workspace.WorkspaceStatus == "Running" {
			branchResult, err := client.DescribeDefaultBranch(ctx, workspaceID)
			if err != nil {
				return volcengine.Workspace{}, volcengine.Branch{}, err
			}
			branch := branchResult.Branch
			if branch.BranchID != "" && branch.BranchStatus == "Ready" {
				return workspace, branch, nil
			}
			fmt.Fprintf(progress, "Waiting for default branch to become Ready (current: %s)...\n", valueOrPlaceholder(branch.BranchStatus))
		} else {
			fmt.Fprintf(progress, "Waiting for workspace to become Running (current: %s)...\n", valueOrPlaceholder(workspace.WorkspaceStatus))
		}
		select {
		case <-ctx.Done():
			return volcengine.Workspace{}, volcengine.Branch{}, errors.Errorf("timed out waiting for workspace %s to become Running and default branch Ready", workspaceID)
		case <-ticker.C:
		}
	}
}

func waitForPagesDeploySuccess(ctx context.Context, client *volcengine.Client, pagesProjectID, deployID string, progress io.Writer) (volcengine.PagesDeploySummary, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		result, err := client.ListPagesDeploy(volcengine.ListPagesDeployParams{
			PagesProjectID: pagesProjectID,
			Limit:          10,
		})
		if err != nil {
			return volcengine.PagesDeploySummary{}, err
		}
		for _, deploy := range result.Deployments {
			id := strings.TrimSpace(deploy.ID)
			if id == "" {
				id = strings.TrimSpace(deploy.DeployID)
			}
			if id != deployID {
				continue
			}
			if deploy.Status == "DeploySuccess" {
				return deploy, nil
			}
			fmt.Fprintf(progress, "Waiting for Pages deployment %s to become DeploySuccess (current: %s)...\n", deployID, valueOrPlaceholder(deploy.Status))
			break
		}
		select {
		case <-ctx.Done():
			return volcengine.PagesDeploySummary{}, errors.Errorf("timed out waiting for Pages deployment %s to become DeploySuccess", deployID)
		case <-ticker.C:
		}
	}
}

func runFastCreateMigrations(ctx context.Context, client *volcengine.Client, workspaceID, branchID, migrationsDir string, progress io.Writer) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return errors.Errorf("failed to list migration files: %w", err)
	}
	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || shouldSkipFastCreateMigrationFile(entry.Name()) {
			continue
		}
		files = append(files, filepath.Join(migrationsDir, entry.Name()))
	}
	sort.Strings(files)
	if len(files) == 0 {
		return errors.Errorf("no SQL migration files found in %s", migrationsDir)
	}
	for _, filePath := range files {
		fmt.Fprintln(progress, "Executing migration:", utils.Bold(filepath.Base(filePath)))
		sql, err := os.ReadFile(filePath)
		if err != nil {
			return errors.Errorf("failed to read migration file %s: %w", filePath, err)
		}
		if strings.TrimSpace(string(sql)) == "" {
			fmt.Fprintln(progress, "Skipping empty migration:", utils.Bold(filepath.Base(filePath)))
			continue
		}
		if _, err := query.ExecuteVolcengine(ctx, client, query.VolcengineParams{
			WorkspaceID: workspaceID,
			BranchID:    branchID,
			SQL:         string(sql),
			ReadOnly:    false,
		}); err != nil {
			return errors.Errorf("failed to execute migration %s: %w", filepath.Base(filePath), err)
		}
	}
	return nil
}

func shouldSkipFastCreateMigrationFile(name string) bool {
	return strings.HasPrefix(name, ".") || !strings.HasSuffix(strings.ToLower(name), ".sql")
}

func deployFastCreateFunctions(ctx context.Context, client *volcengine.Client, workspaceID, branchID, backendDir string, progress io.Writer) error {
	functionsDir := filepath.Join(backendDir, "functions")
	stat, err := os.Stat(functionsDir)
	if err != nil {
		return errors.Errorf("failed to stat functions directory %s: %w", functionsDir, err)
	}
	if !stat.IsDir() {
		return errors.Errorf("functions path %q is not a directory", functionsDir)
	}
	entries, err := os.ReadDir(functionsDir)
	if err != nil {
		return errors.Errorf("failed to read functions directory: %w", err)
	}
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return err
	}
	deployed := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		slug := entry.Name()
		if err := utils.ValidateFunctionSlug(slug); err != nil {
			return err
		}
		functionDir := filepath.Join(functionsDir, slug)
		files, err := collectFastCreateFunctionFiles(functionDir)
		if err != nil {
			return err
		}
		runtime, entrypoint := detectFastCreateFunctionRuntime(functionDir, slug)
		fmt.Fprintln(progress, "Deploying Edge Function:", utils.Bold(slug))
		verifyJWT := true
		if _, err := gateway.Deploy(ctx, access, gateway.DeployRequest{
			ProjectRef: workspaceID,
			Slug:       slug,
			Metadata: gateway.DeployMetadata{
				EntrypointPath: entrypoint,
				Name:           slug,
				Runtime:        runtime,
				VerifyJWT:      &verifyJWT,
			},
			Files: files,
		}); err != nil {
			return errors.Errorf("failed to deploy Edge Function %s: %w", slug, err)
		}
		deployed = append(deployed, slug)
	}
	if len(deployed) == 0 {
		return errors.Errorf("no Edge Functions found under %s", functionsDir)
	}
	fmt.Fprintf(progress, "Deployed Edge Functions: %s\n", strings.Join(deployed, ", "))
	return nil
}

func collectFastCreateFunctionFiles(functionDir string) ([]gateway.File, error) {
	files := make([]gateway.File, 0)
	err := filepath.WalkDir(functionDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if filePath == functionDir {
			return nil
		}
		rel, err := filepath.Rel(functionDir, filePath)
		if err != nil {
			return err
		}
		if shouldSkipFunctionFile(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			return errors.Errorf("failed to read function file %s: %w", filePath, err)
		}
		name := filepath.ToSlash(rel)
		if name == "" || strings.HasPrefix(name, "../") || path.IsAbs(name) {
			return errors.Errorf("invalid function file path %q", name)
		}
		files = append(files, gateway.File{Name: name, Content: string(content)})
		return nil
	})
	if err != nil {
		return nil, errors.Errorf("failed to collect function files: %w", err)
	}
	if len(files) == 0 {
		return nil, errors.Errorf("no function files found in %s", functionDir)
	}
	return files, nil
}

func shouldSkipFunctionFile(rel string, d fs.DirEntry) bool {
	return shouldSkipGeneratedFile(rel, d, nil)
}

func detectFastCreateFunctionRuntime(functionDir, slug string) (runtime, entrypoint string) {
	if _, err := os.Stat(filepath.Join(functionDir, "run.sh")); err == nil {
		return detectFastCreatePythonRuntime(slug), "app.py"
	}
	return functiondeploy.RuntimeNativeNode20, "index.ts"
}

func detectFastCreatePythonRuntime(slug string) string {
	normalized := strings.ToLower(strings.NewReplacer("_", "-", ".", "-").Replace(slug))
	switch {
	case strings.Contains(normalized, "python-3-9") || strings.Contains(normalized, "python39"):
		return functiondeploy.RuntimePython39
	case strings.Contains(normalized, "python-3-10") || strings.Contains(normalized, "python310"):
		return functiondeploy.RuntimePython310
	case strings.Contains(normalized, "python-3-12") || strings.Contains(normalized, "python312"):
		return functiondeploy.RuntimePython312
	default:
		return functiondeploy.RuntimePython312
	}
}

func valueOrPlaceholder(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "<empty>"
	}
	return value
}

func Deploy(ctx context.Context, params DeployParams) error {
	pagesProjectID := strings.TrimSpace(params.PagesProjectID)
	if pagesProjectID == "" {
		return errors.New("missing pages project id")
	}
	resourceID := strings.TrimSpace(params.ProjectDeployResourceID)
	if resourceID == "" {
		return errors.New("missing required flag: --resource-id")
	}
	cfg, err := volcengine.LoadConfigFromEnv()
	if err != nil {
		return err
	}
	// Gate frontend deployment on the Pages access role (DCDNAccessAIDPRole).
	if err := volcengine.NewClient(cfg).CheckPagesAccessRole(); err != nil {
		return err
	}
	if err := confirm(ctx, fmt.Sprintf("deploy Pages project %s with resource %s", pagesProjectID, resourceID)); err != nil {
		return err
	}
	result, err := volcengine.NewWriteClient(cfg).CreatePagesDeploy(volcengine.CreatePagesDeployParams{
		PagesProjectID:          pagesProjectID,
		ProjectDeployResourceID: resourceID,
	})
	if err != nil {
		return err
	}
	output := DeployResult{
		PagesProjectID:          pagesProjectID,
		DeployID:                result.DeployID,
		Provider:                result.Provider,
		ProjectDeployResourceID: resourceID,
	}
	switch utils.OutputFormat.Value {
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	case utils.OutputPretty:
		fmt.Fprintf(os.Stderr, "Created Pages deployment %s for project %s.\n", utils.Aqua(result.DeployID), utils.Aqua(pagesProjectID))
	default:
		if err := utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, output); err != nil {
			return err
		}
	}
	printPagesDeployPreviewHint(os.Stderr)
	return nil
}

func DeployList(ctx context.Context, params DeployListParams) error {
	pagesProjectID := strings.TrimSpace(params.PagesProjectID)
	if pagesProjectID == "" {
		return errors.New("missing pages project id")
	}
	cfg, err := volcengine.LoadConfigFromEnv()
	if err != nil {
		return err
	}
	result, err := volcengine.NewClient(cfg).ListPagesDeploy(volcengine.ListPagesDeployParams{
		PagesProjectID: pagesProjectID,
		Limit:          params.Limit,
		Offset:         params.Offset,
	})
	if err != nil {
		return err
	}
	return outputPagesDeploys(result)
}

func Sync(ctx context.Context, params SyncParams) error {
	workspace, cfg, client, err := resolveWorkspaceWithConfig(ctx, params.WorkspaceID)
	if err != nil {
		return err
	}
	// Gate frontend deployment on the Pages access role (DCDNAccessAIDPRole).
	if err := client.CheckPagesAccessRole(); err != nil {
		return err
	}
	branchID, err := resolveBranchID(ctx, client, workspace.WorkspaceID, params.BranchID)
	if err != nil {
		return err
	}
	binding, err := client.DescribePagesBinding(workspace.WorkspaceID, branchID)
	if err != nil {
		return err
	}
	if binding.Binding == nil {
		return errors.Errorf("no Pages binding found for workspace %s branch %s. Run `pages bind <pages-project-id>` first.", workspace.WorkspaceID, branchID)
	}
	pagesProjectID := strings.TrimSpace(binding.Binding.PagesProjectID)
	if pagesProjectID == "" {
		return errors.Errorf("Pages binding for workspace %s branch %s does not include a Pages project id", workspace.WorkspaceID, branchID)
	}
	if err := confirm(ctx, fmt.Sprintf("sync Supabase env vars to bound Pages project %s for branch %s", pagesProjectID, branchID)); err != nil {
		return err
	}
	if err := volcengine.NewWriteClient(cfg).SyncPagesDeployEnvVars(volcengine.SyncPagesDeployEnvVarsParams{
		WorkspaceID:    workspace.WorkspaceID,
		BranchID:       branchID,
		PagesProjectID: pagesProjectID,
	}); err != nil {
		return err
	}
	result := struct {
		WorkspaceID    string `json:"WorkspaceId" toml:"workspace_id" yaml:"workspace_id"`
		BranchID       string `json:"BranchId" toml:"branch_id" yaml:"branch_id"`
		PagesProjectID string `json:"PagesProjectId" toml:"pages_project_id" yaml:"pages_project_id"`
	}{
		WorkspaceID:    workspace.WorkspaceID,
		BranchID:       branchID,
		PagesProjectID: pagesProjectID,
	}
	if utils.OutputFormat.Value == utils.OutputPretty {
		fmt.Fprintf(os.Stderr, "Synced Supabase env vars to Pages project %s for branch %s.\n", utils.Aqua(pagesProjectID), utils.Aqua(branchID))
		return nil
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
}

func Unbind(ctx context.Context, params ScopedParams) error {
	workspace, cfg, client, err := resolveWorkspaceWithConfig(ctx, params.WorkspaceID)
	if err != nil {
		return err
	}
	branchID, err := resolveBranchID(ctx, client, workspace.WorkspaceID, params.BranchID)
	if err != nil {
		return err
	}
	if err := confirm(ctx, "unbind Pages project from branch "+branchID); err != nil {
		return err
	}
	if err := volcengine.NewWriteClient(cfg).UnbindPagesProject(volcengine.UnbindPagesProjectParams{
		WorkspaceID: workspace.WorkspaceID,
		BranchID:    branchID,
	}); err != nil {
		return err
	}
	result := struct {
		WorkspaceID string `json:"WorkspaceId" toml:"workspace_id" yaml:"workspace_id"`
		BranchID    string `json:"BranchId" toml:"branch_id" yaml:"branch_id"`
	}{
		WorkspaceID: workspace.WorkspaceID,
		BranchID:    branchID,
	}
	if utils.OutputFormat.Value == utils.OutputPretty {
		fmt.Fprintf(os.Stderr, "Unbound Pages project from branch %s.\n", utils.Aqua(branchID))
		return nil
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
}

func uploadResource(ctx context.Context, uploadURL string, body io.Reader, size int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, body)
	if err != nil {
		return errors.Errorf("failed to create pages resource upload request: %w", err)
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return errors.Errorf("failed to upload pages resource: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return errors.Errorf("failed to upload pages resource: status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return nil
}

func resolveWorkspace(ctx context.Context, workspaceID string) (volcengine.Workspace, *volcengine.Client, error) {
	workspace, _, client, err := resolveWorkspaceWithConfig(ctx, workspaceID)
	if err != nil {
		return volcengine.Workspace{}, nil, err
	}
	return workspace, client, nil
}

func resolveWorkspaceWithConfig(ctx context.Context, workspaceID string) (volcengine.Workspace, volcengine.Config, *volcengine.Client, error) {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return volcengine.Workspace{}, volcengine.Config{}, nil, err
	}
	workspace, cfg, err := findSupabaseWorkspace(ctx, cfgSet, workspaceID)
	if err != nil {
		return volcengine.Workspace{}, volcengine.Config{}, nil, err
	}
	return workspace, cfg, volcengine.NewClient(cfg), nil
}

func findSupabaseWorkspace(ctx context.Context, cfgSet volcengine.ConfigSet, workspaceID string) (volcengine.Workspace, volcengine.Config, error) {
	var lastErr error
	for _, region := range cfgSet.Regions {
		cfg := cfgSet.ConfigForRegion(region)
		result, err := volcengine.NewClient(cfg).DescribeWorkspaceDetail(ctx, workspaceID)
		if err != nil {
			lastErr = err
			continue
		}
		workspace := result.Workspace
		if workspace.WorkspaceID == "" {
			continue
		}
		if workspace.EngineType != "" && workspace.EngineType != volcengine.EngineTypeSupabase {
			return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("workspace %s is %s, expected Supabase", workspaceID, workspace.EngineType)
		}
		if workspace.WorkspaceName == "" {
			workspace.WorkspaceName = workspace.WorkspaceID
		}
		return workspace, cfg, nil
	}
	if lastErr != nil {
		return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("failed to describe volcengine project %s: %w", workspaceID, lastErr)
	}
	return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("volcengine project %s not found", workspaceID)
}

func resolveBranchID(ctx context.Context, client *volcengine.Client, workspaceID, branchID string) (string, error) {
	branchID = strings.TrimSpace(branchID)
	if branchID != "" {
		return branchID, nil
	}
	defaultBranch, err := client.DescribeDefaultBranch(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	if defaultBranch.Branch.BranchID == "" {
		return "", errors.Errorf("failed to resolve default branch for workspace %s", workspaceID)
	}
	return defaultBranch.Branch.BranchID, nil
}

func confirm(ctx context.Context, action string) error {
	if shouldRun, err := utils.NewConsole().PromptYesNo(ctx, "Do you want to "+action+"?", false); err != nil {
		return err
	} else if !shouldRun {
		return errors.New(context.Canceled)
	}
	return nil
}

func outputPagesProjects(result volcengine.ListPagesProjectsResult) error {
	hasPreview := false
	for _, project := range result.Projects {
		if strings.TrimSpace(project.PreviewDomain) != "" {
			hasPreview = true
			break
		}
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		var table strings.Builder
		table.WriteString(`|PAGES PROJECT ID|NAME|STATUS|LATEST DEPLOY|PREVIEW DOMAIN|CUSTOM DOMAINS|HAS SUPABASE BINDING|BINDINGS|CREATED AT|UPDATED AT|
|-|-|-|-|-|-|-|-|-|-|
`)
		for _, project := range result.Projects {
			fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%t`|`%s`|`%s`|`%s`|\n",
				escape(project.PagesProjectID),
				escape(project.PagesProjectName),
				escape(project.Status),
				escape(project.LatestDeploy),
				escape(project.PreviewDomain),
				escape(strings.Join(project.CustomDomains, ",")),
				project.HasSupabaseBinding,
				escape(formatBindings(project.SupabaseBindings)),
				escape(project.CreateTime),
				escape(project.UpdateTime),
			)
		}
		if result.Total > len(result.Projects) {
			fmt.Fprintf(os.Stderr, "Showing %d of %d Pages projects. Use --limit and --offset to fetch more.\n", len(result.Projects), result.Total)
		}
		if err := utils.RenderTable(table.String()); err != nil {
			return err
		}
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	default:
		if err := utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result); err != nil {
			return err
		}
	}
	// Emit the bare-domain warning in every non-env mode (always to stderr, so `-o json`
	// stdout stays pure) — the skill steers models to `pages list -o json`, exactly the
	// path that would otherwise grab the non-openable base domain with no warning.
	if hasPreview {
		workspaceID, branchID := singlePagesBinding(result.Projects)
		printPagesBareDomainHint(os.Stderr, workspaceID, branchID)
	}
	return nil
}

func outputFastCreateResult(result FastCreateResult) error {
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		fmt.Fprintf(os.Stderr, "Fast create completed.\n")
		fmt.Fprintf(os.Stderr, "PagesProjectID: %s\n", utils.Aqua(result.PagesProjectID))
		fmt.Fprintf(os.Stderr, "PreviewDomain: %s\n", utils.Aqua(result.PreviewDomain))
		fmt.Fprintf(os.Stderr, "WorkspaceID: %s\n", utils.Aqua(result.WorkspaceID))
		fmt.Fprintf(os.Stderr, "BranchID: %s\n", utils.Aqua(result.BranchID))
		fmt.Fprintf(os.Stderr, "ProjectDeployResourceID: %s\n", utils.Aqua(result.ProjectDeployResourceID))
		fmt.Fprintf(os.Stderr, "DeployID: %s\n", utils.Aqua(result.DeployID))
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	default:
		if err := utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result); err != nil {
			return err
		}
	}
	if strings.TrimSpace(result.PreviewDomain) != "" {
		fmt.Fprintf(os.Stderr, "\n%s open %s to view the site. It is short-lived/single-use; for a fresh link re-run:\n  %s\n",
			utils.Yellow("Tip →"), utils.Aqua(result.PreviewDomain),
			utils.Aqua(pagesBindingCommand(result.WorkspaceID, result.BranchID)))
	}
	return nil
}

// pagesBindingCommand renders a `pages binding` invocation, filling in the workspace and
// branch when they are known and falling back to a placeholder otherwise. `pages binding`
// is the only command that returns the openable (short-lived, token-carrying) preview URL,
// so every preview-domain hint steers the caller here.
func pagesBindingCommand(workspaceID, branchID string) string {
	cmd := "byted-supabase-cli pages binding --workspace-id "
	if strings.TrimSpace(workspaceID) != "" {
		cmd += workspaceID
	} else {
		cmd += "<workspace-id>"
	}
	if strings.TrimSpace(branchID) != "" {
		cmd += " --branch-id " + branchID
	}
	return cmd + " -o json"
}

// printPagesDeployPreviewHint writes a next-step hint after `pages deploy`. The deploy
// command does not return the deployed site's URL — only `pages binding` resolves the
// (short-lived, single-use) PreviewDomain — so weaker agents otherwise finish a deploy
// without knowing the address to hand back. Callers pass os.Stderr, so the hint never
// pollutes `-o json` stdout yet still surfaces in an agent's captured output. `pages
// deploy` is account-level with no workspace in scope, so the workspace id is left as a
// placeholder for the caller to fill from the preceding `pages bind` step.
func printPagesDeployPreviewHint(w io.Writer) {
	fmt.Fprintf(w, "\n%s `pages deploy` does not return a site URL. Get the preview URL with:\n  %s\nthen open %s — it is short-lived/single-use, so share it immediately and re-run the command for a fresh link.\n",
		utils.Yellow("Next →"),
		utils.Aqua(pagesBindingCommand("", "")),
		utils.Bold("PagesProject.PreviewDomain"))
}

// printPagesBindNextStepsHint writes a hint after a successful `pages bind`: binding only
// associates the Pages project with the branch and injects env — nothing is served until
// a deploy, and neither bind nor deploy returns a site URL. It spells out the remaining
// chain (deploy → `pages binding` for the short-lived PreviewDomain) with the known
// workspace/branch/project pre-filled so weaker agents can continue without guessing.
func printPagesBindNextStepsHint(w io.Writer, pagesProjectID, workspaceID, branchID string) {
	deployCmd := fmt.Sprintf("byted-supabase-cli pages deploy %s --resource-id <resource-id>", pagesProjectID)
	fmt.Fprintf(w, "\n%s apply the binding by deploying, then read the (short-lived) preview URL:\n  %s\n  %s   (open %s)\n",
		utils.Yellow("Next →"), utils.Aqua(deployCmd), utils.Aqua(pagesBindingCommand(workspaceID, branchID)), utils.Bold("PagesProject.PreviewDomain"))
}

// printPagesBareDomainHint warns that a PreviewDomain printed by a list-style command
// (`pages list` / `pages deploy list`) is only the project's base domain: it carries no
// access token and will NOT open on its own. The openable preview URL is the short-lived,
// token-carrying PreviewDomain returned by `pages binding`. Weaker agents otherwise hand
// the bare domain to the user as the site address and it fails to load. workspace/branch
// are filled into the suggested command when unambiguous, else left as placeholders.
func printPagesBareDomainHint(w io.Writer, workspaceID, branchID string) {
	fmt.Fprintf(w, "\n%s the PreviewDomain listed here is the base domain only — no access token, so it will NOT open on its own. The openable link is the token-carrying %s from `pages binding`:\n  %s\nShort-lived/single-use — share it immediately and re-run for a fresh link.\n",
		utils.Yellow("Note →"), utils.Bold("PagesProject.PreviewDomain"), utils.Aqua(pagesBindingCommand(workspaceID, branchID)))
}

// singlePagesBinding returns the workspace/branch of the sole binding when the projects
// list holds exactly one project bound to exactly one branch, so the bare-domain hint can
// suggest a fully-formed `pages binding` command; otherwise it returns empty strings and
// the hint falls back to placeholders.
func singlePagesBinding(projects []volcengine.PagesProjectSummary) (workspaceID, branchID string) {
	if len(projects) != 1 || len(projects[0].SupabaseBindings) != 1 {
		return "", ""
	}
	binding := projects[0].SupabaseBindings[0]
	return binding.WorkspaceID, binding.BranchID
}

func outputEnvVars(envVars []volcengine.EnvKV) error {
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		var table strings.Builder
		table.WriteString(`|KEY|VALUE|
|-|-|
`)
		for _, envVar := range envVars {
			fmt.Fprintf(&table, "|`%s`|`%s`|\n", escape(envVar.Key), escape(envVar.Value))
		}
		return utils.RenderTable(table.String())
	case utils.OutputEnv:
		values := make(map[string]string, len(envVars))
		for _, envVar := range envVars {
			values[envVar.Key] = envVar.Value
		}
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, values)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, envVars)
}

func outputBinding(result volcengine.DescribePagesBindingResult) error {
	previewDomain := ""
	if result.PagesProject != nil {
		previewDomain = strings.TrimSpace(result.PagesProject.PreviewDomain)
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		if result.Binding == nil {
			fmt.Fprintln(os.Stderr, "No Pages binding found.")
			return nil
		}
		var table strings.Builder
		table.WriteString(`|WORKSPACE ID|BRANCH ID|PAGES PROJECT ID|PAGES PROJECT NAME|ENV VARS|CUSTOM PREFIX|FRAMEWORK PREFIX|PREVIEW DOMAIN|CUSTOM DOMAINS|
|-|-|-|-|-|-|-|-|-|
`)
		projectName := ""
		customDomains := ""
		if result.PagesProject != nil {
			projectName = result.PagesProject.PagesProjectName
			customDomains = strings.Join(result.PagesProject.CustomDomains, ",")
		}
		fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|\n",
			escape(result.Binding.WorkspaceID),
			escape(result.Binding.BranchID),
			escape(result.Binding.PagesProjectID),
			escape(projectName),
			escape(strings.Join(result.Binding.EnvVarNames, ",")),
			escape(result.Binding.CustomPrefix),
			escape(result.Binding.FrameworkPrefix),
			escape(previewDomain),
			escape(customDomains),
		)
		if err := utils.RenderTable(table.String()); err != nil {
			return err
		}
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	default:
		if err := utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result); err != nil {
			return err
		}
	}
	if previewDomain != "" {
		fmt.Fprintf(os.Stderr, "\n%s the %s returned here is the openable site URL — open it as-is (it carries a one-time access token; not the bare domain that `pages list` shows). Short-lived/single-use; re-run %s for a fresh link.\n",
			utils.Yellow("Tip →"), utils.Bold("PreviewDomain"), utils.Aqua("pages binding"))
	}
	return nil
}

func outputPagesDeploys(result volcengine.ListPagesDeployResult) error {
	hasPreview := false
	for _, deploy := range result.Deployments {
		if strings.TrimSpace(deploy.PreviewDomain) != "" {
			hasPreview = true
			break
		}
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		var table strings.Builder
		table.WriteString(`|ID|DEPLOY ID|PAGES PROJECT ID|STATUS|PROVIDER|RESOURCE ID|PREVIEW DOMAIN|CREATED AT|UPDATED AT|
|-|-|-|-|-|-|-|-|-|
`)
		for _, deploy := range result.Deployments {
			fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|\n",
				escape(deploy.ID),
				escape(deploy.DeployID),
				escape(deploy.PagesProjectID),
				escape(deploy.Status),
				escape(deploy.Provider),
				escape(deploy.ProjectDeployResourceID),
				escape(deploy.PreviewDomain),
				escape(deploy.CreateAt),
				escape(deploy.UpdateAt),
			)
		}
		if result.Total > len(result.Deployments) {
			nextOffset := len(result.Deployments)
			if result.PageNumber > 0 && result.PageSize > 0 {
				nextOffset = result.PageNumber * result.PageSize
			}
			fmt.Fprintf(os.Stderr, "Showing %d of %d Pages deployments. Use --limit and --offset %d to fetch more.\n", len(result.Deployments), result.Total, nextOffset)
		}
		if err := utils.RenderTable(table.String()); err != nil {
			return err
		}
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	default:
		if err := utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result); err != nil {
			return err
		}
	}
	if hasPreview {
		printPagesBareDomainHint(os.Stderr, "", "")
	}
	return nil
}

func maskedEnvVars(envVars []volcengine.EnvKV) []volcengine.EnvKV {
	masked := make([]volcengine.EnvKV, len(envVars))
	for i, envVar := range envVars {
		masked[i] = envVar
		if envVar.Value != "" {
			masked[i].Value = "******"
		}
	}
	return masked
}

func formatBindings(bindings []volcengine.PagesBinding) string {
	if len(bindings) == 0 {
		return ""
	}
	parts := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		part := binding.WorkspaceID + "/" + binding.BranchID
		if binding.CustomPrefix != "" {
			part += " custom-prefix=" + binding.CustomPrefix
		}
		if binding.FrameworkPrefix != "" {
			part += " framework-prefix=" + binding.FrameworkPrefix
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ",")
}

func escape(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}
