# `byted-supabase-cli login` 登录方式与行为规格

本文档说明 `login` 命令的所有登录方式及其在**交互式 / 非交互式（含 AI agent、Coze 等托管环境）**下的行为。命令参考见 [`supabase/login.md`](supabase/login.md)。

## 一、两个正交维度

登录行为由两件事决定：

| 维度 | 取值 | 判定方式 |
|---|---|---|
| **登录模式** | 本地（默认） / 远程 `--remote` | 由 `--remote` flag |
| **交互性** | 交互式 / 非交互式 | `!term.IsTerminal(os.Stdin.Fd())`（stdin 非 TTY，等价 `test -t 0`）**或** `--agent yes` / 已知 agent 环境变量命中 → 非交互 |

> 为什么只看 stdin：要判断的是「能否弹提示并**读到**用户回答」，读发生在 stdin；stdout/stderr 只决定输出去哪，不代表输入可用（例如 `login | tee log` 时 stdout 是管道但 stdin 仍是键盘）。

## 二、四种鉴权方式

### 方式 1 · 本地浏览器登录 `login`（有桌面浏览器）

自动打开本地浏览器 → OAuth 2.0 + PKCE → localhost 回调自动接收授权码 → 完成。

- 适用：本地开发、有 GUI 的机器。
- ⚠️ headless / agent 环境用不了（无浏览器、localhost 回调也够不着），请改用方式 2 或 3。

### 方式 2 · 远程跨设备登录 `login --remote`（无本地浏览器）

打印一个授权 URL，用户在**任意设备的浏览器**用自己的火山账号登录、拿到授权码。**授权码如何回填，由交互性自动决定：**

| 场景 | 行为 |
|---|---|
| **2a. 交互式 TTY**（人在终端） | 打印 URL → 阻塞读 stdin（提示 `Authorization code:`）→ 用户粘贴 → 完成 |
| **2b. 非交互 / agent** | 打印 URL → **自动生成临时文件**（`os.CreateTemp`，名 `byted-supabase-login-*.txt`，权限 0600）→ 打印 `Waiting for authorization code — write it to: <path>` → **轮询该文件** → 有人写入合法授权码 → 完成 → **自动删除该临时文件** |

> 无需任何额外 flag：交互性由 stdin 是否 TTY 自动判定（stdin 非 TTY → 走 2b 自动生成）。

### 方式 3 · 环境变量 / AK-SK（不走 OAuth，适合 CI / 全自动）

```bash
export VOLCENGINE_ACCESS_KEY=...
export VOLCENGINE_SECRET_KEY=...
export VOLCENGINE_REGION=cn-beijing
# 或持久化进 profile：
byted-supabase-cli configure set --access-key ... --secret-key ... [--region ...]
```

直接用密钥鉴权，无任何交互。沙箱 / VeFaaS IAM 等自带角色凭据的环境可不显式设置。

### 方式 4 · 导入已有凭据缓存 `login --credential-file <path>`

把在别处 `login` 成功生成的凭据缓存文件导入当前 profile。不能与 `--remote` 同用。

## 三、文件轮询（方式 2b）细节

- **写入格式**：浏览器给的那串 base64（内容是 `code=<授权码>&state=<状态>`），与交互式粘贴进 stdin 的完全一致。
- **base64 容错**：依次尝试 `StdEncoding` / `RawStdEncoding` / `URLEncoding` / `RawURLEncoding` 四种 → **带不带 `=` 补位、URL-safe 与否都能解**。
- **state 校验**：解出的 `state` 必须与授权 URL 里的一致（防 CSRF），不匹配即报错。
- **轮询参数**：每 1s 读一次；总超时 10 分钟；响应 context 取消。
- **解码失败不再干等**：若文件**内容连续两轮相同却仍解不出**（说明写完了但格式不对，而非写入中的半包），**立即返回格式错误**，不再傻等到超时。
- 换到的是**临时 STS 凭据**，写入 profile 缓存，过期前复用。

## 四、横切行为

- **region（地域）**：
  - `--region` 给了 → 校验并使用；
  - 省略 + 交互式 TTY → 提示 `Please enter region [cn-beijing]:`（回车即默认）；
  - 省略 + 非交互 / agent → **直接默认**，打印 `Using default region: cn-beijing`（不提示、不读 stdin）。
- **确认类操作**（替换已有 login_session、破坏性操作）：`--yes` 自动确认；非交互 / agent 未给 `--yes` → 报错 `confirmation required in non-interactive or agent mode; re-run with --yes`，不阻塞。
- **`--agent` flag**：`auto`（默认，靠环境变量识别已知 agent）/ `yes`（强制非交互）/ `no`（强制交互）。

## 五、Coze / AI agent 环境推荐用法

Coze 编程终端执行命令时 stdin 是非 TTY（`test -t 0` 为 false），因此**裸 `login --remote` 即自动走非交互（2b），无需任何 flag**：

```bash
# ① 后台跑（自动建轮询文件、打印路径，不阻塞对话）
byted-supabase-cli login --remote > /tmp/login.out 2>&1 &
sleep 2; cat /tmp/login.out          # 取授权 URL + "write it to: <path>"

# ② 把 URL 发给用户，用户在自己浏览器用自己火山账号登录、拿到授权串后：
echo "<授权串>" > <上一步打印的 path>   # 后台进程轮询到即完成登录
```

- **多租户隔离**：用谁的火山账号登录，就操作谁的 Supabase —— 每个企业各自用自己账号，资源天然隔离。
- 若某环境的 stdin 是 TTY（如本机终端），加 `--agent yes` 显式强制非交互即可。
- 进程 A（轮询）需在整个登录期间**保活在后台**（用户去浏览器登录可能几分钟）。
