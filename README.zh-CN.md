# sshit

[English](README.md)

**把 SSH 终端带到浏览器，也保留命令行的原生体验。**

`sshit` 是一个轻量级的 SSH 服务：一个端口同时提供标准 SSH 与 Web UI。通过 `ssh` 登录时，你得到熟悉的 PTY Shell；在浏览器中打开同一地址时，则进入一个可多人共享的终端工作区。两种入口由同一个 Go 二进制提供，Web 前端已嵌入二进制，无需额外部署 Web 服务。

![sshit Web UI 截图](screenshots/1.jpg)

```text
                    ┌─────────────┐
ssh -p 2222 host ──►│  SSH / PTY  │──► $SHELL
                    │             │
Browser ── HTTP/WS ►│    sshit    │──► shared Web PTYs
                    └─────────────┘
                         :2222
```

## 为什么是 SSH + Web UI？

SSH 是可靠、通用且脚本友好的远程终端协议；Web UI 则降低了协作和临时访问的门槛。`sshit` 将两者放在同一服务中：

- **命令行用户继续使用 SSH**：使用任意 SSH 客户端连接，获得真实的 PTY 和本机 Shell 体验。
- **浏览器用户无需安装客户端**：打开 URL 即可使用完整终端，输入、输出和窗口尺寸经 WebSocket 实时同步。
- **共享工作区便于协作**：Web 用户看到同一组终端窗口，可创建、移动、缩放或关闭终端；终端输出会保留并同步给后来加入的用户。
- **不仅是终端**：Web UI 支持多人光标、共享 Markdown 编辑器和画布，让命令、说明与讨论可以在一个页面中完成。
- **一个端口、一个二进制**：服务根据连接前缀区分 `SSH-` 与 HTTP；前端静态资源由 Go `embed` 打包，不需要反向代理或单独的 Node.js 运行时。

> Web 工作区中的 Shell 由服务端创建并在已连接的浏览器用户之间共享；SSH 登录则各自创建独立的 PTY Shell。

## 快速开始

### 安装

Linux x64、macOS arm64 和 macOS x64 可通过安装脚本获取最新发行版：

```bash
curl -fsSL https://raw.githubusercontent.com/oboard/sshit/main/install.sh | bash
```

默认安装到 `/usr/local/bin`。若希望安装到用户目录：

```bash
curl -fsSL https://raw.githubusercontent.com/oboard/sshit/main/install.sh \
  | INSTALL_DIR="$HOME/.local/bin" bash
```

