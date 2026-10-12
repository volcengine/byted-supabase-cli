# Sandbox 指南

平台的 Sandbox 兼容 E2B 协议：每个沙箱是一台独立的 Linux 容器，可以读写文件、执行命令、运行代码、对外暴露端口，用完销毁。本文覆盖：

```text
准备资源 → 安装 SDK → 选创建身份 → 创建 → 执行 → 验证输出 → 回收
```

## 1. 选 CLI 还是 SDK

| 方式 | 适合 | 凭据 |
|---|---|---|
| CLI（`byted-supabase-cli sandbox ...`） | 一次性任务、调试、运维、摸清模板环境 | 管理面用 CLI 登录态；文件和进程操作用 `sandbox connect` 返回的 `envdAccessToken` |
| SDK（官方 E2B SDK + `@byted-supabase/sandbox` / `byted_supabase_sandbox`） | 业务应用、连续交互、流式输出、后台任务 | 用户 session 的 `access_token` 或后端 service_role key |

两套凭据不能混用：不要把 CLI `connect` 返回的 `envdAccessToken` 传给 SDK。

## 2. 准备：Workspace、Branch、URL 与凭据

```bash
byted-supabase-cli --version
byted-supabase-cli sandbox --help        # 报命令不存在就先 byted-supabase-cli update
byted-supabase-cli projects list --limit 10
byted-supabase-cli branches list --workspace-id "$WORKSPACE_ID" --limit 10 -o json   # 选 Ready 的分支
byted-supabase-cli endpoints list --workspace-id "$WORKSPACE_ID"                     # 分支的 API 地址 → SUPABASE_URL
byted-supabase-cli projects api-keys --workspace-id "$WORKSPACE_ID" --reveal -o json # anon key / service_role key
byted-supabase-cli templates list --workspace-id "$WORKSPACE_ID" --branch-id "$BRANCH_ID" -o json
```

- `SUPABASE_URL` 是分支的 API 地址（形如 `https://<branch-id>.supabase...volces.com`），SDK 会自动拼上 `/sandbox/v1`。
- 确认实例开通了 Sandbox：带 JWT 访问 `GET {SUPABASE_URL}/sandbox/v1/sandboxes` 返回 200。
- service_role key 只能放后端环境变量，不能进浏览器、前端构建产物、日志或 Git。

常用模板：

| 模板 | 内容 |
|---|---|
| `base` | 通用 Linux、Python、Node.js、Git、命令执行 |
| `code-interpreter` | Python 数据处理、Jupyter，结构化执行结果 |
| `codex` / `opencode` | 预装对应 Coding Agent CLI；平台把 AI Gateway 配置注入沙箱（opencode 为 `OPENCODE_CONFIG_CONTENT`，用法见第 10 节）。`opencode` 模板为 Debian、用户 `user`、免密 sudo、2 核 4GB，自带 python3 和 node，没有 ripgrep、Office 库、LibreOffice 和浏览器 |

**平台限制：**

- Sandbox 不支持暂停和恢复，没有可用的 `pause` / `resume` 生命周期命令。
- Sandbox 实例到期或被回收后，需新建 Sandbox，并从 Storage 等外部持久化介质恢复文件和状态。
- Sandbox 不支持创建、上传或注册自定义模板。只能使用 `templates list` 返回的平台模板；额外依赖和初始化逻辑只能在 Sandbox 创建后通过命令或 SDK 运行时安装、执行。

## 2.1 火山引擎 Sandbox 特有能力

### 平台环境变量

Sandbox 创建后，平台会在运行时为进程或 Jupyter 内核注入基础平台环境变量；这些变量不是镜像启动时的
`boot env`，而是通过运行期注入，因此在 `sudo` 重入的模板中也能生效：

