# syntax=docker/dockerfile:1
# 无托盘 / 无头运行版 Bilidown。在目标机器上直接 `docker compose build`，架构自动匹配（amd64 / arm64 均可）。

# ---------- 1. 构建前端 ----------
FROM node:22-alpine AS web
ARG NPM_REGISTRY=https://registry.npmmirror.com
WORKDIR /src/client
RUN corepack enable
COPY client/package.json client/pnpm-lock.yaml ./
RUN pnpm config set registry ${NPM_REGISTRY} && pnpm install --frozen-lockfile
COPY client/ ./
# vite 的输出目录是 ../server/static
RUN pnpm build

# ---------- 2. 构建后端（纯 Go，无需 CGO） ----------
FROM golang:1.23-alpine AS server
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY} CGO_ENABLED=0
WORKDIR /src/server
COPY server/ ./
COPY --from=web /src/server/static ./static
RUN go mod tidy && go build -trimpath -ldflags="-s -w" -o /out/bilidown .

# ---------- 2b. Windows 版（可选）：docker build --target windows-out --output dist-windows . ----------
FROM server AS server-win
RUN GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/bilidown.exe .

FROM scratch AS windows-out
COPY --from=server-win /out/bilidown.exe /bilidown.exe
COPY --from=server-win /src/server/static /static
COPY windows/README.txt /README.txt

# ---------- 3. 运行镜像 ----------
FROM alpine:3.20
RUN apk add --no-cache ffmpeg ca-certificates tzdata
WORKDIR /app
COPY --from=server /out/bilidown /app/bilidown
COPY --from=server /src/server/static /app/static
ENV BILIDOWN_DB=/data/data.db
VOLUME ["/data", "/app/download"]
EXPOSE 8098
ENTRYPOINT ["/app/bilidown"]
