// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"github.com/go-errors/errors"
	"github.com/volcengine/volcengine-go-sdk/volcengine/universal"
)

const (
	aidapServiceName = "aidap"
	aidapAPIVersion  = "2025-10-01"
	pagesServiceName = "dcdn"
	pagesAPIVersion  = "2021-04-01"
	pagesRegion      = "cn-north-1"
)

type PagesBinding struct {
	WorkspaceID     string   `json:"WorkspaceId" toml:"workspace_id" yaml:"workspace_id"`
	BranchID        string   `json:"BranchId" toml:"branch_id" yaml:"branch_id"`
	PagesProjectID  string   `json:"PagesProjectId" toml:"pages_project_id" yaml:"pages_project_id"`
	EnvVarNames     []string `json:"EnvVarNames,omitempty" toml:"env_var_names,omitempty" yaml:"env_var_names,omitempty"`
	CustomPrefix    string   `json:"CustomPrefix,omitempty" toml:"custom_prefix,omitempty" yaml:"custom_prefix,omitempty"`
	FrameworkPrefix string   `json:"FrameworkPrefix,omitempty" toml:"framework_prefix,omitempty" yaml:"framework_prefix,omitempty"`
	CreateTime      string   `json:"CreateTime,omitempty" toml:"create_time,omitempty" yaml:"create_time,omitempty"`
	UpdateTime      string   `json:"UpdateTime,omitempty" toml:"update_time,omitempty" yaml:"update_time,omitempty"`
}

type PagesProjectSummary struct {
	PagesProjectID     string         `json:"PagesProjectId" toml:"pages_project_id" yaml:"pages_project_id"`
	PagesProjectName   string         `json:"PagesProjectName" toml:"pages_project_name" yaml:"pages_project_name"`
	Status             string         `json:"Status,omitempty" toml:"status,omitempty" yaml:"status,omitempty"`
	LatestDeploy       string         `json:"LatestDeploy,omitempty" toml:"latest_deploy,omitempty" yaml:"latest_deploy,omitempty"`
	PreviewDomain      string         `json:"PreviewDomain,omitempty" toml:"preview_domain,omitempty" yaml:"preview_domain,omitempty"`
	CustomDomains      []string       `json:"CustomDomains,omitempty" toml:"custom_domains,omitempty" yaml:"custom_domains,omitempty"`
	CreateTime         string         `json:"CreateTime,omitempty" toml:"create_time,omitempty" yaml:"create_time,omitempty"`
	UpdateTime         string         `json:"UpdateTime,omitempty" toml:"update_time,omitempty" yaml:"update_time,omitempty"`
	SupabaseBindings   []PagesBinding `json:"SupabaseBindings,omitempty" toml:"supabase_bindings,omitempty" yaml:"supabase_bindings,omitempty"`
	HasSupabaseBinding bool           `json:"HasSupabaseBinding,omitempty" toml:"has_supabase_binding,omitempty" yaml:"has_supabase_binding,omitempty"`
}

type EnvKV struct {
	Key   string `json:"Key" toml:"key" yaml:"key"`
	Value string `json:"Value" toml:"value" yaml:"value"`
}

type VolcSupabaseFilter struct {
	WorkspaceID string `json:"WorkspaceId,omitempty"`
	BranchID    string `json:"BranchId,omitempty"`
}

type ListPagesProjectsParams struct {
	Name      string
	Limit     int
	Offset    int
	AccountID string
	Filter    VolcSupabaseFilter
}

type ListPagesProjectsResult struct {
	Total      int                   `json:"Total" toml:"total" yaml:"total"`
	PageNumber int                   `json:"PageNumber,omitempty" toml:"page_number,omitempty" yaml:"page_number,omitempty"`
	PageSize   int                   `json:"PageSize,omitempty" toml:"page_size,omitempty" yaml:"page_size,omitempty"`
	Projects   []PagesProjectSummary `json:"Projects" toml:"projects" yaml:"projects"`
}

