# Sandbox Hello World for Python

这是一个使用火山引擎 Supabase Sandbox 的最小 Python 示例：

```text
初始化 SDK → 创建 Sandbox → 执行 Python → 验证输出 → 回收 Sandbox
```

## 前置条件

- Python 3.10+
- 已安装并登录 `byted-supabase-cli`
- 一个已就绪的 Supabase 分支
- 后端或可信 Worker 环境

这是四方用户创建 Sandbox 的最小完整示例。运行时会使用测试账号注册（账号已存在时跳过注册）、登录并取得用户 session，再创建 Sandbox。Sandbox 内的请求会遵循该用户的 RLS 权限。不要使用 service-role key 或 CLI 的 `envdAccessToken` 替代它。

## 快速准备环境变量

下面的命令使用一个真实 workspace 和一个 `Ready` 状态的分支。先设置 workspace：

```bash
export WORKSPACE_ID="<workspace-id>"
```

### 1. 选择分支和模板

```bash
byted-supabase-cli branches list \
  --workspace-id "$WORKSPACE_ID" \
  --limit 10 \
  -o json

export BRANCH_ID="<上一步选中的 Ready 分支 ID>"

byted-supabase-cli templates list \
  --workspace-id "$WORKSPACE_ID" \
  --branch-id "$BRANCH_ID" \
  -o json
```

确认模板列表中有 `base`。本示例固定创建 `base` Sandbox。

### 2. 获取 `SUPABASE_URL`

```bash
byted-supabase-cli endpoints list \
  --workspace-id "$WORKSPACE_ID"
```

从输出中找到 `$BRANCH_ID` 对应的 API endpoint（通常显示为
`AddressDomain`），然后设置：

```bash
export SUPABASE_URL="https://<branch-host>"
```

不要把控制台地址、workspace 地址或数据库连接串填入这里；这里需要的是分支的
Supabase API 地址。

### 3. 配置测试账号

自动登录需要分支的 anon key，以及一个测试账号。anon key 可以从以下命令结果中取得（只取 `anon`，不要把 `service_role` 放入示例）：

```bash
byted-supabase-cli projects api-keys \
  --workspace-id "$WORKSPACE_ID" \
  --branch-id "$BRANCH_ID" \
  --reveal \
  -o json
```

把 `SUPABASE_ANON_KEY`、`SUPABASE_EMAIL` 和 `SUPABASE_PASSWORD` 写入 `.env`。示例会自动执行 `注册 → 登录 → 创建 Sandbox`。如果项目开启了邮箱确认，请先在 Auth 中确认这个测试账号。

如果调用方已经有用户 session，也可以只设置 `SUPABASE_ACCESS_TOKEN`，跳过注册和登录；此时不需要邮箱密码。不要提交 `.env`、打印 access token 或将 token 发送到聊天中。

### 4. 写入 `.env`

```bash
cd skills/byted-supabase/examples/hello-world-python
cp .env.example .env
```

把 `.env` 中的两个占位符替换为上面得到的值，然后加载：

```bash
set -a
source .env
set +a
```

`.env` 只用于本地测试，不能提交到 Git。

## 安装

建议使用虚拟环境：

```bash
cd skills/byted-supabase/examples/hello-world-python
python3 -m venv .venv
source .venv/bin/activate
python -m pip install --upgrade pip
python -m pip install -e .
```

依赖会从仓库安装 `byted-supabase-sandbox`，并将官方 E2B SDK 约束在`2.31 <= e2b < 2.51`，避免使用当前尚不兼容的 `2.51+`。

## 配置并运行

```bash
cp .env.example .env
# 编辑 .env，填入真实值
set -a
source .env
set +a
python main.py
```

预期输出包含：

```text
hello world
sandbox verification passed
```

`with Sandbox.create(...)` 会在成功和异常路径自动回收 Sandbox。CLI 的 `envdAccessToken` 是数据面凭据，不能作为这个 SDK 的 `api_key`。
