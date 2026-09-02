# Edge Function 开发指南

本指南介绍 Edge Function 的编写规范和常见模式。本 fork 的部署从**本地函数目录**进行：`byted-supabase-cli functions new <name>` 生成 `supabase/functions/<name>/index.ts` 脚手架 → 编辑 → `byted-supabase-cli functions deploy <name>`。

---

## 目录

- 基础结构
- CORS 配置
- 请求处理
- 带数据库访问
- 常见模式
- 部署与管理
- 注意事项

## 基础结构

每个 Edge Function 是一个 Deno 函数，使用 `Deno.serve()` 处理 HTTP 请求：

```typescript
Deno.serve(async (req: Request) => {
  return new Response(JSON.stringify({ message: 'Hello World' }), {
    headers: { 'Content-Type': 'application/json' },
  });
});
```

部署：

```bash
byted-supabase-cli functions new hello                     # 生成 supabase/functions/hello/index.ts
# 把上面的代码写入该文件后部署
byted-supabase-cli functions deploy hello --workspace-id ws-xxxx
```

---

## CORS 配置

大多数前端调用都需要 CORS 支持：

```typescript
const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
  'Access-Control-Allow-Headers': 'Authorization, Content-Type, x-client-info, apikey',
};

Deno.serve(async (req: Request) => {
  // 处理 CORS 预检请求
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders });
  }

  try {
    const data = { message: 'Hello World' };

    return new Response(JSON.stringify(data), {
      headers: { ...corsHeaders, 'Content-Type': 'application/json' },
    });
  } catch (error) {
    return new Response(JSON.stringify({ error: error.message }), {
      status: 500,
      headers: { ...corsHeaders, 'Content-Type': 'application/json' },
    });
  }
});
```

---

## 请求处理

### 读取请求体

```typescript
Deno.serve(async (req: Request) => {
  // JSON 请求体
  const body = await req.json();
  const { name, email } = body;

  // URL 参数
  const url = new URL(req.url);
  const page = url.searchParams.get('page') || '1';

  // 请求头
  const authHeader = req.headers.get('Authorization');

  return new Response(JSON.stringify({ name, email, page }), {
    headers: { 'Content-Type': 'application/json' },
  });
});
```

### 路由分发

```typescript
Deno.serve(async (req: Request) => {
  const url = new URL(req.url);
  const path = url.pathname;

  if (req.method === 'GET' && path === '/') {
    return handleList(req);
  }
  if (req.method === 'POST' && path === '/') {
    return handleCreate(req);
  }
  if (req.method === 'PUT') {
    return handleUpdate(req);
  }
  if (req.method === 'DELETE') {
    return handleDelete(req);
  }

  return new Response('Not Found', { status: 404 });
});
```

---

## 带数据库访问

Edge Function 可以通过 Supabase SDK 或直接使用环境变量中的连接信息访问数据库：

默认情况下，应使用 `SUPABASE_ANON_KEY` 搭配请求中的用户 JWT，让数据库 RLS
执行最终的数据访问控制。`Access-Control-Allow-Origin: *` 只处理浏览器跨域
策略，不是身份认证或授权机制，不能替代 JWT 校验。

```typescript
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2';

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'Authorization, Content-Type, x-client-info, apikey',
};

Deno.serve(async (req: Request) => {
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders });
  }

  try {
    const supabaseUrl = Deno.env.get('SUPABASE_URL')!;
    const authHeader = req.headers.get('Authorization');
    const tokenMatch = authHeader?.match(/^Bearer\s+(.+)$/i);
    if (!tokenMatch) {
      return new Response(JSON.stringify({ error: 'Unauthorized' }), {
        status: 401,
        headers: { ...corsHeaders, 'Content-Type': 'application/json' },
      });
    }

    // 使用 anon key 搭配用户 JWT；后续查询会尊重 RLS
    const userClient = createClient(
      supabaseUrl,
      Deno.env.get('SUPABASE_ANON_KEY')!,
      { global: { headers: { Authorization: `Bearer ${tokenMatch[1]}` } } },
    );
    const { data: user, error: userError } = await userClient.auth.getUser();
    if (userError || !user.user) {
      return new Response(JSON.stringify({ error: 'Unauthorized' }), {
        status: 401,
        headers: { ...corsHeaders, 'Content-Type': 'application/json' },
      });
    }

    const { data, error } = await userClient
      .from('posts')
      .select('*')
      .order('created_at', { ascending: false })
      .limit(10);

    if (error) throw error;

    return new Response(JSON.stringify(data), {
      headers: { ...corsHeaders, 'Content-Type': 'application/json' },
    });
  } catch (error) {
    return new Response(JSON.stringify({ error: error.message }), {
      status: 500,
      headers: { ...corsHeaders, 'Content-Type': 'application/json' },
    });
  }
});
```