type DescribePagesEnvVarsResult struct {
	EnvVars []EnvKV `json:"EnvVars" toml:"env_vars" yaml:"env_vars"`
}

type DescribePagesBindingResult struct {
	Binding      *PagesBinding        `json:"Binding,omitempty" toml:"binding,omitempty" yaml:"binding,omitempty"`
	PagesProject *PagesProjectSummary `json:"PagesProject,omitempty" toml:"pages_project,omitempty" yaml:"pages_project,omitempty"`
}

type BindPagesProjectParams struct {
	WorkspaceID     string
	BranchID        string
	PagesProjectID  string
	CustomPrefix    string
	FrameworkPrefix string
}

type SyncPagesDeployEnvVarsParams struct {
	WorkspaceID    string
	BranchID       string
	PagesProjectID string
}

type UnbindPagesProjectParams struct {
	WorkspaceID string
	BranchID    string
}

type PagesProjectParam struct {
	Framework     string `json:"Framework,omitempty" toml:"framework,omitempty" yaml:"framework,omitempty"`
	RootDir       string `json:"RootDir,omitempty" toml:"root_dir,omitempty" yaml:"root_dir,omitempty"`
	OutputDir     string `json:"OutputDir,omitempty" toml:"output_dir,omitempty" yaml:"output_dir,omitempty"`
	BuildCmd      string `json:"BuildCmd,omitempty" toml:"build_cmd,omitempty" yaml:"build_cmd,omitempty"`
	InstallCmd    string `json:"InstallCmd,omitempty" toml:"install_cmd,omitempty" yaml:"install_cmd,omitempty"`
	NodejsVersion string `json:"NodejsVersion,omitempty" toml:"nodejs_version,omitempty" yaml:"nodejs_version,omitempty"`
}

type CreatePagesProjectParams struct {
	Name                    string
	Provider                string
	Scope                   string
	ProjectDeployResourceID string
	ProjectParam            PagesProjectParam
	NoDeploy                bool
}

type CreatePagesProjectResult struct {
	ProjectID               string            `json:"ProjectID" toml:"project_id" yaml:"project_id"`
	DeployID                string            `json:"DeployID,omitempty" toml:"deploy_id,omitempty" yaml:"deploy_id,omitempty"`
	PagesProjectName        string            `json:"Name,omitempty" toml:"pages_project_name,omitempty" yaml:"pages_project_name,omitempty"`
	Scope                   string            `json:"Scope,omitempty" toml:"scope,omitempty" yaml:"scope,omitempty"`
	Provider                string            `json:"Provider,omitempty" toml:"provider,omitempty" yaml:"provider,omitempty"`
	Status                  string            `json:"Status,omitempty" toml:"status,omitempty" yaml:"status,omitempty"`
	LatestDeploy            string            `json:"LatestDeploy,omitempty" toml:"latest_deploy,omitempty" yaml:"latest_deploy,omitempty"`
	ProjectDeployResourceID string            `json:"ProjectDeployResourceID,omitempty" toml:"project_deploy_resource_id,omitempty" yaml:"project_deploy_resource_id,omitempty"`
	ProjectParam            PagesProjectParam `json:"ProjectParam,omitempty" toml:"project_param,omitempty" yaml:"project_param,omitempty"`
	NoDeploy                bool              `json:"NoDeploy,omitempty" toml:"no_deploy,omitempty" yaml:"no_deploy,omitempty"`
	CreateAt                string            `json:"CreateAt,omitempty" toml:"create_at,omitempty" yaml:"create_at,omitempty"`
	UpdateAt                string            `json:"UpdateAt,omitempty" toml:"update_at,omitempty" yaml:"update_at,omitempty"`
}

type CreatePagesDeployParams struct {
	PagesProjectID          string
	ProjectDeployResourceID string
}

