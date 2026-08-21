# 从源码构建

## 前置要求

- Go 1.22 或更高版本；
- Node.js；
- pnpm。

## 构建应用

应用的 Svelte 前端会构建到 `internal/web/dist/`，随后通过 Go `embed` 打包进最终二进制。该生成目录不提交到仓库，因此必须先构建前端。

```bash
# 1. 安装并构建前端
cd web
pnpm install --frozen-lockfile
pnpm run build
cd ..

# 2. 构建或运行 Go 服务
go build ./...
# 或
 go run . --port 2222
```

## 构建文档站点

VitePress 的依赖和命令位于仓库根目录的 `package.json`，文档源文件和输出仍位于 `docs/`。输出为 `docs/.vitepress/dist/`，不会影响嵌入式应用前端：

```bash
pnpm install --frozen-lockfile
pnpm run docs:build
```

本地预览与开发：

```bash
pnpm run docs:dev
pnpm run docs:preview
```

`docs:dev` 默认启动 VitePress 开发服务器；可附加 Vite 参数来修改端口或监听地址。

## 发行产物

GitHub Actions 会构建 Linux x64、macOS arm64/x64 和 Windows x64 二进制。推送 `v*` 标签会创建 GitHub Release 并上传相应发行文件。
