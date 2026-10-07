# syntax=docker/dockerfile:1
# 无托盘 / 无头运行版 Bilidown。多架构：amd64 / arm64 / armv7（Armbian 机顶盒等）。
# 前端产物与架构无关，固定在构建机原生架构上构建；Go 交叉编译，避免 QEMU 模拟下跑 pnpm/go 很慢。

# ---------- 1. 构建前端 ----------
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
ARG NPM_REGISTRY=https://registry.npmmirror.com
WORKDIR /src/client
RUN corepack enable
COPY client/package.json client/pnpm-lock.yaml ./
RUN pnpm config set registry ${NPM_REGISTRY} && pnpm install --frozen-lockfile
COPY client/ ./
# vite 的输出目录是 ../server/static
RUN pnpm build

# ---------- 2. 构建后端（纯 Go，无需 CGO） ----------
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS server
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY} CGO_ENABLED=0
WORKDIR /src/server
COPY server/ ./
COPY --from=web /src/server/static ./static
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go mod tidy && GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -trimpath -ldflags="-s -w" -o /out/bilidown .

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