| 环境变量 | 含义 | 注入条件 |
|---|---|---|
| `SUPABASE_URL` | 当前分支的 Supabase API 地址；优先使用公网地址 | 平台配置了分支地址时 |
| `SUPABASE_ANON_KEY` | 通过 Kong `apikey` 校验的公开 key，只负责“过门”，不提供 RLS 权限 | 平台配置了 anon key 时 |
| `SANDBOX_JWT` | 绑定当前 Sandbox 和归属用户的短期 JWT，用于访问 Supabase API 并执行 RLS | 创建身份满足下方 JWT 签发条件时 |

访问 Supabase API 时通常需要同时使用：

```bash
curl "$SUPABASE_URL/rest/v1/<table>" \
  -H "apikey: $SUPABASE_ANON_KEY" \
  -H "Authorization: Bearer $SANDBOX_JWT"
```

`SUPABASE_ANON_KEY` 是公开的 Kong 信封，不能替代 `SANDBOX_JWT`；真正决定用户身份和 RLS 权限的是
`Authorization` 中的 JWT。平台不会把长期有效的 `SUPABASE_SERVICE_ROLE_KEY` 注入 Sandbox。

平台环境变量是默认值，优先级从低到高为：

```text
PlatformEnv（平台默认值）< SandboxEnv（创建时 env）< RequestEnv（单次命令 env）
```

用户在创建参数或单次命令中显式提供同名变量时，用户值会覆盖平台默认值。请勿因此把服务端密钥、
原始用户 token 或 `SANDBOX_JWT` 写入日志、镜像或代码仓库。

### 三方用户与四方用户身份

这里的“三方/四方”对应网关代码中的两类 Supabase JWT 身份：三方是受信后端调用方，四方是终端用户调用方。

| 调用方 | 控制面 JWT | 创建方式 | Sandbox 内身份 |
|---|---|---|---|
| 三方用户 / 受信后端 | `role=service_role` | 使用后端 `service_role` JWT 创建；可通过 `metadata.ownerUserId` 代某个四方用户创建 | 传 `ownerUserId` 时签发该用户 scoped 的 `authenticated` JWT |
| 三方用户 / 受信管理任务 | `role=service_role` | 使用后端 `service_role` JWT，并显式传 `metadata.PassServiceRoleJwtToSandbox=true` | 签发短期 `service_role` `SANDBOX_JWT`，绕过 RLS；仅用于完全受控的管理任务 |
| 四方用户 / 终端用户 | `role=authenticated` 且必须有 `sub` | 使用该用户的 Supabase session `access_token` 创建；实例需开启 `control.allow_authenticated` | 平台按该用户 `sub` 签发 `SANDBOX_JWT`，仅代表该用户并受 RLS 约束 |

关键规则：

- `anon` JWT 不能创建 Sandbox，也不能作为 SDK 的 `apiKey`。
- 四方用户持 Supabase session JWT 直建只在实例的 `control.allow_authenticated` 开关开启时允许；
  该用户的 `sub` 会成为 Sandbox 归属和配额主体。
- 三方用户使用 `service_role` 代建四方用户的 Sandbox 时，优先传 `metadata.ownerUserId`，获得最小权限的
  用户 scoped JWT；不要默认请求 `PassServiceRoleJwtToSandbox`。
- `ownerUserId` 与 `PassServiceRoleJwtToSandbox=true` 不能同时使用；后者是显式提权选项，只允许
  `service_role` 调用方使用。
- 平台会按归属用户计算 Sandbox 配额；`ownerUserId` 是业务用户 ID，不是 Sandbox ID，也不是 JWT 原文。

示例：

```js
// 三方用户：使用当前登录用户的 Supabase session access_token。
// 这是 SDK 的 apiKey，不是 CLI 的管理面登录凭据。
initBytedSupabaseSandbox({
  url: process.env.SUPABASE_URL,
  apiKey: userSession.access_token,
})
const sandbox = await Sandbox.create("base")
```

