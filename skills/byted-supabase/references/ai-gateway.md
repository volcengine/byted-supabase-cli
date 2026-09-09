# AI Gateway 接入与额度管理

AIDAP Supabase 的 AI Gateway 为 Workspace 分支提供 OpenAI 兼容的大模型访问能力。本文覆盖模型访问、应用接入和额度管理。

## 前置准备

AI Gateway 的目标是 **Workspace + Branch**；省略 `--branch-id` 时使用 Workspace 的默认分支。

```bash
byted-supabase-cli ai-gateway status --workspace-id <workspace-id>
byted-supabase-cli ai-gateway models list --workspace-id <workspace-id> -o json
```

调用类命令需要 Bearer Token，可以使用 `--token`、`--token-env`，或环境变量 `AI_GATEWAY_TOKEN`。也支持 `SUPABASE_SERVICE_ROLE_KEY` 和 `SUPABASE_ACCESS_TOKEN`。模型列表可能变化，接入前应以 `models list` 返回结果为准。

常用环境变量：

```text
SUPABASE_URL=https://br-xxx.supabase.aidap.cn-beijing.volces.com
SUPABASE_ANON_KEY=<supabase-anon-key>
SUPABASE_SERVICE_ROLE_KEY=<service-role-key>
```

以下示例模型仅用于演示，实际使用前请确认 Workspace 支持该模型。

## 模型使用

### CLI 入口

使用 CLI 查询目标 Workspace 分支的 AI Gateway 状态和模型：

```bash
byted-supabase-cli ai-gateway status \
  --workspace-id <workspace-id> \
  --branch-id <branch-id>

byted-supabase-cli ai-gateway models list \
  --workspace-id <workspace-id> \
  --branch-id <branch-id> \
  -o json
```

### 查看可用模型

模型列表展示模型名称、模型类型和提供方。常见模型包括：

| 模型 | 类型 | 用途 |
| --- | --- | --- |
| `bytedance/doubao-seed-2.1-pro` | 文本 | 通用对话、复杂推理 |
| `bytedance/doubao-seed-2.1-turbo` | 文本 | 快速对话和高吞吐场景 |
| `bytedance/doubao-seed-2.0-lite` | 文本 | 轻量文本任务 |
| `bytedance/doubao-seed-2.0-mini` | 文本 | 简单文本任务 |
| `bytedance/doubao-seedance-2.0` | 视频 | 视频生成 |
| `bytedance/doubao-seedance-2.0-fast` | 视频 | 快速视频生成 |
| `bytedance/doubao-seedream-5.0-pro` | 图片 | 高质量图片生成 |
| `bytedance/doubao-seedream-5.0-lite` | 图片 | 轻量图片生成 |

实际可用模型可能随平台更新变化，应以下面命令的实时结果为准：

```bash
byted-supabase-cli ai-gateway models list \
  --workspace-id <workspace-id> \
  --branch-id <branch-id> \
  -o json
```

已停止服务的模型不要继续使用。模型迁移时，优先根据当前列表选择替代模型；例如旧版 `doubao-seed-2.0-pro`、`doubao-seed-2.0-code` 已停止服务时，应分别迁移到平台当前提供的 `doubao-seed-2.1-pro`、`doubao-seed-2.1-turbo` 或其他适合模型。

### CLI 测试模型

使用 `test` 做最小连通性测试，使用 `chat completions` 传入自定义提示词：

```bash
export AI_GATEWAY_TOKEN='<access-token-or-service-role-key>'

byted-supabase-cli ai-gateway test \
  --workspace-id <workspace-id> \
  --branch-id <branch-id> \
  --token-env AI_GATEWAY_TOKEN

byted-supabase-cli ai-gateway chat completions \
  --workspace-id <workspace-id> \
  --branch-id <branch-id> \
  --model bytedance/doubao-seed-2.1-pro \
  --message '什么是大模型？' \
  --stream \
  --token-env AI_GATEWAY_TOKEN
```