type CreatePagesDeployResult struct {
	PagesProjectID          string `json:"ProjectID" toml:"project_id" yaml:"project_id"`
	DeployID                string `json:"DeployID" toml:"deploy_id" yaml:"deploy_id"`
	Provider                string `json:"Provider" toml:"provider" yaml:"provider"`
	ProjectDeployResourceID string `json:"ProjectDeployResourceID" toml:"project_deploy_resource_id" yaml:"project_deploy_resource_id"`
}

type ListPagesDeployParams struct {
	PagesProjectID string
	Limit          int
	Offset         int
}

type PagesDeploySummary struct {
	ID                      string `json:"ID,omitempty" toml:"id,omitempty" yaml:"id,omitempty"`
	DeployID                string `json:"DeployID" toml:"deploy_id" yaml:"deploy_id"`
	PagesProjectID          string `json:"ProjectID,omitempty" toml:"project_id,omitempty" yaml:"project_id,omitempty"`
	Status                  string `json:"Status,omitempty" toml:"status,omitempty" yaml:"status,omitempty"`
	Provider                string `json:"Provider,omitempty" toml:"provider,omitempty" yaml:"provider,omitempty"`
	ProjectDeployResourceID string `json:"ProjectDeployResourceID,omitempty" toml:"project_deploy_resource_id,omitempty" yaml:"project_deploy_resource_id,omitempty"`
	PreviewDomain           string `json:"PreviewDomain,omitempty" toml:"preview_domain,omitempty" yaml:"preview_domain,omitempty"`
	CreateAt                string `json:"CreateAt,omitempty" toml:"create_at,omitempty" yaml:"create_at,omitempty"`
	UpdateAt                string `json:"UpdateAt,omitempty" toml:"update_at,omitempty" yaml:"update_at,omitempty"`
}

type ListPagesDeployResult struct {
	Total       int                  `json:"Total" toml:"total" yaml:"total"`
	PageNumber  int                  `json:"PageNumber,omitempty" toml:"page_number,omitempty" yaml:"page_number,omitempty"`
	PageSize    int                  `json:"PageSize,omitempty" toml:"page_size,omitempty" yaml:"page_size,omitempty"`
	Deployments []PagesDeploySummary `json:"Deployments" toml:"deployments" yaml:"deployments"`
}

type UploadPagesResourcesPostURLParams struct {
	FileName string
}

type UploadPagesResourcesPostURLResult struct {
	ProjectDeployResourceID string `json:"ProjectDeployResourceID" toml:"project_deploy_resource_id" yaml:"project_deploy_resource_id"`
	TosUploadURL            string `json:"TosUploadURL" toml:"-" yaml:"-"`
}

type listPagesProjectReq struct {
	Name               string              `json:"Name,omitempty"`
	PageSize           int                 `json:"PageSize,omitempty"`
	PageNumber         int                 `json:"PageNumber,omitempty"`
	AccountID          string              `json:"AccountId,omitempty"`
	VolcSupabaseFilter *VolcSupabaseFilter `json:"VolcSupabaseFilter,omitempty"`
}

type describePagesEnvVarsReq struct {
	WorkspaceID string `json:"WorkspaceId"`
	BranchID    string `json:"BranchId"`
}

type bindPagesProjectReq struct {
	WorkspaceID     string `json:"WorkspaceId"`
	BranchID        string `json:"BranchId"`
	PagesProjectID  string `json:"PagesProjectId"`
	CustomPrefix    string `json:"CustomPrefix,omitempty"`
	FrameworkPrefix string `json:"FrameworkPrefix,omitempty"`
}

type syncPagesDeployEnvVarsReq struct {
	WorkspaceID    string `json:"WorkspaceId"`
	BranchID       string `json:"BranchId"`
	PagesProjectID string `json:"PagesProjectId"`
}

type unbindPagesProjectReq struct {
	WorkspaceID string `json:"WorkspaceId"`
	BranchID    string `json:"BranchId"`
}

