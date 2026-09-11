# Pages 数据面指南

## 适用范围

CLI的 Pages 命令使用当前 `link` profile 中的 Supabase URL 和
`service-role key`，直接操作当前已绑定的 Supabase 实例。先检查：

```bash
byted-supabase-cli link status -o json
byted-supabase-cli pages list -o json
byted-supabase-cli pages binding -o json
```


不要使用 Workspace/Manager Tree 命令处理当前实例的 Pages 操作；也不要把
 Tree 的 `pages` 命令与控制面版本混用。

## 命令树

```text
pages
├── list
├── binding
├── bind <pages-project-id>
├── create <project-name>
├── upload <file>
├── deploy <pages-project-id>
│   └── list <pages-project-id>
├── env-vars
├── sync
├── unbind
└── fast
    └── create <project-name>
```

## 手动部署流程

只部署前端到当前已 link 的 Supabase 实例时，使用：

```bash
# 1. 上传构建好的静态站点归档，取得 resource id
byted-supabase-cli pages upload <archive-or-file> -o json

# 2. 创建 Pages 项目；传 resource id 时会发起首个部署
byted-supabase-cli pages create <project-name> \
  --resource-id <project-deploy-resource-id> \
  -o json

# 3. 将 Pages 项目绑定到当前 Supabase 实例
byted-supabase-cli pages bind <pages-project-id> \
  --framework-prefix NEXT_PUBLIC_

# 4. 后续资源部署
byted-supabase-cli pages upload <archive-or-file> -o json
byted-supabase-cli pages deploy <pages-project-id> \
  --resource-id <project-deploy-resource-id> \
  -o json

# 5. 查询部署历史
byted-supabase-cli pages deploy list <pages-project-id> -o json
```

`pages bind` 是建立 Pages 与当前 Supabase 实例关联的关键步骤。前端需要
使用 Supabase URL、anon key、Auth、REST、Storage 或 Edge Functions 时，
不要跳过 bind；`--framework-prefix` 用于指定浏览器端环境变量前缀，例如：

- Next.js：`NEXT_PUBLIC_`
- Vite：`VITE_`

如果同一个 Pages 项目已经被其他 Supabase 分支使用，可使用
`--custom-prefix <prefix>` 区分注入的变量名：

```bash
byted-supabase-cli pages bind <pages-project-id> \
  --framework-prefix NEXT_PUBLIC_ \
  --custom-prefix <prefix>
```

上传、创建、绑定和部署都会产生线上变更，执行前确认资源文件、项目名称和
Pages 项目 ID。

## Pages 项目

列出当前账号可见的 Pages 项目：

```bash
byted-supabase-cli pages list -o json
byted-supabase-cli pages list --name <name> --limit 10 --offset 0 -o json
```

分页参数：

- `--limit` 默认 `10`
- `--limit` 必须大于 `0`
- `--offset` 不能为负数
- `--offset` 必须是 `--limit` 的整数倍

上传站点产物：

```bash
byted-supabase-cli pages upload <file> -o json
```

`upload` 返回的 `ProjectDeployResourceID` 用于 `pages create` 或
`pages deploy`。

创建 Pages 项目：

```bash
byted-supabase-cli pages create <project-name> \
  --resource-id <project-deploy-resource-id> \
  --scope domestic \
  --framework nextjs \
  --root-dir frontend \
  --output-dir out \
  --build-cmd 'npm run build' \
  --install-cmd 'npm ci' \
  --nodejs-version 20 \
  -o json
```

可用参数：

- `--scope`：`domestic`、`overseas` 或 `global`，默认 `domestic`
- `--resource-id`：`pages upload` 返回的资源 ID
- `--framework`
- `--root-dir`
- `--output-dir`
- `--build-cmd`
- `--install-cmd`
- `--nodejs-version`

当前  Tree 的 `pages create` 不提供 Manager Tree 版本的
`--provider` 或 `--no-deploy` 参数。传入 `--resource-id` 时创建请求使用该
资源；不传时由命令实现决定是否创建未部署项目，执行前以本地
`pages create --help` 为准。

