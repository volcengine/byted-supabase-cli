# Edge Functions 指南

## 查看和下载

```bash
byted-supabase-cli functions list -o json
byted-supabase-cli functions download <slug> --output-dir <dir>
byted-supabase-cli functions --help
```

部署前检查函数 slug、入口文件、runtime、依赖、环境变量和当前线上版本。下载线上代码到本地时注意不要把配置中的 secret 提交到仓库。

## 创建和开发

```bash
byted-supabase-cli functions new <slug>
byted-supabase-cli functions new <slug> --help
```

保持入口文件和 runtime 与当前 CLI 帮助一致。对外暴露的函数要明确鉴权策略、输入校验、错误返回和超时行为。

## 部署

```bash
byted-supabase-cli functions deploy <slug>
```

`functions deploy` 会修改线上版本。执行前确认：

- 函数 slug 和目标实例；
- 代码来源和变更范围；
- runtime 和依赖；
- 是否覆盖当前线上部署；
- 所需 secret 是否已配置。

部署后重新执行 `functions list -o json`，再做最小只读健康检查。失败时保留状态码和服务端错误，但对 endpoint、token 和 secret 脱敏。

## 安全

不要在函数代码、前端 bundle、日志或错误响应中暴露 service-role key、database URL、JWT secret 或 secret value。需要配置敏感变量时使用 `secrets` 命令，不要硬编码。
