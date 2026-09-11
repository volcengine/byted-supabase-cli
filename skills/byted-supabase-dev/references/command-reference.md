## 常用命令

| 用户目标 | 命令 |
|---|---|
| 查看实例和凭据状态 | `link status -o json`、`configure get` |
| Auth 配置 | `auth config get`、`auth config set` |
| Auth Hooks / 第三方登录 | `auth hooks ...`、`auth third-party ...` |
| 数据库查询 | `db query "<read-only SQL>"` |
| 数据库查询和巡检 | `db query ...`、`db advisors` |
| 生成类型 | `gen types typescript` |
| Storage | `storage buckets ...`、`storage ls`、`storage cp`、`storage mv`、`storage rm` |
| Edge Functions | `functions list`、`functions download`、`functions new`、`functions deploy` |
| Function Secrets | `secrets list`、`secrets set`、`secrets unset` |
| Pages | `pages list`、`pages binding`、`pages create`、`pages upload`、`pages deploy` |
| Pages 快速流程 | `pages fast create <name> --file-path <path>` |
| MCP | `mcp serve` |

始终以当前二进制帮助为准：

```bash
byted-supabase-cli <command> --help
```

不要使用 `projects`、`branches`、`computes` 等 Workspace 资源管理命令处理实例内部资源。