创建部署：

```bash
byted-supabase-cli pages deploy <pages-project-id> \
  --resource-id <project-deploy-resource-id> \
  -o json
```

部署历史：

```bash
byted-supabase-cli pages deploy list <pages-project-id> \
  --limit 10 --offset 0 -o json
```

## 绑定和环境变量

查看当前实例的 Pages 绑定：

```bash
byted-supabase-cli pages binding -o json
```

绑定响应包含当前实例对应的 Pages 项目及其部署状态。绑定操作：

```bash
byted-supabase-cli pages bind <pages-project-id> \
  [--framework-prefix NEXT_PUBLIC_] \
  [--custom-prefix <prefix>]
```

同步当前实例环境变量到已绑定的 Pages 项目：

```bash
byted-supabase-cli pages sync \
  --pages-project-id <pages-project-id> \
  -o json
```

 Tree 的 `pages sync` 要求显式提供 `--pages-project-id`；没有绑定项目
或项目 ID 缺失时会失败。

 Tree 的 `pages env-vars` 当前不可用，会明确返回：

```text
pages env-vars is not available for a directly linked instance;
the Pages API exposes only env-vars sync
```

不要在 Skill 中把 `pages env-vars --workspace-id ...` 作为  用法。需要
查询或显示具体环境变量时，应使用支持该能力的 Manager Tree 命令，并先确认
当前运行模式和认证上下文。

解除当前实例与 Pages 的绑定：

```bash
byted-supabase-cli pages unbind
```

这是破坏性配置变更，执行前确认当前绑定关系。

## 快速流程

```bash
byted-supabase-cli pages fast create <project-name> \
  --file-path <frontend-directory-or-archive>
```

 Tree 的 `pages fast create` 固定执行当前实例范围内的：

```text
upload → create Pages project → deploy
```

它不会新建 Supabase Workspace、Branch 或 Compute，也不执行 migrations 或
Edge Functions 初始化。它当前只支持：

- `--file-path`

如果需要 `migrations-init`、`functions-init`、新建 Workspace，使用
Manager Tree 的 Pages workflow，而不是当前  Tree。

## 资料盒子 Demo

官方资料盒子（filebox / Supabase Tools）是一个 **Next.js + Supabase**
演示应用，包含以下能力：

- 文件上传
- 文件共享
- AI 问答
- Supabase Auth 登录
- Postgres 数据存储
- Storage 文件存储
- Edge Functions 后端处理
- AI Gateway 调用

资料盒子压缩包下载地址：

```text
https://lf3-static.bytednsdoc.com/obj/eden-cn/whkph/ljhwZthlaukjlkulzlp/nextjs-supabase-filebox.zip
```

解压后的目录约定如下：

```text
nextjs-supabase-filebox/
├── frontend/
│   └── Next.js 前端应用
├── backend/
│   └── functions/
│       └── processing-worker/
└── migrations/
    └── *.sql
```

其中：

- `frontend/` 是需要部署到 Pages 的静态站点或前端项目
- `backend/functions/processing-worker/` 是资料处理 Edge Function
- `migrations/` 是初始化数据库结构和数据的 SQL 文件
- Next.js 前端通常使用 `NEXT_PUBLIC_` 作为浏览器端环境变量前缀
- AI 问答功能使用当前 Supabase 实例的 AI Gateway

下载并查看 Demo：

```bash
curl -fsSL \
  https://lf3-static.bytednsdoc.com/obj/eden-cn/whkph/ljhwZthlaukjlkulzlp/nextjs-supabase-filebox.zip \
  -o filebox.zip
unzip -q filebox.zip
```

### 部署限制

当前 Tree 的 `pages fast create` 只支持：

```bash
byted-supabase-cli pages fast create <project-name> \
  --file-path nextjs-supabase-filebox/frontend
```

它只执行前端资源上传、Pages 项目创建和部署，不会自动执行资料盒子的
`migrations/`，也不会部署 `processing-worker` Edge Function。

