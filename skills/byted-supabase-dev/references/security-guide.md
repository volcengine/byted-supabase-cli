# 安全基线

## 凭据

- `anon key` 可用于公开数据面请求，但不代表管理权限。
- `service-role key` 只用于受控后端或 CLI 操作，不能进入前端、浏览器环境变量或 bundle。
- database URL 通常包含密码；日志和报告中使用 `<REDACTED>`。
- 不把 key、密码或 token 写入 Git、Skill、测试 fixture、shell 脚本或临时报告。

## 数据与 RLS

- 对外暴露的表应启用 RLS，并按 `anon`、`authenticated` 和所有权设计 policy。
- 不用 `user_metadata` 作为授权依据；用户可修改的数据不能作为权限来源。
- `UPDATE` policy 同时检查 `USING` 和 `WITH CHECK`，并确认存在必要的 SELECT policy。
- 不用 service-role key 绕过 RLS 来验证普通用户行为。

## 写操作

以下操作必须先向用户说明影响并取得确认：

- 数据库 `UPDATE`、`DELETE`、`DROP`、`ALTER` 和批量迁移；
- Storage 上传、覆盖、移动、递归删除；
- Functions 部署；
- Secrets 设置和删除；
- Auth、Hooks、第三方 provider 配置修改；
- Pages 创建和部署；
- MCP 写工具。

## 网络和输出

- 优先使用 CLI 的 HTTPS 数据面请求和 context 超时。
- 真实探测只使用只读 endpoint，不为验证权限执行写操作。
- 错误中保留必要的 HTTP 状态码，但脱敏 URL、host、key、密码和 token。
- 不把完整 endpoint 当作敏感 key 输出；报告中仍应按项目要求使用 `<PUBLIC_ENDPOINT>`。
