# syntax=docker/dockerfile:1

# ---------- Frontend build ----------
FROM node:22-alpine AS web
WORKDIR /app/web
RUN npm install --global pnpm
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
# vite outputs to ../internal/web/dist (see web/vite.config.ts)
RUN pnpm run build

# ---------- Go build ----------
FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Embedded assets come from the frontend stage, not the build context.
COPY --from=web /app/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/sshit .

# ---------- Runtime ----------
FROM alpine:3.21
# bash: default login shell for SSH/Web PTYs; ca-certificates/tzdata for TLS and logs.
RUN apk add --no-cache bash ca-certificates tzdata
COPY --from=build /out/sshit /usr/local/bin/sshit
ENV SHELL=/bin/bash
EXPOSE 2222
ENTRYPOINT ["sshit"]
# Override port/address e.g.: docker run ... sshit -p 2222
CMD ["-p", "2222"]
