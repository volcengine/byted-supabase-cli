# Auth 服务指南

## 配置读取

先只读取需要的配置项：

```bash
byted-supabase-cli auth config get --key SITE_URL
byted-supabase-cli auth config get --key DISABLE_SIGNUP
byted-supabase-cli auth config get -o json
```

完整配置可能包含 SMTP、JWT、第三方 provider 等敏感信息，不要直接复制到聊天、日志或工单。

## 配置修改

修改前先读取当前值，确认 key 名称和类型，再执行：

```bash
byted-supabase-cli auth config set --key SITE_URL --value https://app.example.com
```

不同版本的 flag 可能不同，先执行：

```bash
byted-supabase-cli auth config set --help
```

修改后重新读取相同 key 验证。生产环境修改 URL、注册开关、邮件配置和 JWT 相关配置前，必须说明影响并取得确认。

## Hooks

```bash
byted-supabase-cli auth hooks get -o json
byted-supabase-cli auth hooks set --help
```

修改 Hooks 前保存非敏感的当前配置，并保留未修改字段。URI、secret 和内部数据库函数地址不要写入报告。

## 第三方登录

```bash
byted-supabase-cli auth third-party list -o json
byted-supabase-cli auth third-party add --help
byted-supabase-cli auth third-party remove --help
byted-supabase-cli auth third-party sync
```

OIDC issuer、JWKS、client secret 等配置属于敏感信息。遇到 401、403 和 5xx 时分别判断凭据、权限和服务状态，不要通过重复修改 provider 配置排障。

## 验证

Auth 配置修改后，优先重新读取配置；需要验证登录流程时使用测试账号和最小权限，不要在生产环境创建临时用户或输出 access token。
