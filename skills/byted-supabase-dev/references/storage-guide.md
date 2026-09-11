# Storage 服务指南

## 只读检查

```bash
byted-supabase-cli storage buckets list -o json
byted-supabase-cli storage buckets get <bucket> -o json
byted-supabase-cli storage ls ss:///bucket/path --output json
```

先确认 bucket 名称、public 配置、对象前缀和当前身份。对象路径使用 `ss:///bucket/path` 形式，执行前查看对应命令的 `--help`。

## 上传和复制

```bash
byted-supabase-cli storage cp <local-file> ss:///bucket/path
byted-supabase-cli storage cp ss:///bucket/path <local-file>
byted-supabase-cli storage cp -r <local-dir> ss:///bucket/prefix
```

上传前确认是否覆盖已有对象、Content-Type、Cache-Control 和目标前缀。递归上传需要先评估文件数量和总大小。

## 移动和删除

```bash
byted-supabase-cli storage mv ss:///bucket/source ss:///bucket/destination
byted-supabase-cli storage rm ss:///bucket/path --yes
```

移动、覆盖和删除是写操作；批量或递归删除必须明确确认，并先执行等价的 list/count 检查。不要仅凭对象不存在错误扩大删除范围。

## 权限排查

Storage 操作可能同时受 bucket 配置、对象路径、Data API 权限和 RLS 影响。upsert 常需要 INSERT、SELECT、UPDATE 权限；只给 INSERT 可能导致覆盖失败。使用 service-role key 排查时，不要把它作为前端访问方案。

## 验证

写入后用 `storage ls` 或下载单个对象验证；不要把对象内容、签名 URL 或密钥写入报告。
