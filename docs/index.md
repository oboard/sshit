---
layout: home

hero:
  name: sshit
  text: SSH 与共享 Web 终端，运行在同一个端口
  tagline: 用一个轻量级 Go 二进制提供原生 SSH、浏览器终端和实时协作工作区。
  actions:
    - theme: brand
      text: 开始使用
      link: /guide/getting-started
    - theme: alt
      text: GitHub
      link: https://github.com/oboard/sshit

features:
  - icon: ⌨️
    title: 保留 SSH 工作流
    details: 任意 SSH 客户端均可连接，每次 SSH 登录获得一个独立的 PTY Shell。
  - icon: 🌐
    title: 浏览器即终端
    details: 打开同一地址即可使用完整终端；输入、输出和尺寸经 WebSocket 同步。
  - icon: 🤝
    title: 共享协作空间
    details: 多名浏览器用户共享终端、Markdown 编辑器、画布与实时光标。
  - icon: 📦
    title: 单文件部署
    details: 前端静态资源嵌入 Go 二进制；不需要单独部署 Node.js 服务或反向代理。
---

## 一个端口，两种连接方式

```text
                    ┌─────────────┐
ssh -p 2222 host ──►│  SSH / PTY  │──► $SHELL
                    │             │
Browser ── HTTP/WS ►│    sshit    │──► shared Web PTYs
                    └─────────────┘
                         :2222
```

`sshit` 通过连接的前缀区分 SSH 与 HTTP 请求。命令行用户继续使用熟悉的 SSH，浏览器用户则进入共享工作区，无需占用额外端口。