type createPagesProjectReq struct {
	Name                    string            `json:"Name"`
	Scope                   string            `json:"Scope,omitempty"`
	Provider                string            `json:"Provider"`
	ProjectDeployResourceID string            `json:"ProjectDeployResourceID,omitempty"`
	ProjectParam            PagesProjectParam `json:"ProjectParam,omitempty"`
	NoDeploy                bool              `json:"NoDeploy,omitempty"`
}

type createPagesDeployReq struct {
	ProjectID               string `json:"ProjectID"`
	Provider                string `json:"Provider"`
	ProjectDeployResourceID string `json:"ProjectDeployResourceID"`
}

type listPagesDeployReq struct {
	ProjectID  string `json:"ProjectID"`
	PageSize   int    `json:"PageSize,omitempty"`
	PageNumber int    `json:"PageNumber,omitempty"`
}

type listPagesDeployResp struct {
	Total      int                  `json:"Total,omitempty"`
	PageNumber int                  `json:"PageNumber,omitempty"`
	PageSize   int                  `json:"PageSize,omitempty"`
	Infos      []PagesDeploySummary `json:"Infos,omitempty"`
}

type uploadPagesResourcesPostURLReq struct {
	FileName string `json:"FileName"`
}

func (c *Client) ListPagesProjects(params ListPagesProjectsParams) (ListPagesProjectsResult, error) {
	if params.Limit < 0 || params.Offset < 0 {
		return ListPagesProjectsResult{}, errors.New("limit and offset must be non-negative")
	}
	if params.Limit > 0 && params.Offset%params.Limit != 0 {
		return ListPagesProjectsResult{}, errors.New("--offset must be a multiple of --limit for Pages pagination")
	}
	req := listPagesProjectReq{
		Name:       params.Name,
		PageSize:   params.Limit,
		PageNumber: 1,
		AccountID:  params.AccountID,
	}
	if params.Limit > 0 {
		req.PageNumber = params.Offset/params.Limit + 1
	}
	if params.Filter.WorkspaceID != "" || params.Filter.BranchID != "" {
		req.VolcSupabaseFilter = &params.Filter
	}
	var out ListPagesProjectsResult
	if err := c.callAIDAPUniversal("ListPagesProject", &req, &out); err != nil {
		return ListPagesProjectsResult{}, errors.Errorf("failed to call volcengine ListPagesProject: %w", err)
	}
	return out, nil
}

func (c *Client) DescribeSupabaseDeployEnvVars(workspaceID, branchID string) (DescribePagesEnvVarsResult, error) {
	var out DescribePagesEnvVarsResult
	if err := c.callAIDAPUniversal("DescribeSupabaseDeployEnvVars", &describePagesEnvVarsReq{
		WorkspaceID: workspaceID,
		BranchID:    branchID,
	}, &out); err != nil {
		return DescribePagesEnvVarsResult{}, errors.Errorf("failed to call volcengine DescribeSupabaseDeployEnvVars: %w", err)
	}
	return out, nil
}

func (c *Client) BindPagesProject(params BindPagesProjectParams) error {
	if err := c.callAIDAPUniversal("BindPagesProject", &bindPagesProjectReq{
		WorkspaceID:     params.WorkspaceID,
		BranchID:        params.BranchID,
		PagesProjectID:  params.PagesProjectID,
		CustomPrefix:    params.CustomPrefix,
		FrameworkPrefix: params.FrameworkPrefix,
	}, &struct{}{}); err != nil {
		return errors.Errorf("failed to call volcengine BindPagesProject: %w", err)
	}
	return nil
}

func (c *Client) DescribePagesBinding(workspaceID, branchID string) (DescribePagesBindingResult, error) {
	var out DescribePagesBindingResult
	if err := c.callAIDAPUniversal("DescribePagesBinding", &describePagesEnvVarsReq{
		WorkspaceID: workspaceID,
		BranchID:    branchID,
	}, &out); err != nil {
		return DescribePagesBindingResult{}, errors.Errorf("failed to call volcengine DescribePagesBinding: %w", err)
	}
	return out, nil
}