```bash
# 三方用户：后端 service_role 代某个四方终端用户创建。
# CLI 示例使用 CLI 自身的管理面登录态；SDK 也可用 service_role JWT。
byted-supabase-cli sandbox create \
  --workspace-id "$WORKSPACE_ID" \
  --branch-id "$BRANCH_ID" \
  --template-name base \
  --metadata "ownerUserId=$USER_ID" \
  --detach -o json
```

示例中的 token、Sandbox ID 和 `ownerUserId` 只使用占位符，不要回显真实值。

### AI Gateway 模板适配

平台会从同实例 AI Gateway 的模型目录解析当前默认模型，并对 Agent 模板做运行期适配。不是所有模板
都会注入 AI Gateway 变量：

| 模板名前缀 | 注入内容 | 注入方式 |
|---|---|---|
| `claude` / `claude-*` | `ANTHROPIC_BASE_URL`、`ANTHROPIC_AUTH_TOKEN`、`ANTHROPIC_MODEL` | 运行期环境变量 |
| `codex` / `codex-*` | Codex provider 的 `base_url`、模型配置；认证引用 `SANDBOX_JWT` | Sandbox claim 后写入 `/etc/codex/config.toml` |
| `opencode` / `opencode-*` | `OPENCODE_CONFIG_CONTENT`，其中 provider 指向 Supabase AI Gateway，认证引用 `SANDBOX_JWT` | 运行期环境变量 |
| `base`、`code-interpreter` 或其他模板 | 不注入 Agent 专属 AI Gateway 配置 | 使用模板原生环境 |

Agent 适配只有在以下条件同时满足时才启用：模板名称匹配、平台签发了 `SANDBOX_JWT`、存在
`SUPABASE_URL`、AI Gateway 能解析出模型，并且实例公网地址可从 Sandbox 访问。私网专用实例不会注入
这组 Agent 配置，避免生成必然连接超时的地址。

用户可通过创建 metadata 中的 `DisableAiGatewayInjection=true` 关闭 Agent 专属适配；该开关只停止
`ANTHROPIC_*`、`OPENCODE_CONFIG_CONTENT` 和 Codex 配置写入，不影响基础的 `SUPABASE_URL`、
`SUPABASE_ANON_KEY` 或身份 JWT。若用户自带 Anthropic/OpenAI 配置并希望指向其他网关，应使用这个
开关，避免平台配置覆盖用户预期。

## 3. 安装 SDK

### e2b 版本必须低于 2.51

从 2.51.0 起，官方 SDK（Node 和 Python）创建、连接沙箱改走 `/v2/sandboxes`，网关 6.9.1 之前不支持，返回 404 `sandbox control endpoint is not available`。所以：

- Node：`e2b@2.50.0`；需要 code-interpreter 时 `@e2b/code-interpreter@2.8.0`（2.8.2 起依赖 `e2b@^2.53`）。
- Python：`byted_supabase_sandbox` 已约束 `e2b<2.51`，pip 会自动选兼容的 `e2b-code-interpreter`。
- 不要不带版本号安装 `e2b`，会装到最新版。

### Node.js

SDK 还没发布到 npm，从 GitHub 构建：

```bash
git clone https://github.com/volcengine/byted-supabase-cli.git .vendor/byted-supabase-cli
(cd .vendor/byted-supabase-cli/sandbox_sdk/node && npm install && npm pack)
npm install ./.vendor/byted-supabase-cli/sandbox_sdk/node/byted-supabase-sandbox-0.1.0.tgz e2b@2.50.0
npm install @supabase/supabase-js           # 用户 session 模式需要
npm install @e2b/code-interpreter@2.8.0     # code-interpreter 模板需要
```

必须先 `npm pack` 再装 tgz。直接 `npm install <目录>` 会建软链，包会解析到它自己目录下的另一份 `e2b`，补丁打在那份上，业务代码里 `import { Sandbox } from 'e2b'` 拿到的类没有补丁，请求会发往 e2b 官方云。装完用 `npm ls e2b` 确认全树只有一份 `e2b@2.50.0`。