### 受信任后台任务

只有在调用方已经是受信任的后台服务，并且应用层已经完成鉴权、授权和
最小权限过滤时，才使用 `SUPABASE_SERVICE_ROLE_KEY`。该密钥会绕过 RLS，
不能放在客户端，也不应直接暴露给不可信调用者。下面的
`assertTrustedBackendRequest` 代表项目自己的服务间认证逻辑，不能省略或
用 CORS 配置替代：

```typescript
const serviceClient = createClient(
  Deno.env.get('SUPABASE_URL')!,
  Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')!,
);

// 必须先校验服务身份、调用权限和租户范围
await assertTrustedBackendRequest(req);
const tenantId = await getAuthorizedTenantId(req);

const { data, error } = await serviceClient
  .from('posts')
  .select('*')
  .eq('tenant_id', tenantId)
  .order('created_at', { ascending: false })
  .limit(10);
```

---

## 常见模式

### CRUD API

```typescript
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2';

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'Authorization, Content-Type, x-client-info, apikey',
};

Deno.serve(async (req: Request) => {
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders });
  }

  const supabase = createClient(
    Deno.env.get('SUPABASE_URL')!,
    Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')!,
  );

  try {
    let result;

    switch (req.method) {
      case 'GET': {
        const url = new URL(req.url);
        const id = url.searchParams.get('id');
        if (id) {
          result = await supabase.from('items').select('*').eq('id', id).single();
        } else {
          result = await supabase.from('items').select('*').order('created_at', { ascending: false });
        }
        break;
      }
      case 'POST': {
        const body = await req.json();
        result = await supabase.from('items').insert(body).select();
        break;
      }
      case 'PUT': {
        const body = await req.json();
        const { id, ...updates } = body;
        result = await supabase.from('items').update(updates).eq('id', id).select();
        break;
      }
      case 'DELETE': {
        const url = new URL(req.url);
        const id = url.searchParams.get('id');
        result = await supabase.from('items').delete().eq('id', id!);
        break;
      }
    }

    if (result?.error) throw result.error;

    return new Response(JSON.stringify(result?.data), {
      headers: { ...corsHeaders, 'Content-Type': 'application/json' },
    });
  } catch (error) {
    return new Response(JSON.stringify({ error: error.message }), {
      status: 500,
      headers: { ...corsHeaders, 'Content-Type': 'application/json' },
    });
  }
});
```

### Webhook 处理器

如果函数通过 `--no-verify-jwt` 部署为公开入口，必须使用 Webhook
提供方的共享密钥验证签名，不能只检查签名请求头是否存在。下面示例使用
`HMAC-SHA256`，签名内容为 `timestamp.nonce.body`，并拒绝超出时间窗口的
请求。生产环境还必须将 nonce 持久化到数据库或其他共享存储中，拒绝已处理
的 nonce；仅使用内存集合无法跨实例可靠防重放。