图片模型使用 Images API；视频和图片模型所需的尺寸、比例、时长等参数，以模型接口实际返回的参数校验结果为准。CLI 目前提供图片生成的 OpenAI 兼容 cURL 示例，见[图片生成](#图片生成)。

历史生成的视频产物会写入对应分支的 Storage：

```text
Bucket: _supabase_internal_ai_gateway_output
Video:  video/<tenant>/<user>/<task_id>/video.mp4
Frame:  video/<tenant>/<user>/<task_id>/last_frame.<ext>
```

### 调用方式

模型调用统一使用 OpenAI 兼容协议：

- 文本模型：`POST /v1/chat/completions`
- 图片模型：`POST /v1/images/generations`
- 模型发现：`GET /v1/models`

通用流程是：获取当前用户 Session 的 `access_token`，将其作为 Bearer Token，使用 `<SUPABASE_URL>/v1` 作为 API Base URL，再传入模型 ID。完整的 JavaScript、Python、Vercel AI SDK 和 cURL 示例见本文的[应用接入](#应用接入)。


## 应用接入

### Supabase Auth access_token 前端访问

适用于纯前端应用或 Agent。用户登录后，Supabase Auth 会话中的 `access_token` 作为 AI Gateway Bearer Token。

```bash
npm install @supabase/supabase-js openai
```

```javascript
import { createClient } from '@supabase/supabase-js';
import OpenAI from 'openai';

const supabaseUrl = 'https://br-xxx.supabase.aidap.cn-beijing.volces.com';
const supabaseAnonKey = 'your-supabase-anon-key';
const supabase = createClient(supabaseUrl, supabaseAnonKey);

async function chatWithAI() {
  const {
    data: { session },
    error: sessionError,
  } = await supabase.auth.getSession();

  if (sessionError) throw sessionError;
  if (!session?.access_token) throw new Error('请先登录 Supabase Auth');

  const openai = new OpenAI({
    baseURL: `${supabaseUrl}/v1`,
    apiKey: session.access_token,
    dangerouslyAllowBrowser: true,
  });

  const stream = await openai.chat.completions.create({
    model: 'bytedance/doubao-seed-2.1-pro',
    messages: [{ role: 'user', content: '你好，请介绍一下自己。' }],
    stream: true,
  });

  for await (const chunk of stream) {
    process.stdout.write(chunk.choices[0]?.delta?.content || '');
  }
}

await chatWithAI();
```

使用 Vercel AI SDK：

```bash
npm install @supabase/supabase-js ai @ai-sdk/openai-compatible
```

```javascript
import { createClient } from '@supabase/supabase-js';
import { createOpenAICompatible } from '@ai-sdk/openai-compatible';
import { streamText } from 'ai';

const supabaseUrl = 'https://br-xxx.supabase.aidap.cn-beijing.volces.com';
const supabase = createClient(supabaseUrl, 'your-supabase-anon-key');
const { data: { session } } = await supabase.auth.getSession();

if (!session?.access_token) throw new Error('请先登录 Supabase Auth');

const gateway = createOpenAICompatible({
  name: 'supabase-ai-gateway',
  baseURL: `${supabaseUrl}/v1`,
  apiKey: session.access_token,
});

const result = streamText({
  model: gateway('bytedance/doubao-seed-2.1-pro'),
  prompt: '你好，请介绍一下自己。',
});

for await (const text of result.textStream) {
  process.stdout.write(text);
}
```

### Third-Party Auth JWT 前端访问

适用于已配置 Third-Party Auth 或自定义 JWKS 的 Workspace。参考[设置 Third-Party 登录](https://www.volcengine.com/docs/87275/2482138)，从受信任的第三方认证系统获取用户 JWT，并将 JWT 作为 AI Gateway Bearer Token。

```javascript
import OpenAI from 'openai';

const supabaseUrl = 'https://br-xxx.supabase.aidap.cn-beijing.volces.com';
const thirdPartyJwt = await getUserThirdPartyJwtToken();

if (!thirdPartyJwt) throw new Error('未获取到 Third-Party Auth JWT');

const openai = new OpenAI({
  baseURL: `${supabaseUrl}/v1`,
  apiKey: thirdPartyJwt,
  dangerouslyAllowBrowser: true,
});

const stream = await openai.chat.completions.create({
  model: 'bytedance/doubao-seed-2.1-pro',
  messages: [{ role: 'user', content: '你好。' }],
  stream: true,
});

for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content || '');
}
```

Vercel AI SDK 只需要将 `apiKey` 换成第三方 JWT：

```javascript
import { createOpenAICompatible } from '@ai-sdk/openai-compatible';
import { streamText } from 'ai';

const gateway = createOpenAICompatible({
  name: 'supabase-ai-gateway',
  baseURL: 'https://br-xxx.supabase.aidap.cn-beijing.volces.com/v1',
  apiKey: await getUserThirdPartyJwtToken(),
});

const result = streamText({
  model: gateway('bytedance/doubao-seed-2.1-pro'),
  prompt: '请用一句话介绍 AI Gateway。',
});

for await (const text of result.textStream) {
  process.stdout.write(text);
}
```

### service_role_key 后端访问

适用于服务端任务、定时任务和后端 API。`service_role_key` 权限较高，**绝不能放进浏览器、移动端包或前端环境变量**。

```bash
pip install openai
export SUPABASE_URL='https://br-xxx.supabase.aidap.cn-beijing.volces.com'
export SUPABASE_SERVICE_ROLE_KEY='<service-role-key>'
```

```python
import os
from openai import OpenAI

client = OpenAI(
    base_url=f"{os.environ['SUPABASE_URL']}/v1",
    api_key=os.environ["SUPABASE_SERVICE_ROLE_KEY"],
)

response = client.chat.completions.create(
    model="bytedance/doubao-seed-2.1-pro",
    messages=[{"role": "user", "content": "你好，请简单介绍一下自己。"}],
)

print(response.choices[0].message.content)
```

后端流式输出：

```python
stream = client.chat.completions.create(
    model="bytedance/doubao-seed-2.1-pro",
    messages=[{"role": "user", "content": "写一个 Python 快速排序示例。"}],
    stream=True,
)

for chunk in stream:
    print(chunk.choices[0].delta.content or "", end="", flush=True)
```

不使用 SDK 时：

```bash
curl "$SUPABASE_URL/v1/chat/completions" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "bytedance/doubao-seed-2.1-pro",
    "messages": [{"role": "user", "content": "你好，请介绍一下 AI Gateway。"}],
    "stream": false
  }'
```

## 图片生成

图片模型通过 OpenAI 兼容的 Images API 调用：

```bash
curl "$SUPABASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "bytedance/doubao-seedream-5.0-lite",
    "prompt": "一只可爱的橘猫坐在窗边，阳光，简洁插画风格",
    "size": "1920x1920"
  }'
```

图片尺寸需满足具体模型的最小像素要求；例如实测 `doubao-seedream-5.0-lite` 使用 `1024x1024` 会被拒绝，`1920x1920` 可以成功。返回的图片 URL 通常是带有效期的临时地址，应及时下载或持久化。

## 限额管理

### CLI 入口

使用以下命令查询和配置 Workspace 分支的全局默认额度及用户自定义额度：

```bash
byted-supabase-cli ai-gateway quota get \
  --workspace-id <workspace-id> \
  --branch-id <branch-id> \
  -o json

byted-supabase-cli ai-gateway quota users list \
  --workspace-id <workspace-id> \
  --branch-id <branch-id> \
  --limit 10 \
  --offset 0 \
  -o json
```

### 全局默认限额

`quota get` 返回当前 Workspace 分支的全局默认限额。新创建 Workspace 的默认值通常为：

- 每个用户每日 Token 限额：`0`
- 每个用户每日图片生成限额：`0`

限额为 `0` 时，用户调用 AI Gateway 可能收到：

```json
{"error":{"message":"no quota configured for user","type":"quota_exceeded"}}
```

使用 `quota set-default`，可以分别将 Token 和图片额度设置为指定额度或不限量。修改后，所有未设置自定义额度的用户都会使用新的全局默认值。

### 单用户限额

`quota users list` 返回：

| 字段 | 说明 |
| --- | --- |
| 用户 ID / 邮箱 | 用户 UUID 及关联身份信息 |
| 今日消耗 Token | 当前用户当天的 Token 用量 |
| 历史消耗 Token | 当前用户累计 Token 用量 |
| 今日生成图片 | 当前用户当天的图片生成量 |
| 历史生成图片 | 当前用户累计图片生成量 |
| 每日限额 | 用户使用默认额度或自定义额度 |
| 加入时间 | 用户首次使用 AI Gateway 的时间 |

当前 CLI 支持按分页读取用户列表，并使用 `quota users get --user-id` 查询单个用户；再通过 `quota users set` 设置 Token 或图片生成额度。设置自定义额度后，该用户不再跟随全局默认额度。当前 CLI 未提供按用户名、邮箱或手机号筛选参数。

### CLI 操作

```bash
# 查询全局默认额度
byted-supabase-cli ai-gateway quota get \
  --workspace-id <workspace-id> --branch-id <branch-id> -o json

# 设置全局 Token 和图片额度
byted-supabase-cli ai-gateway quota set-default \
  --workspace-id <workspace-id> --branch-id <branch-id> \
  --token-limit 100000 --image-limit 20

# 分页查看用户额度和用量
byted-supabase-cli ai-gateway quota users list \
  --workspace-id <workspace-id> --branch-id <branch-id> \
  --limit 10 --offset 0 -o json

# 查询单个用户
byted-supabase-cli ai-gateway quota users get \
  --workspace-id <workspace-id> --branch-id <branch-id> \
  --user-id <user-id> -o json

# 设置单用户额度；至少传入一个额度参数
byted-supabase-cli ai-gateway quota users set \
  --workspace-id <workspace-id> --branch-id <branch-id> \
  --user-id <user-id> --token-limit 50000 --image-limit 10

# 汇总当前分支用量
byted-supabase-cli ai-gateway usage \
  --workspace-id <workspace-id> --branch-id <branch-id> -o json
```


参数说明：

- `--token-limit` 是原始 Token 数量/天，例如 `100000` 表示 10 万 Token/天。
- `--image-limit` 是图片张数/天，例如 `20` 表示每天最多生成 20 张图片。
- 对应参数传 `-1` 表示不限量；`set-default` 和 `quota users set` 至少传入一个额度参数，未传入的另一项不会被修改。
- 新 Workspace 默认额度通常为 `0`，出现 `no quota configured for user` 或 `quota_exceeded` 时，先配置全局或用户额度。
- 用户列表包含 Token 和图片的今日/历史用量；`usage` 会分页汇总当前分支数据。平台后置计量可能导致实际消耗略高于配置额度。


## 排障

- `no quota configured for user` / `quota_exceeded`：执行 `quota get`，再用 `quota set-default` 或 `quota users set` 配置额度。
- 认证错误：确认 Token 属于当前 Workspace，并确认 URL 是 `<SUPABASE_URL>/v1`。
- 模型不存在：执行 `ai-gateway models list`，使用实际返回的模型 ID。
