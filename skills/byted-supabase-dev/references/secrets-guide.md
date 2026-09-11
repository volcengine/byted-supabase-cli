# Secrets 服务指南

## 查看名称

```bash
byted-supabase-cli secrets list -o json
```

列表只用于确认 secret 名称和配置状态，不要求服务端回显 secret value。若输出包含 value，必须在报告和聊天中脱敏。

## 设置和删除

```bash
byted-supabase-cli secrets set NAME=<value>
byted-supabase-cli secrets unset NAME --yes
```

执行前确认当前实例、Function 影响范围和 secret 名称。`set` 可能覆盖已有值，`unset` 会让依赖该变量的函数行为改变；两者都需要用户明确确认。

命令行参数可能进入 shell history。生产环境优先使用 CLI 当前版本支持的受控输入或 env-file 方式，并先查看：

```bash
byted-supabase-cli secrets set --help
byted-supabase-cli secrets unset --help
```

## 验证和排障

修改后只验证 secret 名称、函数状态或函数健康行为，不验证 secret 原文。函数拿不到变量时，检查 profile、secret 名称大小写、部署版本和运行时日志，不要把 secret 写进诊断输出。

## 红线

- 不在 Git、Skill、测试 fixture、前端代码或命令输出中保存真实 secret。
- 不使用 service-role key 作为应用运行时环境变量。
- 不为了确认 secret 是否正确而打印或回显它。