### 快速部署资料盒子前端

如果当前 Supabase 实例的数据库和 `processing-worker` 已经准备好，只需要
快速发布资料盒子前端，可以按以下步骤操作：

```bash
# 1. 下载并解压资料盒子
curl -fsSL \
  https://lf3-static.bytednsdoc.com/obj/eden-cn/whkph/ljhwZthlaukjlkulzlp/nextjs-supabase-filebox.zip \
  -o filebox.zip
unzip -q filebox.zip

# 2. 快速创建并部署 Pages 项目
byted-supabase-cli pages fast create filebox \
  --file-path nextjs-supabase-filebox/frontend
```

`fast create` 输出中的 `PagesProjectID` 用于后续绑定。由于 `fast create`
只负责上传、创建和部署，前端要真正连接当前 Supabase 实例，还必须执行
绑定：

```bash
# 3. 将 Pages 项目绑定到当前实例
byted-supabase-cli pages bind <pages-project-id> \
  --framework-prefix NEXT_PUBLIC_

# 4. 绑定后重新上传并部署，使注入的 Supabase 环境变量进入部署
byted-supabase-cli pages upload nextjs-supabase-filebox/frontend -o json
byted-supabase-cli pages deploy <pages-project-id> \
  --resource-id <project-deploy-resource-id> \
  -o json

# 5. 获取部署状态和短效预览地址
byted-supabase-cli pages deploy list <pages-project-id> -o json
byted-supabase-cli pages binding -o json
```

其中：

- `--framework-prefix NEXT_PUBLIC_` 适用于资料盒子的 Next.js 前端；
- `<project-deploy-resource-id>` 是第二次 `pages upload` 返回的资源 ID；
- `pages binding` 返回的 `PagesProject.PreviewDomain` 才是可直接打开的短效预览地址；
- 如果不执行 `pages bind`，Pages 项目不会注入当前 Supabase 的 URL、anon key
  等环境变量，资料盒子前端无法连接当前后端。

如果当前 Supabase 实例已经准备好数据库和 Edge Function，推荐只把
`frontend/` 作为 Pages 资源部署，然后执行：

```bash
byted-supabase-cli pages upload nextjs-supabase-filebox/frontend
byted-supabase-cli pages create <project-name> \
  --resource-id <project-deploy-resource-id>
byted-supabase-cli pages bind <pages-project-id> \
  --framework-prefix NEXT_PUBLIC_
byted-supabase-cli pages deploy <pages-project-id> \
  --resource-id <project-deploy-resource-id>
```

如果从零开始，需要同时执行 SQL migration、部署
`processing-worker` 并创建新的 Supabase Workspace，应使用 Manager Tree
对应的完整 Pages workflow；不要把 `--migrations-init` 或
`--functions-init` 传给当前  Tree 命令。

## 预览地址

`pages deploy` 不负责返回可直接访问的预览地址。部署后执行：

```bash
byted-supabase-cli pages binding -o json
```

从绑定响应的 `PagesProject.PreviewDomain` 获取预览地址。

注意：

- `PreviewDomain` 是短效、一次性的预览链接，通常携带 `iga_token` 和
  `iga_time`
- 拿到后应立即交给用户打开
- 不要缓存或长期复用该 URL
- 需要再次访问时重新执行 `pages binding`
- `pages list` 或 `pages deploy list` 中的基础域名通常不带访问 token，
  不能替代 `pages binding` 返回的可访问预览链接

## 安全和排障

- `401` / `403`：检查当前  profile 的 `service-role key` 和 Supabase URL。
- `404`：确认当前实例是否已部署对应 Pages 数据面路由。
- `pages bind` 失败：先确认 Pages 项目 ID，并检查当前实例是否已有冲突绑定。
- `pages sync` 失败：先执行 `pages binding`，确认目标 Pages 项目 ID。
- 报告和日志中不要写入 service-role key、database URL 或完整预览 token URL。
- 执行创建、上传、绑定、部署、同步或解绑前，先确认用户明确要求和目标。
