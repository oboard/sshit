# 快速开始

`sshit` 在一个 TCP 端口上同时提供 SSH 服务和 Web 工作区。SSH 连接获得独立的 Shell；浏览器连接进入所有 Web 用户共享的工作区。

## 安装

Linux x64、macOS arm64 和 macOS x64 可使用安装脚本：

```bash
curl -fsSL https://ghfast.top/https://raw.githubusercontent.com/oboard/sshit/main/install.sh | bash
```

默认安装目录为 `/usr/local/bin`。如需安装到用户目录：

```bash
curl -fsSL https://ghfast.top/https://raw.githubusercontent.com/oboard/sshit/main/install.sh \
  | INSTALL_DIR="$HOME/.local/bin" bash
```

其他平台请从 [GitHub Releases](https://github.com/oboard/sshit/releases) 下载构建产物。

## 启动服务

```bash
sshit
```

默认监听 `0.0.0.0:2222`。首次启动时会在 `~/.ssh/sshit_host_ed25519_key` 创建 SSH 主机密钥，后续启动会复用该密钥。

## 用 SSH 连接

```bash
ssh -p 2222 localhost
```

SSH 会启动服务进程的 `$SHELL`；若未设置则使用 `/bin/sh`。终端使用 `xterm-256color`，支持 true color，并自动同步 SSH 客户端的窗口尺寸。

## 在浏览器中连接

在浏览器地址栏打开 `http://localhost:2222`。页面会自动建立 WebSocket 连接并显示交互式终端工作区。

> Web 工作区中的 Shell 会在已连接的浏览器用户之间共享；每个 SSH 登录始终拥有自己的 PTY Shell。

## 更改端口

```bash
# 长参数
sshit --port 2022

# 短参数
sshit -p 2022

# 连接到指定端口
ssh -p 2022 user@server.example
```

下一步可阅读[协作工作区](/guide/collaboration)，了解共享终端、Markdown 和画布的使用方式。