func (c *Client) SyncPagesDeployEnvVars(params SyncPagesDeployEnvVarsParams) error {
	if err := c.callAIDAPUniversal("SyncPagesDeployEnvVars", &syncPagesDeployEnvVarsReq{
		WorkspaceID:    params.WorkspaceID,
		BranchID:       params.BranchID,
		PagesProjectID: params.PagesProjectID,
	}, &struct{}{}); err != nil {
		return errors.Errorf("failed to call volcengine SyncPagesDeployEnvVars: %w", err)
	}
	return nil
}

func (c *Client) UnbindPagesProject(params UnbindPagesProjectParams) error {
	if err := c.callAIDAPUniversal("UnbindPagesProject", &unbindPagesProjectReq{
		WorkspaceID: params.WorkspaceID,
		BranchID:    params.BranchID,
	}, &struct{}{}); err != nil {
		return errors.Errorf("failed to call volcengine UnbindPagesProject: %w", err)
	}
	return nil
}

func (c *Client) CreatePagesProject(params CreatePagesProjectParams) (CreatePagesProjectResult, error) {
	if params.Name == "" {
		return CreatePagesProjectResult{}, errors.New("missing pages project name")
	}
	if params.Provider == "" {
		return CreatePagesProjectResult{}, errors.New("missing pages project provider")
	}
	if params.Provider != "upload_v2" {
		return CreatePagesProjectResult{}, errors.New("only upload_v2 provider is currently supported")
	}
	if params.Provider == "upload_v2" && params.ProjectDeployResourceID == "" {
		return CreatePagesProjectResult{}, errors.New("missing pages deploy resource id")
	}
	var out CreatePagesProjectResult
	if err := c.callPagesUniversal("CreatePagesProject", &createPagesProjectReq{
		Name:                    params.Name,
		Scope:                   params.Scope,
		Provider:                params.Provider,
		ProjectDeployResourceID: params.ProjectDeployResourceID,
		ProjectParam:            params.ProjectParam,
		NoDeploy:                params.NoDeploy,
	}, &out); err != nil {
		return CreatePagesProjectResult{}, errors.Errorf("failed to call volcengine CreatePagesProject: %w", err)
	}
	if out.PagesProjectName == "" {
		out.PagesProjectName = params.Name
	}
	if out.Scope == "" {
		out.Scope = params.Scope
	}
	if out.Provider == "" {
		out.Provider = params.Provider
	}
	if out.ProjectDeployResourceID == "" {
		out.ProjectDeployResourceID = params.ProjectDeployResourceID
	}
	if params.NoDeploy {
		out.NoDeploy = true
	}
	if out.ProjectParam == (PagesProjectParam{}) {
		out.ProjectParam = params.ProjectParam
	}
	return out, nil
}

func (c *Client) CreatePagesDeploy(params CreatePagesDeployParams) (CreatePagesDeployResult, error) {
	if params.PagesProjectID == "" {
		return CreatePagesDeployResult{}, errors.New("missing pages project id")
	}
	if params.ProjectDeployResourceID == "" {
		return CreatePagesDeployResult{}, errors.New("missing pages deploy resource id")
	}
	var out CreatePagesDeployResult
	if err := c.callPagesUniversal("CreatePagesDeploy", &createPagesDeployReq{
		ProjectID:               params.PagesProjectID,
		Provider:                "upload_v2",
		ProjectDeployResourceID: params.ProjectDeployResourceID,
	}, &out); err != nil {
		return CreatePagesDeployResult{}, errors.Errorf("failed to call volcengine CreatePagesDeploy: %w", err)
	}
	if out.DeployID == "" {
		return CreatePagesDeployResult{}, errors.New("CreatePagesDeploy returned empty DeployID")
	}
	if out.PagesProjectID == "" {
		out.PagesProjectID = params.PagesProjectID
	}
	if out.Provider == "" {
		out.Provider = "upload_v2"
	}
	if out.ProjectDeployResourceID == "" {
		out.ProjectDeployResourceID = params.ProjectDeployResourceID
	}
	return out, nil
}