```typescript
const encoder = new TextEncoder();
const maxWebhookAgeSeconds = 300;

function hexToBytes(value: string): Uint8Array | null {
  if (!/^[0-9a-f]{64}$/i.test(value)) return null;
  const bytes = new Uint8Array(32);
  for (let index = 0; index < bytes.length; index++) {
    bytes[index] = Number.parseInt(value.slice(index * 2, index * 2 + 2), 16);
  }
  return bytes;
}

function timingSafeEqual(left: Uint8Array, right: Uint8Array): boolean {
  if (left.length !== right.length) return false;
  let difference = 0;
  for (let index = 0; index < left.length; index++) {
    difference |= left[index] ^ right[index];
  }
  return difference === 0;
}

async function verifyWebhook(
  body: string,
  timestamp: string,
  nonce: string,
  signature: string,
): Promise<boolean> {
  const timestampSeconds = Number(timestamp);
  if (!Number.isInteger(timestampSeconds)) return false;
  if (Math.abs(Date.now() / 1000 - timestampSeconds) > maxWebhookAgeSeconds) {
    return false;
  }

  const secret = Deno.env.get('WEBHOOK_SECRET');
  if (!secret || !nonce || nonce.length > 256) return false;

  const key = await crypto.subtle.importKey(
    'raw',
    encoder.encode(secret),
    { name: 'HMAC', hash: 'SHA-256' },
    false,
    ['sign'],
  );
  const expected = new Uint8Array(
    await crypto.subtle.sign(
      'HMAC',
      key,
      encoder.encode(`${timestamp}.${nonce}.${body}`),
    ),
  );
  const provided = hexToBytes(signature);
  return provided !== null && timingSafeEqual(expected, provided);
}

Deno.serve(async (req: Request) => {
  if (req.method !== 'POST') {
    return new Response('Method Not Allowed', { status: 405 });
  }

  try {
    // 必须验证原始请求体；解析 JSON 后重新序列化可能改变签名内容
    const body = await req.text();
    const timestamp = req.headers.get('x-webhook-timestamp') ?? '';
    const nonce = req.headers.get('x-webhook-nonce') ?? '';
    const signature = req.headers.get('x-webhook-signature');
    if (
      !signature ||
      !(await verifyWebhook(body, timestamp, nonce, signature)) ||
      !(await isUnprocessedWebhookNonce(nonce))
    ) {
      return new Response('Unauthorized', { status: 401 });
    }

    // isUnprocessedWebhookNonce 必须使用持久化共享存储，并在成功处理前原子占用 nonce
    const payload = JSON.parse(body);

    // 处理事件
    console.log('Received webhook:', payload.type);

    switch (payload.type) {
      case 'order.created':
        // 处理订单创建事件
        break;
      case 'payment.completed':
        // 处理支付完成事件
        break;
      default:
        console.log('Unhandled event type:', payload.type);
    }

    return new Response(JSON.stringify({ received: true }), {
      headers: { 'Content-Type': 'application/json' },
    });
  } catch (error) {
    return new Response(JSON.stringify({ error: error.message }), {
      status: 500,
      headers: { 'Content-Type': 'application/json' },
    });
  }
});
```

### 数据聚合 / 报表

```typescript
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2';

Deno.serve(async (req: Request) => {
  const supabase = createClient(
    Deno.env.get('SUPABASE_URL')!,
    Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')!,
  );

  try {
    // 使用 RPC 调用数据库函数进行聚合
    const { data: stats, error } = await supabase.rpc('get_dashboard_stats');
    if (error) throw error;

    return new Response(JSON.stringify(stats), {
      headers: {
        'Content-Type': 'application/json',
        'Cache-Control': 'public, max-age=300', // 缓存 5 分钟
      },
    });
  } catch (error) {
    return new Response(JSON.stringify({ error: error.message }), {
      status: 500,
      headers: { 'Content-Type': 'application/json' },
    });
  }
});
```

---

## 部署与管理

```bash
# 1. 新建函数脚手架（本地 supabase/functions/my-api/index.ts）
byted-supabase-cli functions new my-api
# 2. 编辑该文件写入逻辑，然后部署
byted-supabase-cli functions deploy my-api --workspace-id ws-xxxx

# 部署时禁用 JWT 验证（公开 API，如 Webhook）
byted-supabase-cli functions deploy public-api --workspace-id ws-xxxx --no-verify-jwt

# 指定运行时（默认 auto；可选 deno / native-node20/v1 / python3.9|3.10|3.12）
byted-supabase-cli functions deploy my-api --workspace-id ws-xxxx --runtime deno

# 查看已部署函数
byted-supabase-cli functions list --workspace-id ws-xxxx -o json

# 拉取线上函数源码到本地
byted-supabase-cli functions download my-api --workspace-id ws-xxxx

# 删除函数
byted-supabase-cli functions delete my-api --workspace-id ws-xxxx
```

---

## 注意事项

- **JWT 验证**：默认启用。如果函数需要公开访问（如 Webhook），使用 `--no-verify-jwt`
- **函数命名**：只能使用小写字母、数字和连字符（如 `my-api`、`process-order`）
- **超时**：Edge Function 有执行时间限制，避免长时间阻塞操作
- **日志**：使用 `console.log()` 记录日志，可在平台控制台查看
- **依赖导入**：使用 `https://esm.sh/` 导入 npm 包，或使用 `https://deno.land/` 导入 Deno 标准库
