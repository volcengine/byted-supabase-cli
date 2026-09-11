## Auth

- 先执行 `auth config get`，只读取所需 key。
- 修改前保留非敏感旧值，修改后用 `auth config get --key <key>` 验证。
- Hooks 和第三方 provider 修改前先读取当前配置，保留未修改字段。

## Storage

- 列举和下载优先使用只读命令。
- 上传、覆盖、移动、删除前确认 bucket 和对象路径。
- `storage rm`、递归复制和批量操作必须明确确认。
- Storage upsert 可能同时需要 INSERT、SELECT、UPDATE 权限。

## Edge Functions 和 Secrets

- 用 `functions list`、`functions download` 检查现状。
- `functions deploy` 前确认函数名、代码来源、runtime 和是否覆盖线上版本。
- 用 `secrets list/set/unset` 管理 secrets，绝不打印 secret value。
- 部署后再次执行 `functions list` 或只读检查确认版本和状态。
