# sshit

A tiny SSH server that gives PTY-backed sessions an interactive shell using the server process' `SHELL` environment variable, falling back to `/bin/sh`.

The same TCP port serves both SSH and a browser terminal UI:

```text
TCP :2222
    ├── SSH  → PTY / Shell
    └── HTTP → embedded Web UI / WebSocket / PTY
```

## Build frontend first

`internal/web/dist/` is generated and intentionally not committed. Build the frontend before `go run` / `go build`:

```bash
cd web
npm install
npm run build
cd ..
```

## Run

```bash
go run .
```

The default port is `2222`. Use `-p` or `--port` to choose another port:

```bash
go run . --port 2022
# or
go run . -p 2022
```

By default, SSH and Web UI access do not require a password. Use `--password` to require the same password for both SSH and Web UI:

```bash
go run . --password test123
```

The server uses a persistent host key at `~/.ssh/sshit_host_ed25519_key`. If the key already exists it is reused; otherwise it is generated automatically with `0600` permissions.

## SSH client

```bash
ssh -p 2222 localhost
```

## Web UI

Open the same port in a browser:

```text
http://localhost:2222
```

The web UI is extracted from sshx visual assets/theme and embedded into the Go binary. It connects back to `/ws` on the same host and starts a PTY shell over WebSocket.

## Build

```bash
cd web
npm install
npm run build
cd ..
go build ./...
```
