# Supabase Sandbox SDK 使用指南

AI 原生 BaaS 平台 Supabase 版的 Sandbox 功能**兼容 E2B 协议**，您可以直接使用 [E2B 官方 SDK](https://e2b.dev) 来使用沙箱：不需要学习新的 API，E2B 的官方文档对您同样适用，只需在程序启动时加一行初始化，把 SDK 指向您的实例即可。

Sandbox 让您在 Supabase 实例里安全地运行任意代码 —— 执行用户提交的脚本、运行 AI Coding Agent、起一个能对外访问的临时服务。每个沙箱是一台独立的容器化 Linux 机器，用完即销毁。

---

# Node / TypeScript

## 一、前置准备

您需要的三样东西：

| | 用途 | 能放前端吗 |
|---|---|---|
| **实例 URL** | 您的 Supabase 实例地址 | 可以 |
| **anon key** | 走用户登录、拿 session（模式 1 用） | 可以 |
| **service_role key** | 后端代表用户创建沙箱（模式 2、3 用） | **绝对不行** |

```bash
# 建议放环境变量，不要写进代码
export SUPABASE_URL="https://your-instance.example.com"
export SUPABASE_ANON_KEY="eyJhbGciOi..."
export SUPABASE_SERVICE_ROLE_KEY="eyJhbGciOi..."
```

> ⚠️ **anon key 不能直接用来操作沙箱。** 它是公开凭据，网关会拒绝（403）。它的作用是帮用户登录，换出 session token —— 见模式 1。

## 二、安装

从 GitHub 安装：

```bash
git clone https://github.com/volcengine/byted-supabase-cli.git
npm install ./byted-supabase-cli/sandbox_sdk/node e2b
```

按需追加：

```bash
npm install @supabase/supabase-js      # 模式 1：走用户登录拿 session
npm install @e2b/code-interpreter      # code-interpreter 模板的 runCode
```

## 三、初始化

初始化只做一次（通常在程序启动入口）。三种模式的区别只有两处：**用哪个 key**，以及**创建时传什么 metadata**。

### 模式 1：用登录用户自己的 session 创建（推荐）

每个用户用自己的身份和额度，沙箱归属和权限都自动对齐登录态，前端后端都能用。先用 **anon key** 走 Supabase 登录拿到 session，再把 `access_token` 交给 Sandbox。

```ts
import { createClient } from '@supabase/supabase-js'
import { initBytedSupabaseSandbox } from '@byted-supabase/sandbox'

const supabase = createClient(
  process.env.SUPABASE_URL!,
  process.env.SUPABASE_ANON_KEY!,      // 这里用 anon key
)

// 用户登录，拿到 session
const { data, error } = await supabase.auth.signInWithPassword({
  email: 'user@example.com',
  password: '••••••••',
})
if (error) throw error

console.log(data.session.access_token)  // ← 这就是 Sandbox 要的 key
```

把拿到的 `access_token` 传给 Sandbox 即可：

```ts
initBytedSupabaseSandbox({
  url: process.env.SUPABASE_URL!,
  apiKey: data.session.access_token,
})
```

这个模式下创建沙箱**不需要传 metadata**，归属自动就是这个登录用户：

```ts
import { Sandbox } from 'e2b'

const sbx = await Sandbox.create('base')
```

### 模式 2：后端用 service_role 代表用户创建

后端统一持有 service_role key，为每个终端用户创建沙箱。适合用户不直接登录 Supabase、或归属由您自己的账号体系决定的场景。沙箱归属该用户，沙箱内拿到的是**该用户的身份**，读写数据仍受 RLS 约束。

```ts
import { initBytedSupabaseSandbox } from '@byted-supabase/sandbox'

// service_role key 只能待在后端，绝不下发到浏览器
initBytedSupabaseSandbox({
  url: process.env.SUPABASE_URL!,
  apiKey: process.env.SUPABASE_SERVICE_ROLE_KEY!,
})
```

创建时用 `metadata.ownerUserId` 标明归属：

```ts
import { Sandbox } from 'e2b'

const sbx = await Sandbox.create('base', {
  metadata: { ownerUserId: user.id },   // user.id 来自您自己的登录态
})
```

### 模式 3：service_role + 让沙箱也拿到 service_role

沙箱内的代码需要**绕过 RLS** 做管理级操作时用（例如批量数据处理）。初始化与模式 2 相同，区别在创建时显式开启：

```ts
const sbx = await Sandbox.create('base', {
  metadata: { passServiceRoleJwtToSandbox: 'true' },
})
```

> ⚠️ 这会把**绕过 RLS 的最高权限**交给沙箱里运行的代码。只在您完全信任沙箱内容时使用；能用前两种模式走用户身份的场景就不要开它。
>
> 它与 `ownerUserId` **互斥**，同时传会被拒绝。

### 三种模式对照

| | 用的 key | 创建时传 | 沙箱归属 | 沙箱内身份 |
|---|---|---|---|---|
| **模式 1**（推荐） | 用户 session token | 不用传 | 登录用户 | 该用户，受 RLS |
| **模式 2** | service_role | `ownerUserId` | 指定的用户 | 该用户，受 RLS |
| **模式 3** | service_role | `passServiceRoleJwtToSandbox` | 无（平台账户） | service_role，**绕过 RLS** |

## 四、示例代码

以下示例都**假设初始化已完成**（见上一节），直接从创建沙箱开始。

### base —— 通用 Linux 沙箱

不传模板名就是 base。适合执行命令、读写文件、跑任意程序。

```ts
import { Sandbox } from 'e2b'

const sbx = await Sandbox.create()      // 不传模板名 = base
try {
  // 执行命令
  const info = await sbx.commands.run('uname -a')
  console.log(info.stdout)

  // 写文件再执行
  await sbx.files.write('/tmp/hello.py', "print('hi from sandbox')")
  const out = await sbx.commands.run('python3 /tmp/hello.py')
  console.log(out.stdout)

  // 读文件
  console.log(await sbx.files.read('/tmp/hello.py'))
} finally {
  await sbx.kill()                      // 用完记得关，按存活时长计费
}
```

### code-interpreter —— 执行 Python 并拿到结构化结果

预装 Jupyter 内核，可以用 `runCode`。与 `commands.run` 的区别：**变量在多次调用之间保留**，并且图表、表格等富输出会直接作为结果返回。

```ts
import { Sandbox as CodeInterpreter } from '@e2b/code-interpreter'

const sbx = await CodeInterpreter.create('code-interpreter')
try {
  await sbx.runCode('import numpy as np; data = np.arange(10)')

  // 上一次定义的 data 仍然存在
  const sum = await sbx.runCode('print(data.sum())')
  console.log(sum.logs.stdout)
} finally {
  await sbx.kill()
}
```

> ❗ `runCode` **只在 code-interpreter 模板上可用**。base 镜像不带内核，调用会失败。

### codex —— OpenAI Codex CLI

```ts
import { Sandbox } from 'e2b'

const sbx = await Sandbox.create('codex')
try {
  const cmd = await sbx.commands.run(
    'codex exec --skip-git-repo-check "你好，很高兴认识你"',
    { timeoutMs: 300_000 },             // AI 推理慢，务必放宽
  )
  console.log(cmd.stdout)
} finally {
  await sbx.kill()
}
```

### claude —— Claude Code CLI

```ts
const sbx = await Sandbox.create('claude')
try {
  const cmd = await sbx.commands.run(
    'claude -p "你好，很高兴认识你"',
    { timeoutMs: 300_000 },
  )
  console.log(cmd.stdout)
} finally {
  await sbx.kill()
}
```

### opencode —— opencode CLI

```ts
const sbx = await Sandbox.create('opencode')
try {
  const cmd = await sbx.commands.run(
    'opencode run "你好，很高兴认识你"',
    { timeoutMs: 300_000 },
  )
  console.log(cmd.stdout)
} finally {
  await sbx.kill()
}
```

> ✅ 三个 Agent 模板**不需要您自己的模型 API Key**。Supabase 提供了访问大模型功能，开箱即用。
>
> ❗ 但它们都要真的向模型发请求，**SDK 默认 60 秒的命令超时不够用**，必须显式放宽到 5 分钟量级。

## 五、在沙箱里访问您的 Supabase

平台会向沙箱注入两个环境变量，沙箱里的代码可以直接用它们回连您的实例（查表、写 Storage 等）：

| 环境变量 | 含义 |
|---|---|
| `SUPABASE_URL` | 您的实例 API 地址 |
| `SANDBOX_JWT` | 沙箱的身份凭据，权限由创建时选的模式决定（见三种模式对照表）。由平台签发、有效期 24 小时，您不需要管理它的刷新 |

沙箱内的用法就是标准 Supabase 客户端：

```ts
const sbx = await Sandbox.create('base', { metadata: { ownerUserId: user.id } })

await sbx.files.write('/tmp/query.py', `
import os, urllib.request, json

req = urllib.request.Request(
    os.environ["SUPABASE_URL"] + "/rest/v1/todos?select=*",
    headers={
        "apikey": os.environ["SANDBOX_JWT"],
        "Authorization": "Bearer " + os.environ["SANDBOX_JWT"],
    },
)
print(urllib.request.urlopen(req).read().decode())
`)

const out = await sbx.commands.run('python3 /tmp/query.py')
console.log(out.stdout)   // 只会返回这个用户能看到的数据（RLS 生效）
```

## 六、注意事项

- **沙箱有存活时间**，默认 24h 到期自动回收。需要更久就在创建时传 `timeoutMs`，或用 `setTimeout` 续期。
- **anon key 不能操作沙箱**，只能用来登录换 session。
- **service_role key 绝不能出现在前端**，它等同于数据库的最高权限。

---

# Python

## 一、前置准备

您需要的三样东西：

| | 用途 | 能放前端吗 |
|---|---|---|
| **实例 URL** | 您的 Supabase 实例地址 | 可以 |
| **anon key** | 走用户登录、拿 session（模式 1 用） | 可以 |
| **service_role key** | 后端代表用户创建沙箱（模式 2、3 用） | **绝对不行** |

```bash
# 建议放环境变量，不要写进代码
export SUPABASE_URL="https://your-instance.example.com"
export SUPABASE_ANON_KEY="eyJhbGciOi..."
export SUPABASE_SERVICE_ROLE_KEY="eyJhbGciOi..."
```

> ⚠️ **anon key 不能直接用来操作沙箱。** 它是公开凭据，网关会拒绝（403）。它的作用是帮用户登录，换出 session token —— 见模式 1。

## 二、安装

从 GitHub 安装：

```bash
pip install "git+https://github.com/volcengine/byted-supabase-cli.git#subdirectory=sandbox_sdk/python"
```

按需追加：

```bash
pip install supabase                 # 模式 1：走用户登录拿 session
pip install e2b-code-interpreter     # code-interpreter 模板的 run_code
```

## 三、初始化

初始化只做一次（通常在程序启动入口）。三种模式的区别只有两处：**用哪个 key**，以及**创建时传什么 metadata**。

### 模式 1：用登录用户自己的 session 创建（推荐）

每个用户用自己的身份和额度，沙箱归属和权限都自动对齐登录态。先用 **anon key** 走 Supabase 登录拿到 session，再把 `access_token` 交给 Sandbox。

```python
import os
from supabase import create_client

supabase = create_client(
    os.environ["SUPABASE_URL"],
    os.environ["SUPABASE_ANON_KEY"],     # 这里用 anon key
)

# 用户登录，拿到 session
res = supabase.auth.sign_in_with_password({
    "email": "user@example.com",
    "password": "••••••••",
})

print(res.session.access_token)          # ← 这就是 Sandbox 要的 key
```

把拿到的 `access_token` 传给 Sandbox 即可：

```python
from byted_supabase_sandbox import init_byted_supabase_sandbox

init_byted_supabase_sandbox(
    url=os.environ["SUPABASE_URL"],
    api_key=res.session.access_token,
)
```

这个模式下创建沙箱**不需要传 metadata**，归属自动就是这个登录用户：

```python
from e2b import Sandbox

sbx = Sandbox.create("base")
```

### 模式 2：后端用 service_role 代表用户创建

后端统一持有 service_role key，为每个终端用户创建沙箱。适合用户不直接登录 Supabase、或归属由您自己的账号体系决定的场景。沙箱归属该用户，沙箱内拿到的是**该用户的身份**，读写数据仍受 RLS 约束。

```python
import os
from byted_supabase_sandbox import init_byted_supabase_sandbox

# service_role key 只能待在后端，绝不下发到客户端
init_byted_supabase_sandbox(
    url=os.environ["SUPABASE_URL"],
    api_key=os.environ["SUPABASE_SERVICE_ROLE_KEY"],
)
```

创建时用 `metadata.ownerUserId` 标明归属：

```python
from e2b import Sandbox

sbx = Sandbox.create("base", metadata={"ownerUserId": user_id})   # user_id 来自您自己的登录态
```

### 模式 3：service_role + 让沙箱也拿到 service_role

沙箱内的代码需要**绕过 RLS** 做管理级操作时用（例如批量数据处理）。初始化与模式 2 相同，区别在创建时显式开启：

```python
sbx = Sandbox.create("base", metadata={"passServiceRoleJwtToSandbox": "true"})
```

> ⚠️ 这会把**绕过 RLS 的最高权限**交给沙箱里运行的代码。只在您完全信任沙箱内容时使用；能用前两种模式走用户身份的场景就不要开它。
>
> 它与 `ownerUserId` **互斥**，同时传会被拒绝。

### 三种模式对照

| | 用的 key | 创建时传 | 沙箱归属 | 沙箱内身份 |
|---|---|---|---|---|
| **模式 1**（推荐） | 用户 session token | 不用传 | 登录用户 | 该用户，受 RLS |
| **模式 2** | service_role | `ownerUserId` | 指定的用户 | 该用户，受 RLS |
| **模式 3** | service_role | `passServiceRoleJwtToSandbox` | 无（平台账户） | service_role，**绕过 RLS** |

## 四、示例代码

以下示例都**假设初始化已完成**（见上一节），直接从创建沙箱开始。

### base —— 通用 Linux 沙箱

不传模板名就是 base。适合执行命令、读写文件、跑任意程序。

```python
from e2b import Sandbox

with Sandbox.create() as sbx:            # 不传模板名 = base，退出 with 自动回收
    # 执行命令
    info = sbx.commands.run("uname -a")
    print(info.stdout)

    # 写文件再执行
    sbx.files.write("/tmp/hello.py", "print('hi from sandbox')")
    out = sbx.commands.run("python3 /tmp/hello.py")
    print(out.stdout)

    # 读文件
    print(sbx.files.read("/tmp/hello.py"))
```

### code-interpreter —— 执行 Python 并拿到结构化结果

预装 Jupyter 内核，可以用 `run_code`。与 `commands.run` 的区别：**变量在多次调用之间保留**，并且图表、表格等富输出会直接作为结果返回。

```python
from e2b_code_interpreter import Sandbox as CodeInterpreter

with CodeInterpreter.create("code-interpreter") as sbx:
    sbx.run_code("import numpy as np; data = np.arange(10)")

    # 上一次定义的 data 仍然存在
    execution = sbx.run_code("print(data.sum())")
    print(execution.logs.stdout)
```

> ❗ `run_code` **只在 code-interpreter 模板上可用**。base 镜像不带内核，调用会失败。

### codex —— OpenAI Codex CLI

```python
from e2b import Sandbox

with Sandbox.create("codex") as sbx:
    cmd = sbx.commands.run(
        'codex exec --skip-git-repo-check "你好，很高兴认识你"',
        timeout=300,                     # AI 推理慢，务必放宽（单位：秒）
    )
    print(cmd.stdout)
```

### claude —— Claude Code CLI

```python
with Sandbox.create("claude") as sbx:
    cmd = sbx.commands.run(
        'claude -p "你好，很高兴认识你"',
        timeout=300,
    )
    print(cmd.stdout)
```

### opencode —— opencode CLI

```python
with Sandbox.create("opencode") as sbx:
    cmd = sbx.commands.run(
        'opencode run "你好，很高兴认识你"',
        timeout=300,
    )
    print(cmd.stdout)
```

> ✅ 三个 Agent 模板**不需要您自己的模型 API Key**。Supabase 提供了访问大模型功能，开箱即用。
>
> ❗ 但它们都要真的向模型发请求，**SDK 默认 60 秒的命令超时不够用**，必须显式放宽到 5 分钟量级。

## 五、在沙箱里访问您的 Supabase

平台会向沙箱注入两个环境变量，沙箱里的代码可以直接用它们回连您的实例（查表、写 Storage 等）：

| 环境变量 | 含义 |
|---|---|
| `SUPABASE_URL` | 您的实例 API 地址 |
| `SANDBOX_JWT` | 沙箱的身份凭据，权限由创建时选的模式决定（见三种模式对照表）。由平台签发、有效期 24 小时，您不需要管理它的刷新 |

沙箱内的用法就是标准 Supabase 客户端：

```python
with Sandbox.create("base", metadata={"ownerUserId": user_id}) as sbx:
    sbx.files.write("/tmp/query.py", """
import os, urllib.request

req = urllib.request.Request(
    os.environ["SUPABASE_URL"] + "/rest/v1/todos?select=*",
    headers={
        "apikey": os.environ["SANDBOX_JWT"],
        "Authorization": "Bearer " + os.environ["SANDBOX_JWT"],
    },
)
print(urllib.request.urlopen(req).read().decode())
""")

    out = sbx.commands.run("python3 /tmp/query.py")
    print(out.stdout)   # 只会返回这个用户能看到的数据（RLS 生效）
```

## 六、注意事项

- **沙箱有存活时间**，默认 24h 到期自动回收。需要更久就在创建时传 `timeout`（单位：秒），或用 `set_timeout` 续期。
- **anon key 不能操作沙箱**，只能用来登录换 session。
- **service_role key 绝不能出现在前端**，它等同于数据库的最高权限。
