# 架构与协议

`sshit` 使用单个 TCP 监听器。连接到达后，服务读取开头字节：以 `SSH-` 开头的连接交给 SSH 服务，其余请求按 HTTP 处理并返回嵌入式 Web UI。

| 连接 | 识别方式或路径 | 用途 |
| --- | --- | --- |
| SSH | 以 `SSH-` 开头 | 启动独立 PTY 与 Shell。 |
| HTTP | 其他请求 | 返回嵌入到 Go 二进制中的 Web 静态资源。 |
| WebSocket | `/ws` | 传递终端事件、PTY 输入输出与工作区状态。 |
| WebSocket | `/collab` | 同步 Markdown、画布和协作状态。 |

```text
                    ┌─────────────────────────┐
SSH client ─────────► SSH server ─► PTY / shell
                    │          sshit          │
Browser ─ HTTP/WS ──► embedded Web UI / hub ─► shared Web PTYs
                    └─────────────────────────┘
                              one port
```

这种设计让传统命令行操作和浏览器协作自然共存：无须额外端口，无须运行独立的前端服务。
