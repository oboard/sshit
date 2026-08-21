# 命令行配置

```text
sshit [flags]
```

## 选项

| 选项 | 默认值 | 说明 |
| --- | --- | --- |
| `--port`, `-p` | `2222` | SSH 和 HTTP/WebSocket 共用的监听端口。 |
| `--password` | 空 | SSH 密码认证与 Web UI 登录共用的密码。空值表示不要求密码。 |
| `--persist` | `true` | 将 Web 工作区保存到 `~/.sshit/<port>/`，并在重启后恢复。 |
| `--persist-history` | `true` | 保存终端滚动历史并在恢复时回放。历史可能包含敏感信息。 |

## 示例

```bash
# 在 2022 端口运行
sshit --port 2022

# 使用短参数
sshit -p 2022

# 同时启用密码
sshit --port 2022 --password 'change-me'

# 保留布局和协作文档，但不写终端历史
sshit --persist-history=false

# 完全不保存状态
sshit --persist=false
```

有关访问密码的部署边界，请阅读[安全建议](/guide/security)。