在 Deno（Edge Function）里：没有 npm 包可引用，把 `sandbox_sdk/node/src/e2b_gateway.ts` 拷进函数目录，用 `npm:e2b@2.50.0`。

### Python（3.10+）

```bash
python3 -m venv .venv && source .venv/bin/activate
pip install "git+https://github.com/volcengine/byted-supabase-cli.git#subdirectory=sandbox_sdk/python"
pip install "byted-supabase-sandbox[code-interpreter] @ git+https://github.com/volcengine/byted-supabase-cli.git#subdirectory=sandbox_sdk/python"  # 需要 code-interpreter 时
pip install supabase   # 用户 session 模式需要
```

## 4. 选创建身份

| 模式 | `apiKey` | 创建参数 | 沙箱内 `SANDBOX_JWT` | 适合 |
|---|---|---|---|---|
| 用户 session（推荐） | 用户登录后的 `access_token` | 无需 metadata | 该用户，受 RLS 约束 | 前端或后端代表已登录用户 |
| 后端代建 | service_role key | `metadata: { ownerUserId: <user id> }` | 该用户，受 RLS 约束 | 后端统一编排，归属到某个用户 |
| 管理任务 | service_role key | `metadata: { PassServiceRoleJwtToSandbox: 'true' }` | service_role，绕过 RLS | 完全受控的管理级处理 |

- anon key 不能当 `apiKey`，网关会拒绝。
- 后端代建时，沙箱归属、每用户沙箱配额都按 `ownerUserId` 计。连接用户已有的沙箱时可以用用户自己的 token `Sandbox.connect`，网关会校验归属。
- 一个进程同时服务多个用户时（如 Edge Function），`initBytedSupabaseSandbox` 是进程级全局状态：初始化时不放凭据，每次 `create` / `connect` 显式传 `apiKey`。

用户 session 模式（Node）：

```js
import { createClient } from '@supabase/supabase-js'
import { initBytedSupabaseSandbox } from '@byted-supabase/sandbox'
import { Sandbox } from 'e2b'

const supabase = createClient(process.env.SUPABASE_URL, process.env.SUPABASE_ANON_KEY)
const { data, error } = await supabase.auth.signInWithPassword({ email, password })
if (error) throw error

initBytedSupabaseSandbox({ url: process.env.SUPABASE_URL, apiKey: data.session.access_token })
const sandbox = await Sandbox.create('base', { timeoutMs: 10 * 60 * 1000 })
```

## 5. Node.js 闭环（后端代建）

```js
import { initBytedSupabaseSandbox } from '@byted-supabase/sandbox'
import { Sandbox } from 'e2b'

initBytedSupabaseSandbox({
  url: process.env.SUPABASE_URL,
  apiKey: process.env.SUPABASE_SERVICE_ROLE_KEY,
})

const sandbox = await Sandbox.create('base', {
  timeoutMs: 10 * 60 * 1000,
  metadata: { ownerUserId: process.env.SANDBOX_OWNER_USER_ID },
})
try {
  await sandbox.files.write('/tmp/hello.py', 'print("hello from Supabase Sandbox")\n')
  const result = await sandbox.commands.run('python3 /tmp/hello.py')
  if (result.exitCode !== 0 || !result.stdout.includes('hello from Supabase Sandbox')) {
    throw new Error(`sandbox task failed: exit ${result.exitCode}, stderr: ${result.stderr}`)
  }
} finally {
  await sandbox.kill()
}
```

`SANDBOX_OWNER_USER_ID` 是业务用户 ID，不是沙箱 ID。完整示例见 [`scripts/sandbox-node.mjs`](../scripts/sandbox-node.mjs)。