其他已构建平台的二进制请前往 [GitHub Releases](https://github.com/oboard/sshit/releases) 下载。

### 升级

安装后，`sshit` 可以自行更新到最新发行版，无需重新运行安装脚本。它会下载当前平台的二进制并原子替换正在运行的可执行文件：

```bash
# 更新到最新发行版
sshit upgrade

# 仅查看是否有更新，不改动任何文件
sshit upgrade --check
```

在 macOS/Linux 上，新二进制会就地替换正在运行的旧版本；在 Windows 上会先把正在运行的 `.exe` 改名避让，然后替换，因此请在更新后重启 `sshit`（或重开终端）以使用新版本。

### 启动服务

```bash
sshit
```

默认监听 `0.0.0.0:2222`。首次启动会在 `~/.ssh/sshit_host_ed25519_key` 自动生成 SSH 主机密钥，之后会持续复用该密钥。

### 从两种入口连接

**SSH：**

```bash
ssh -p 2222 localhost
```

**浏览器：**

打开 <http://localhost:2222>。页面会自动建立 WebSocket 连接，并创建可交互的终端。

## 使用方式

### 保持 SSH 工作流

SSH 会使用服务进程的 `$SHELL` 启动交互式 Shell；未设置时回退到 `/bin/sh`。终端环境默认使用 `xterm-256color`、true color，并正确同步 SSH 客户端的窗口大小。

```bash
# 指定服务端口
sshit --port 2022

# 监听指定地址（短参数：-a）
sshit --address 127.0.0.1 --port 2022

# 端口使用短参数
sshit -p 2022

# 连接到指定端口
ssh -p 2022 user@server.example
```

### 会话持久化（重启还原）

默认情况下，sshit 会把 Web 工作区持久化到 `~/.sshit/<port>/`，并在服务（或电脑）重启后还原。还原的窗口会保持原有的位置、尺寸和叠放层级。

```bash
# 默认：布局 + 终端历史回放 + AI Agent 恢复全部开启
sshit

# 关闭终端历史落盘（布局和 AI Agent 恢复仍然生效）
sshit --persist-history=false

# 完全关闭持久化
sshit --persist=false
```

重启后会还原的内容：

- **窗口布局**——每个终端和编辑器窗口的位置、尺寸与 z 序；
- **终端历史**——每个窗格回放之前的屏幕内容。普通 shell 进程本身不恢复，窗格会在保存的工作目录中以新 shell 重新打开；
- **编辑器与画布内容**——Markdown 文档和画布涂鸦保存在共享 CRDT 文档中，其更新日志会持久化，编辑器窗口恢复时内容一并还原；
- **AI Agent 会话**——运行了受支持 Agent（`claude`、`codex`）的终端会用其恢复命令重新拉起（例如 `claude --resume <id>`），让对话从中断处继续。

状态写入 `~/.sshit/<port>/session.json`（布局）、`~/.sshit/<port>/history/<id>.txt`（滚动历史）与 `~/.sshit/<port>/collab.json`（协作文档）。只要存在历史文件，恢复时一律回放——`--persist-history` 只控制是否继续写入新输出。

> **安全提示：** 终端历史可能包含密码、令牌和命令输出。请把 `~/.sshit/` 视同 shell 历史对待——文件以 `0700`/`0600` 权限写入。在共享机器上可用 `--persist-history=false` 关闭。

### 在 Web 工作区协作

在浏览器中，你可以：

1. 在画布上创建终端窗口；
2. 拖动、缩放、聚焦或关闭终端；
3. 与其他已连接用户实时查看相同的终端输出；
4. 使用共享 Markdown 窗口和画布记录命令、操作步骤或排障过程；
5. 通过用户列表和多人光标确认谁正在工作区中。

这使 `sshit` 适合远程演示、结对排障、教学，以及需要同时照顾终端用户与浏览器用户的临时运维场景。

## 访问控制

默认情况下，SSH 和 Web UI 均不要求密码，适合本地开发或受信任网络。对外暴露服务前，请启用密码，或在可信网络与额外访问控制之后使用：

```bash
sshit --password 'change-me'
```

启用后，SSH 密码认证与 Web UI 登录使用同一密码：

```bash
ssh -p 2222 user@server.example
# Password: change-me
```

> `--password` 是一个简单的共享密码机制。生产环境请结合防火墙、VPN、反向代理/TLS 或其他外围认证措施，并避免直接将未受保护的服务暴露到公网。

## 工作原理

连接到同一 TCP 端口后，`sshit` 读取连接开头的字节：

| 连接类型 | 识别方式 | 处理方式 |
| --- | --- | --- |
| SSH | 以 `SSH-` 开头 | 交给 SSH Server，创建独立 PTY 并运行 Shell |
| HTTP | 其他请求 | 返回嵌入式 Web UI |
| WebSocket | `/ws` | 传输 Web 终端事件、PTY 输入输出及工作区状态 |
| WebSocket | `/collab` | 同步 Markdown、画布和协作状态 |

因此，命令行与浏览器可以自然地共存：无需额外端口，也不必选择不同的服务。

## 从源码构建

要求：Go 1.22+、Node.js 与 pnpm。

前端构建产物位于 `internal/web/dist/`，该目录会嵌入 Go 二进制且不提交到仓库。因此，运行或构建 Go 程序前必须先构建前端：

```bash
# 1. 构建并生成嵌入式前端资源
cd web
pnpm install --frozen-lockfile
pnpm run build
cd ..

# 2. 编译或直接运行
go build ./...
# 或
go run . --port 2222
```

## 发布构建

GitHub Actions 会构建 Linux x64、macOS arm64/x64 与 Windows x64 发行产物；推送 `v*` 标签会创建 GitHub Release 并上传相应二进制。

## 技术栈

- **服务端**：Go、`gliderlabs/ssh`、`creack/pty`、Gorilla WebSocket
- **Web UI**：Svelte、xterm.js、Yjs、CodeMirror
- **分发方式**：Go `embed` 将构建后的静态资源合并进单一可执行文件

## License

本项目采用 [GNU Affero General Public License v3.0](LICENSE)（AGPL-3.0-only）开源。

若你修改 sshit 后通过网络向用户提供该修改版本，AGPL-3.0 要求你向这些用户提供该版本的对应源代码。详情请参阅[许可证第 13 节](LICENSE)。