func (c *Client) ListPagesDeploy(params ListPagesDeployParams) (ListPagesDeployResult, error) {
	if params.PagesProjectID == "" {
		return ListPagesDeployResult{}, errors.New("missing pages project id")
	}
	if params.Limit < 0 || params.Offset < 0 {
		return ListPagesDeployResult{}, errors.New("limit and offset must be non-negative")
	}
	if params.Limit > 0 && params.Offset%params.Limit != 0 {
		return ListPagesDeployResult{}, errors.New("--offset must be a multiple of --limit for Pages deploy pagination")
	}
	req := listPagesDeployReq{
		ProjectID:  params.PagesProjectID,
		PageSize:   params.Limit,
		PageNumber: 1,
	}
	if params.Limit > 0 {
		req.PageNumber = params.Offset/params.Limit + 1
	}
	var out listPagesDeployResp
	if err := c.callPagesUniversal("ListPagesDeploy", &req, &out); err != nil {
		return ListPagesDeployResult{}, errors.Errorf("failed to call volcengine ListPagesDeploy: %w", err)
	}
	for i := range out.Infos {
		if out.Infos[i].PagesProjectID == "" {
			out.Infos[i].PagesProjectID = params.PagesProjectID
		}
	}
	return ListPagesDeployResult{
		Total:       out.Total,
		PageNumber:  out.PageNumber,
		PageSize:    out.PageSize,
		Deployments: out.Infos,
	}, nil
}

func (c *Client) UploadPagesResourcesPostURL(params UploadPagesResourcesPostURLParams) (UploadPagesResourcesPostURLResult, error) {
	if params.FileName == "" {
		return UploadPagesResourcesPostURLResult{}, errors.New("missing pages resource file name")
	}
	var out UploadPagesResourcesPostURLResult
	if err := c.callPagesUniversal("UploadPagesResourcesPostURL", &uploadPagesResourcesPostURLReq{
		FileName: params.FileName,
	}, &out); err != nil {
		return UploadPagesResourcesPostURLResult{}, errors.Errorf("failed to call volcengine UploadPagesResourcesPostURL: %w", err)
	}
	if out.ProjectDeployResourceID == "" {
		return UploadPagesResourcesPostURLResult{}, errors.New("UploadPagesResourcesPostURL returned empty ProjectDeployResourceID")
	}
	if out.TosUploadURL == "" {
		return UploadPagesResourcesPostURLResult{}, errors.New("UploadPagesResourcesPostURL returned empty TosUploadURL")
	}
	return out, nil
}

func (c *Client) callAIDAPUniversal(action string, input, output interface{}) error {
	// Transitional path: Pages actions are in IDL but not yet in the released
	// strong-typed Go SDK. Replace this with generated SDK methods once available.
	return c.uni.DoCallWithType(universal.RequestUniversal{
		ServiceName: aidapServiceName,
		Action:      action,
		Version:     aidapAPIVersion,
		HttpMethod:  universal.POST,
		ContentType: universal.ApplicationJSON,
	}, input, output)
}

func (c *Client) callPagesUniversal(action string, input, output interface{}) error {
	// Transitional path: Pages OpenAPI actions are not in the released
	// strong-typed Go SDK yet. Replace this with generated SDK methods once available.
	return c.pagesUni.DoCallWithType(universal.RequestUniversal{
		ServiceName: pagesServiceName,
		Action:      action,
		Version:     pagesAPIVersion,
		HttpMethod:  universal.POST,
		ContentType: universal.ApplicationJSON,
	}, input, output)
}