命令非零退出时 SDK 会直接抛 `CommandExitError`（Python 为 `CommandExitException`），需要容忍失败时 catch 后读它的 `exitCode` / `stderr`。验证时同时看 `stdout`、`stderr`、`exitCode`、产出文件，以及业务数据的只读查询结果，不要只看外层进程退出码。

## 6. Python 闭环（后端代建）

```python
import os

from byted_supabase_sandbox import init_byted_supabase_sandbox
from e2b import Sandbox

init_byted_supabase_sandbox(
    url=os.environ["SUPABASE_URL"],
    api_key=os.environ["SUPABASE_SERVICE_ROLE_KEY"],
)

with Sandbox.create(
    "base",
    timeout=600,
    metadata={"ownerUserId": os.environ["SANDBOX_OWNER_USER_ID"]},
) as sandbox:
    sandbox.files.write("/tmp/hello.py", 'print("hello from Supabase Sandbox")\n')
    result = sandbox.commands.run("python3 /tmp/hello.py")
    if result.exit_code != 0 or "hello from Supabase Sandbox" not in result.stdout:
        raise RuntimeError(f"sandbox task failed: exit {result.exit_code}, stderr: {result.stderr}")
```

离开 `with` 时自动回收，抛异常也一样。完整示例见 [`scripts/sandbox-python.py`](../scripts/sandbox-python.py)。

## 7. CLI 一次性流程

```bash
CREATE_JSON="$(byted-supabase-cli sandbox create \
  --workspace-id "$WORKSPACE_ID" --branch-id "$BRANCH_ID" \
  --template-name base --timeout 600 --detach -o json)"
export SANDBOX_ID="$(printf '%s' "$CREATE_JSON" | jq -r '.sandboxID')"
unset CREATE_JSON

CONNECT_JSON="$(byted-supabase-cli sandbox connect \
  --workspace-id "$WORKSPACE_ID" --branch-id "$BRANCH_ID" \
  --sandbox-id "$SANDBOX_ID" --timeout 600 -o json)"
export SANDBOX_ACCESS_TOKEN="$(printf '%s' "$CONNECT_JSON" | jq -r '.envdAccessToken')"
unset CONNECT_JSON

byted-supabase-cli sandbox exec \
  --workspace-id "$WORKSPACE_ID" --branch-id "$BRANCH_ID" \
  --sandbox-id "$SANDBOX_ID" --sandbox-access-token "$SANDBOX_ACCESS_TOKEN" \
  --command echo --arg sandbox-ok -o json

byted-supabase-cli sandbox kill \
  --workspace-id "$WORKSPACE_ID" --branch-id "$BRANCH_ID" \
  --sandbox-id "$SANDBOX_ID" --yes -o json
unset SANDBOX_ID SANDBOX_ACCESS_TOKEN
```

- `exec` 的结果要解析 JSON 里的 `exit_code` / `status` 和 stdout / stderr，CLI 自身退出码为 0 不代表命令成功。
- `sandboxID` 和 `envdAccessToken` 只放环境变量，不打印、不写日志；失败时也要 `kill`。
- 其他子命令：`files list/read/write/exists/rename/remove`、`code run`（code-interpreter）、`process start/list`、`port url`、`timeout`、`health`、`list`、`quota`。完整脚本见 [`scripts/sandbox-cli-lifecycle.sh`](../scripts/sandbox-cli-lifecycle.sh)。

## 8. 沙箱内访问 Supabase

平台向沙箱注入：

| 环境变量 | 用途 |
|---|---|
| `SUPABASE_URL` | Supabase API 地址 |
| `SUPABASE_ANON_KEY` | API 网关的 `apikey` |
| `SANDBOX_JWT` | `Authorization: Bearer ...`；身份由创建模式决定（见第 4 节），带 `sandbox_id` claim，24 小时有效 |

请求时同时带 `apikey: $SUPABASE_ANON_KEY` 和 `Authorization: Bearer $SANDBOX_JWT`。不要把 `SANDBOX_JWT` 当 `apikey`（网关返回 401），不要自行生成、刷新或打印它。

