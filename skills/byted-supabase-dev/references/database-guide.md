## 数据库工作流

执行 SQL 前明确数据库、schema、表和影响范围；先用只读 SQL 检查现状。对 `UPDATE`、`DELETE`、`DROP`、`ALTER` 和批量写入先说明影响并取得确认，优先使用事务或幂等 SQL，完成后再次查询验证。

示例：

```bash
byted-supabase-cli db query "select current_database(), current_user"
```

涉及 REST 暴露的表时检查：

- schema/table 是否暴露给 Data API；
- `anon`/`authenticated` 所需的 `GRANT`；
- RLS 是否启用；
- policy 是否覆盖真实的所有权和写入路径。

不要用 `service_role` 绕过问题来替代正确的 RLS 或权限设计。需要类型生成或数据库检查时，先执行对应命令的 `--help`。
