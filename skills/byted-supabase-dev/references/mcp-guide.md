# MCP 指南

## 启动

先查看当前二进制支持的参数：

```bash
byted-supabase-cli mcp --help
byted-supabase-cli mcp serve --help
```

启动标准输入输出 MCP 服务：

```bash
byted-supabase-cli mcp serve
```

启动后 stdout 只能输出 MCP 协议数据；进度、诊断和错误日志应写 stderr。不要在 MCP 启动过程中输出欢迎语或普通文本。

## 工具调用边界

- 先确认当前 profile 和目标 Supabase URL；
- 读工具可以直接执行；
- 写工具必须取得用户明确确认；
- 删除、解绑、覆盖部署和批量变更需要二次确认；
- 工具参数使用结构化 JSON，拒绝不完整或歧义目标；
- 返回 HTTP 状态码、可操作错误和必要的资源 ID。

## 敏感信息

MCP 描述、参数、返回值、日志和 trace 中不得包含 service-role key、database password、JWT secret 或 secret value。endpoint 和数据库 host 在报告中使用 `<PUBLIC_ENDPOINT>`、`<DB_HOST>` 等占位符。

## 故障排查

MCP 无法启动时依次检查二进制路径、profile、stdio 是否被其他日志污染、Node/npx 依赖和实例网络。不要把 Workspace 管理模式凭据混用于当前实例直连服务。