沙箱可以用自己的 `SANDBOX_JWT` 管理自己（`sandbox_id` 取自 JWT 的 claim）：

```bash
# 续期：从现在起再活 1800 秒
curl -X POST "$SUPABASE_URL/sandbox/v1/sandboxes/$SANDBOX_ID/timeout" \
  -H "apikey: $SUPABASE_ANON_KEY" -H "Authorization: Bearer $SANDBOX_JWT" \
  -H 'content-type: application/json' -d '{"timeout":1800}'
# 任务做完销毁自己（数秒内消失；会杀掉发起请求的进程，先写完其他状态）
curl -X DELETE "$SUPABASE_URL/sandbox/v1/sandboxes/$SANDBOX_ID" \
  -H "apikey: $SUPABASE_ANON_KEY" -H "Authorization: Bearer $SANDBOX_JWT"
```

## 9. 在应用里长期使用沙箱

- **后台命令**：`sandbox.commands.run(cmd, { background: true, timeoutMs: 0 })`。`timeoutMs: 0` 不能省，否则 SDK 60 秒后结束命令。启动后立即 `handle.disconnect()`：每个实例只有 64 条数据面连接，不要让前端或后端长期挂着命令流。
- **环境变量**：envd 启动的命令不继承镜像里的 ENV，需要的变量（如 `PIP_INDEX_URL`）在命令里自己设。
- **耗时**：命中预热池建沙箱约 1 秒；池空时冷启动 90–133 秒，`create` 一直阻塞，`requestTimeoutMs` 要留够。
- **生命周期**：最长 24 小时，不支持暂停和恢复，也不支持通过 pause/resume 保留实例状态。需要长期保留的文件和状态写回 Storage，下次创建新沙箱后恢复状态。
- **配额**：每用户每天默认 100 个沙箱，实例默认最多 120 个并发沙箱（`sandbox quota` 查看）。
- **网络**：默认可以访问公网。apt 已配 volces 镜像；pip 未配源，推荐 `https://mirrors.volces.com/pypi/simple/`。
- **端口预览**：`sandbox port url` 或 `sandbox.getHost(port)` 返回 `{SUPABASE_URL}/sandbox/v1/proxy/{sandbox_id}/{port}/`，末尾斜杠不能省（页面里的相对资源靠它）。默认（`allowPublicTraffic` 为 true）持有地址即可访问、CORS 全开，所以地址只给沙箱归属者；创建时关掉 `allowPublicTraffic` 后，请求要带 `e2b-traffic-access-token`。30 秒无响应返回 502；WebSocket 需要网关 6.9.1 及以上。
- **回收判断**：kill 之后几分钟内 `getInfo` 可能仍显示 running，判断沙箱是否还在用 `list` 或 `connect`。
- **复用已有沙箱**：保存 `sandboxId`，下次先 `Sandbox.connect`（传 `timeoutMs`，否则 SDK 默认把寿命改成 5 分钟）；返回 403 / 404 / 409 / 410 / 502 或 `SandboxNotFoundError` 时当作已回收，新建一个并从 Storage 恢复状态。
- **分支休眠**：有运行中的沙箱时分支不会自动休眠。

## 10. 在沙箱里运行 opencode

`opencode` 模板里，平台通过 `OPENCODE_CONFIG_CONTENT` 注入一个指向 AI Gateway 的 provider（id 为 `supabase-ai-gateway`），认证引用 `SANDBOX_JWT`，用的是沙箱归属用户的 AI 额度。

```bash
cd ~/workspace
PWD=$HOME/workspace OPENCODE_DISABLE_AUTOUPDATE=1 \
  opencode run --auto --format json [--session <id>] [--model supabase-ai-gateway/<model>] -- "<prompt>"
```

