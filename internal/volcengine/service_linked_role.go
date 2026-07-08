// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"fmt"
	"strings"

	"github.com/volcengine/volcengine-go-sdk/volcengine/universal"
)

const (
	// IAM is a separate, GLOBAL Volcengine service (not aidap). CheckServiceLinkedRole
	// lives under API version 2018-01-01 and is not in the strong-typed Go SDK, so it is
	// driven through the generic universal client.
	iamServiceName    = "iam"
	iamAPIVersion     = "2018-01-01"
	iamGlobalEndpoint = "https://iam.volcengineapi.com"

	// aidapServiceLinkedRoleService is the ServiceName *parameter* passed to
	// CheckServiceLinkedRole — i.e. the service whose linked role we verify.
	aidapServiceLinkedRoleService = "aidap"

	// AIDAPServiceLinkedRoleName is the IAM service-linked role that AIDAP requires
	// before a workspace can be created.
	AIDAPServiceLinkedRoleName = "ServiceRoleForAIDAP"

	// roleNotExistErrorCode is the IAM error code returned by CheckServiceLinkedRole
	// when the service-linked role has not been authorized for the account.
	roleNotExistErrorCode = "RoleNotExist"

	// aidapConsoleAuthURLTemplate is the AIDAP console entry (region-templated; %s is
	// the region) that prompts the user to authorize the service-linked role.
	//
	// TODO(bp-adaptation): the public-cloud console host is hardcoded here. When adapting
	// to BP / other deployment sites (e.g. boe / i18n / private deployments, mirroring an
	// alternative control-plane backend's multi-site handling), make this host site-aware
	// instead of always pointing at console.volcengine.com.
	aidapConsoleAuthURLTemplate = "https://console.volcengine.com/aidap/region:aidap+%s/workspaces?scene=create"
)

// ServiceLinkedRoleNotAssociatedError is returned before workspace creation when the
// AIDAP service-linked role has not been authorized for the account. Both the CLI and
// MCP create paths surface its message — Chinese authorization guidance plus the
// region-scoped console link — so the user can complete the one-time setup.
type ServiceLinkedRoleNotAssociatedError struct {
	Region string
}

// AuthorizationURL returns the AIDAP console entry that prompts the user to authorize
// the service-linked role, scoped to the workspace's region.
func (e *ServiceLinkedRoleNotAssociatedError) AuthorizationURL() string {
	region := strings.TrimSpace(e.Region)
	if region == "" {
		region = DefaultRegion
	}
	return fmt.Sprintf(aidapConsoleAuthURLTemplate, region)
}

func (e *ServiceLinkedRoleNotAssociatedError) Error() string {
	return "无法创建工作区:尚未授权 AIDAP 服务关联角色(" + AIDAPServiceLinkedRoleName + ")。首次使用请先完成授权:\n" +
		"  • 如果您是主账号:请为账号授权 " + AIDAPServiceLinkedRoleName + " 角色;\n" +
		"  • 如果您是子账号:请联系您组织的主账号授权 " + AIDAPServiceLinkedRoleName + " 角色，并为您再授予 IAM 的 AIDAPFullAccess 预设策略(或包含 AIDAP 权限点的自定义策略)。\n" +
		"完成授权后即可创建工作区。授权入口(请在浏览器中打开):\n  " + e.AuthorizationURL()
}

// CheckAIDAPServiceLinkedRole verifies that the AIDAP service-linked role is authorized
// for the current account, gating workspace creation. It returns:
//
//   - *ServiceLinkedRoleNotAssociatedError when IAM reports the role is missing
//     (error code RoleNotExist) — callers MUST block creation and surface the message;
//   - nil when the role exists, OR when the check is inconclusive (any other error, e.g.
//     the caller lacks iam:CheckServiceLinkedRole permission, the route is unavailable in
//     proxy mode, or a transient failure). The check is a best-effort guard and must not
//     introduce a new failure mode that blocks an otherwise-valid creation.
func (c *Client) CheckAIDAPServiceLinkedRole() error {
	input := map[string]interface{}{"ServiceName": aidapServiceLinkedRoleService}
	output := map[string]interface{}{}
	err := c.iam.DoCallWithType(universal.RequestUniversal{
		ServiceName: iamServiceName,
		Action:      "CheckServiceLinkedRole",
		Version:     iamAPIVersion,
		HttpMethod:  universal.GET,
	}, &input, &output)
	if err == nil {
		return nil
	}
	if VolcengineErrorDetailsFromErr(err).ErrorCode == roleNotExistErrorCode {
		return &ServiceLinkedRoleNotAssociatedError{Region: c.cfg.Region}
	}
	// Inconclusive check: do not block creation on a best-effort gate.
	return nil
}
