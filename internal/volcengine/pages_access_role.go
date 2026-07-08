// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"fmt"

	"github.com/volcengine/volcengine-go-sdk/volcengine/universal"
)

const (
	// dcdnAccessRoleName is the IAM role that Pages (IGA Pages / DCDN-backed static
	// hosting) requires: it lets the dcdn service assume into the account to serve a
	// deployed frontend. The frontend checks the same role via IAM GetRole before a
	// deploy. Reuses iamServiceName / iamAPIVersion / roleNotExistErrorCode and the
	// global-endpoint IAM universal client from service_linked_role.go.
	dcdnAccessRoleName = "DCDNAccessAIDPRole"

	// dcdnAccessRolePolicyName is the system policy attached to dcdnAccessRoleName, per
	// the IAM "普通 Service Role"(方案二) naming convention ${service}${scenario}RolePolicy
	// (mirrors the role's ${service}${scenario}Role). It is the policy1_1 query param the
	// authorization page attaches when creating the role.
	dcdnAccessRolePolicyName = "DCDNAccessAIDPRolePolicy"

	// dcdnAccessRoleServiceCode is the requesting service code (the role's trust
	// principal). Used as the ServiceName query param (display + presence check only).
	dcdnAccessRoleServiceCode = "dcdn"

	// pagesAccessRoleAuthHost is the prod (volcengine.com) console host that serves the
	// shared IAM custom-role authorization page. BOE/stable use different hosts; we target
	// prod (官网) only.
	pagesAccessRoleAuthHost = "https://console.volcengine.com"
)

// PagesAccessRoleNotAssociatedError is returned before a Pages (frontend) deployment when
// the DCDNAccessAIDPRole role has not been authorized for the account. Both the CLI and MCP
// Pages deployment paths surface its message — Chinese authorization guidance plus the
// console link — so the user can complete the one-time setup.
type PagesAccessRoleNotAssociatedError struct{}

// AuthorizationURL returns the shared IAM custom-role authorization page (方案二), with
// the role + system policy passed as query params per the "IAM 自定义Role跨服务授权前端
// 接入文档" spec: /iam/service/attach_custom_role?ServiceName=&role1=&policy1_1=. Opening
// it creates DCDNAccessAIDPRole and attaches its policy in one step.
func (e *PagesAccessRoleNotAssociatedError) AuthorizationURL() string {
	return fmt.Sprintf("%s/iam/service/attach_custom_role?ServiceName=%s&role1=%s&policy1_1=%s",
		pagesAccessRoleAuthHost, dcdnAccessRoleServiceCode, dcdnAccessRoleName, dcdnAccessRolePolicyName)
}

func (e *PagesAccessRoleNotAssociatedError) Error() string {
	return "无法部署前端(Pages):当前账号尚未授权 Pages 所需的服务角色(" + dcdnAccessRoleName + ")。首次使用前端部署请先完成授权:\n" +
		"  • 如果您是主账号:请为账号授权 " + dcdnAccessRoleName + " 角色;\n" +
		"  • 如果您是子账号:请联系您组织的主账号授权 " + dcdnAccessRoleName + " 角色。\n" +
		"完成授权后即可部署。授权入口(请在浏览器中打开,将自动创建角色并关联策略):\n  " + e.AuthorizationURL()
}

// CheckPagesAccessRole verifies that the DCDNAccessAIDPRole role is authorized for the
// current account, gating Pages (frontend) deployment. It returns:
//
//   - *PagesAccessRoleNotAssociatedError when IAM reports the role is missing (GetRole
//     error code RoleNotExist) — callers MUST block the deployment and surface the message;
//   - nil when the role exists, OR when the check is inconclusive (any other error, e.g. the
//     caller lacks iam:GetRole permission, the route is unavailable in proxy mode, or a
//     transient failure). The check is a best-effort guard and must not introduce a new
//     failure mode that blocks an otherwise-valid deployment.
func (c *Client) CheckPagesAccessRole() error {
	input := map[string]interface{}{"RoleName": dcdnAccessRoleName}
	output := map[string]interface{}{}
	err := c.iam.DoCallWithType(universal.RequestUniversal{
		ServiceName: iamServiceName,
		Action:      "GetRole",
		Version:     iamAPIVersion,
		HttpMethod:  universal.GET,
	}, &input, &output)
	if err == nil {
		return nil
	}
	if VolcengineErrorDetailsFromErr(err).ErrorCode == roleNotExistErrorCode {
		return &PagesAccessRoleNotAssociatedError{}
	}
	// Inconclusive check: do not block deployment on a best-effort gate.
	return nil
}
