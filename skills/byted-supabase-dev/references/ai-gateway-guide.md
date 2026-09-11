# AI Gateway 数据面指南

CLI的 AI Gateway 直接访问当前 Supabase 实例的数据面，不经过 Workspace/Manager 的控制面 API。当前实例由 `link` 保存的 Supabase URL 和凭据确定；命令参数以本地二进制的 `--help` 为准。

## 前置条件

先确认当前目录已经 link 到目标实例：

```bash
byted-supabase-cli link status -o json
```

AI Gateway 的模型查询、模型调用和限额管理使用当前  profile 中的 Supabase URL。限额管理必须使用 `service-role key`：

- `anon key` 适合公开数据面请求，不能用于限额管理。
- `service-role key` 仅用于受控 CLI 或后端操作，不能写入前端、日志、聊天输出或代码仓库。
- 不要在命令行示例、脚本或报告中回显真实 key。

如果尚未 link：

```bash
byted-supabase-cli link \
  --supabase-url <url> \
  [--anon-key <anon-key>] \
  [--service-role-key <service-role-key>]
```

## 模型查询

列出当前实例可用的模型：

```bash
byted-supabase-cli ai-gateway models list -o json
```

该命令调用数据面：

```text
GET <SUPABASE_URL>/v1/models
```

优先使用接口实际返回的模型 ID，不要凭经验猜测模型名称。

## 模型调用

使用 CLI 调用 OpenAI 兼容的聊天接口：

```bash
byted-supabase-cli ai-gateway chat completions \
  --model <model-id> \
  --message "请用一句话介绍这个实例。" \
  -o json
```

常用参数：

```text
--system <text>       可选系统提示词
--message <text>      用户消息
--model <model-id>    模型 ID
--stream               流式输出
--temperature <value> 采样温度
--max-tokens <value>  最大输出 Token
--top-p <value>       nucleus sampling 参数
--request <file>      使用完整 JSON 请求
```

对应数据面接口：

```text
POST <SUPABASE_URL>/v1/chat/completions
```

直接使用 HTTP 时，按受控后端方式传递凭据：

```bash
curl "$SUPABASE_URL/v1/chat/completions" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<model-id>",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": false
  }'
```

## 限额管理

CLI 的限额管理直接调用以下数据面接口，不使用 Workspace ID、Branch ID 或控制面 `internal/user/*` 接口：

| 操作 | 方法 | 数据面路径 |
|-|-|-|
| 查询全局默认限额 | `GET` | `/v1/aigateway/quota/default` |
| 修改全局默认限额 | `PUT` | `/v1/aigateway/quota/default` |
| 查询用户限额和用量 | `POST` | `/v1/aigateway/quota/users/summaries` |
| 批量修改用户限额 | `PUT` | `/v1/aigateway/quota/users` |

所有限额接口使用：

```text
Authorization: Bearer <service_role key>
```

限额值含义：

- `-1`：不限
- `0`：关闭
- 大于 `0`：具体上限

带有 `service_role:` 前缀的系统账号不允许修改。

### 默认限额

查询默认每日 Token 和图片限额：

```bash
byted-supabase-cli ai-gateway quota get -o json
```

修改默认限额，至少传入一个限额参数：

```bash
byted-supabase-cli ai-gateway quota set-default \
  --token-limit 100000 \
  --image-limit 50
```

CLI 请求体对应：

```json
{
  "user_default_quota": {
    "daily_token_limit": 100000,
    "daily_image_limit": 50
  }
}
```

未传入的限额不会被修改；使用 `-1` 表示不限。

### 用户限额和用量

分页查询用户信息、用户自定义限额及用量：

```bash
byted-supabase-cli ai-gateway quota users list \
  --offset 0 \
  --limit 20 \
  --search "@example.com" \
  -o json
```

查询单个用户：

```bash
byted-supabase-cli ai-gateway quota users get \
  --user-id <user-id> \
  -o json
```

设置用户自定义限额：

```bash
byted-supabase-cli ai-gateway quota users set \
  --user-id <user-id> \
  --token-limit 200000 \
  --image-limit 100
```

恢复用户的默认限额：

```bash
byted-supabase-cli ai-gateway quota users set \
  --user-id <user-id> \
  --reset
```

当前用户修改请求使用 `items` 数组；只传入一个限额时，另一项保持不变：

```json
{
  "items": [
    {
      "user_id": "<user-id>",
      "quota_customized": true,
      "custom_quota": {
        "daily_token_limit": 200000
      }
    }
  ]
}
```

### 用量汇总

汇总所有用户的今日和累计 Token、图片用量：

```bash
byted-supabase-cli ai-gateway usage -o json
```

该命令通过用户汇总数据面接口分页读取数据，再在本地计算总量。

## 状态和排障

检查 AI Gateway 数据面是否可用：

```bash
byted-supabase-cli ai-gateway status -o json
```

常见问题：

- `401` / `403`：确认当前 profile 的 key 有效；限额管理必须使用 `service-role key`。
- `404`：确认当前实例支持 `/v1/aigateway/quota/*` 数据面路由，并检查 Supabase URL。
- `quota_exceeded` 或 `no quota configured for user`：先执行 `quota get`，再根据需要执行 `quota set-default` 或 `quota users set`。
- 模型不可用：先执行 `ai-gateway models list`，使用返回的实际模型 ID。
- 当 profile 缺失或配置损坏：重新执行 `link`，不要手工修改凭据内容。

写入默认限额或用户限额前，先确认目标实例、用户 ID 和影响范围；不要为了验证权限执行写操作。
