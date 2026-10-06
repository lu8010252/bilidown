# Bilidown 无头版（服务器 / 机顶盒部署）

基于上游 `iuroc/bilidown` v2.1.1 的改动：

- 去掉系统托盘和自动打开浏览器，纯服务运行，`CGO_ENABLED=0` 即可编译，arm64 / amd64 通用。
- 任务列表新增 **↓ 下载到本机** 按钮：浏览器完整收到文件后，服务器才删除该文件，服务器只做临时中转。传输中断则文件保留，可重试。
- 磁盘保护：下载前检查剩余空间（`BILIDOWN_MIN_FREE_MB`，默认 1024），空间不足时拒绝任务；任务失败时自动清理 `.audio` / `.video` 临时文件。
- **自动取回 / 批量取回**：下载完成后自动把文件传到当前浏览器所在电脑（可在任务页顶部开关），也可勾选多个或“取回全部已完成”；同一文件不会被重复取回。
- **暂停 / 继续 / 取消**下载（HTTP Range 断点续传，网络中断也会从断点自动重试）。
- **Windows 本机版**：见下文“Windows 版”。
- 可选访问密码：`BILIDOWN_AUTH=用户名:密码`（HTTP Basic）。
- 安全修复：移除原 `/api/downloadVideo`（接受任意路径，可读取服务器上任意文件）和在服务器上调用 explorer/xdg-open 的 `/api/showFile`；同时去掉应用内的播放预览，文件只能按任务 ID 通过 `/api/fetchFile` 下载。

## 环境变量

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `BILIDOWN_PORT` | 8098 | 监听端口 |
| `BILIDOWN_HOST` | 空 | 监听地址，空为所有网卡 |
| `BILIDOWN_AUTH` | 空 | `用户名:密码`，空则不启用认证 |
| `BILIDOWN_DB` | `./data.db` | 数据库路径（容器内为 `/data/data.db`） |
| `BILIDOWN_MODE` | 自动 | `local` 本机版 / `server` 服务器版；未设置时 Windows 为 local，其余为 server |
| `BILIDOWN_MIN_FREE_MB` | 1024 | 保留的最小剩余空间，0 关闭 |

## 部署

```bash
git clone https://github.com/iuroc/bilidown && cd bilidown
git apply bilidown-headless.patch     # 或 patch -p1 < bilidown-headless.patch
# 修改 docker-compose.yml 里的密码和下载目录
docker compose up -d --build
```

浏览器访问 `http://盒子IP:8098`，首次使用先在页面里扫码登录 B 站。
需要 `ffmpeg`（镜像内已包含；不用 Docker 时请 `apt install ffmpeg`）。

## Windows 版

本机版直接在你的电脑上运行：只监听 127.0.0.1、自动打开浏览器、文件直接存在本机（无需“取回”）。Windows 上默认就是本机版。

不装 Go / Node，用 Docker Desktop 构建（产物在 `dist-windows`）：

```bash
docker build --target windows-out --output dist-windows .
```

或在装有 Node.js 和 Go 1.23+ 的 Windows 上运行 `windows\build-windows.bat`。
然后把 `ffmpeg.exe` 放进 `dist-windows`（或其 `bin` 子目录），双击 `bilidown.exe`。详见 `windows/README.txt`。

## 来源与许可

本项目基于 [iuroc/bilidown](https://github.com/iuroc/bilidown) v2.1.1 修改而来，沿用其 [Apache-2.0](./LICENSE) 许可证。
上面列出的内容是相对上游所做的修改；上游的原始说明、桌面版安装包和截图未随本仓库提供，请见上游项目主页。