- **工作目录看 `$PWD`**：opencode 按环境变量 `PWD` 认项目目录。envd 启动的进程继承 `PWD=/home/user`，只设子进程 cwd 不够，会在 `~` 下干活、找不到项目里的 skill。
- **换模型**：注入的配置只声明了默认模型，直接 `--model` 指定其他模型会报 `UnknownError`。先解析 `OPENCODE_CONFIG_CONTENT`，把模型加到 `provider["supabase-ai-gateway"].models`（`{"<model>": {"name": "<model>"}}`），再把改过的 JSON 作为同名环境变量传给 opencode。
- **新实例**：沙箱服务每 10 分钟从 AI Gateway 同步一次模型列表，实例创建后约 10 分钟内沙箱里可能没有 `OPENCODE_CONFIG_CONTENT`。
- **事件流**：`--format json` 时 stdout 每行一个 JSON：`text`（整段文本，`part.text`）、`tool_use`（调用和结果合在一条，工具结束时才输出；`part.tool`、`part.callID`、`part.state`）、`step_finish`（`part.tokens`、`part.cost`）、`error`、`step_start`；`reasoning` 只在加 `--thinking` 时出现。会话 id 在顶层 `sessionID`。
- **续聊**：会话绑定工作目录，必须在同一目录下 `--session <id>`。换沙箱续聊时，把 `~/.local/share/opencode/opencode.db`、`opencode.db-wal`、`opencode.db-shm` 存到 Storage，新沙箱里解压回原位置、在同一目录下继续。
- **AGENTS.md 与 Skills**：opencode 读取工作目录下的 `AGENTS.md`，以及 `<工作目录>/.opencode/skills/<name>/SKILL.md`（frontmatter 要有 `name`、`description`）。skill 工具依赖 ripgrep，先 `sudo apt-get install -y ripgrep`。
- **卡住与取消**：AI Gateway 不可达时 opencode 每 30 秒重试、不退出也不输出，要设整体超时。opencode 的 bash 工具让命令跑在独立进程组里，取消时只杀 opencode 的进程组会漏掉这些子进程，要逐个杀。

## 11. 常见问题

| 现象 | 处理 |
|---|---|
| `sandbox` 命令不存在 | `byted-supabase-cli update`，再看 `sandbox --help` |
| 创建返回 404 `sandbox control endpoint is not available` | e2b 版本 ≥2.51，按第 3 节锁版本 |
| 报「请去 e2b.dev 获取 API key」或请求打到 e2b 官方域名 | 补丁没生效：Node 是目录安装导致多份 `e2b`（`npm ls e2b` 检查），或没先调用 `initBytedSupabaseSandbox` |
| 创建返回 401 / 403 | 检查凭据和 Workspace / Branch 归属；不要用 anon key 当 `apiKey` |
| 创建返回 429 | 每用户每日或实例并发配额用完，`sandbox quota` 查看 |
| 创建返回 501 | 当前环境不支持该参数（如 `--allow-internet=false`），看 `sandbox create --help` |
| 后台命令一分钟后消失 | 没传 `timeoutMs: 0` |
| opencode `--model` 报 `UnknownError` | 模型没在 `OPENCODE_CONFIG_CONTENT` 里声明，见第 10 节 |
| opencode 找不到项目里的 skill、文件写到了 `~` | 没设环境变量 `PWD`，见第 10 节 |
| CLI 文件 / 进程操作返回 401 | `envdAccessToken` 不是本次 `connect` 拿到的，或 Sandbox ID 不一致 |
| code-interpreter 返回 404 / 502 | 模板不是 `code-interpreter`，或没装对应 SDK |
| 端口地址返回 401 | 沙箱关了 `allowPublicTraffic` 但请求没带 traffic token |
| 端口地址返回 502 | 沙箱里的服务没启动、没监听 `0.0.0.0`，或 30 秒没有响应 |

排障时只记录 CLI / SDK 版本、脱敏后的命令、HTTP 状态码和原始错误，不记录 key、token、密码或完整连接串。
