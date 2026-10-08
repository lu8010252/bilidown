# Bilidown 改版

基于 [iuroc/bilidown](https://github.com/iuroc/bilidown) v2.1.1（Apache-2.0）修改。**同一份代码、两种运行方式**，由构建方式决定，页面会自动适配：

| 运行方式 | 适用 | 说明 |
| --- | --- | --- |
| **Windows 本机版**（默认构建） | Windows 电脑本机使用 | 和上游一样是独立程序：双击 `bilidown.exe`，系统托盘图标，自动打开浏览器；页面和图标已打包进 exe，只需再放一个 `ffmpeg.exe`。只监听 127.0.0.1，文件直接存本机，任务列表有“打开位置” |
| **Docker 无头版**（`-tags headless`） | 服务器 / 机顶盒（arm64 / amd64 / armv7） | 无托盘、无头运行，浏览器访问；下载完成后可自动 / 批量“取回”到你当前使用的电脑，传完服务器自动删除文件，适合磁盘很小的设备做临时中转；可选访问密码 |

## 两种方式共有的改动

- 下载可 **暂停 / 继续 / 取消**（HTTP Range 断点续传，网络中断自动从断点重试）
- 外观设置：浅色 / 深色 / 日出日落 / 跟随系统，6 种背景配色，玻璃效果
- 命名方式：默认用解析时显示的名字，也可用完整名或自定义模板
- 解析时显示每个视频 / 音频的预估大小，任务列表显示真实大小
- 磁盘保护：剩余空间不足时拒绝任务，失败任务自动清理临时文件
- 安全修复：移除上游会读取任意路径的 `/api/downloadVideo` 和应用内播放预览

## 构建与发布

- **Windows**：推送到 `main` 后 GitHub Actions 自动构建 `bilidown.exe`（附带上游精简版 ffmpeg）并更新 `windows-latest` 预发布；把 `windows/VERSION` 里的版本号改掉并推送，会自动发布同名正式 Release。本机构建见 `windows/build-windows.bat` 或 `Dockerfile.windows`。
- **Docker**：推送到 `main` 后自动构建并推送 `ghcr.io/lu8010252/bilidown:latest`（`Dockerfile` 里用 `go build -tags headless`）。
- 前端会向后端的 `/api/mode` 询问当前是哪种方式：无头版显示“取回”相关按钮，本机版显示“打开位置”。后端同理：`/api/fetchFile` 只在无头版注册，`/api/showFile` 只在本机版注册（后者会在运行程序的电脑上调用 explorer，不能暴露在服务器上）。

## Windows 本机版使用

下载 [Releases](../../releases) 里的 zip，解压后双击 `bilidown.exe`（已含 ffmpeg），会自动打开浏览器；托盘图标右键可“打开主界面 / 打开下载目录 / 退出”。数据库、下载目录、日志（`bilidown.log`）都在程序所在目录。端口可用环境变量 `BILIDOWN_PORT` 修改。

## Docker 无头版使用

基于上游的改动：

- 去掉系统托盘和自动打开浏览器，纯服务运行，`CGO_ENABLED=0` 即可编译。
- 任务列表新增 **↓ 下载到本机** 按钮：浏览器完整收到文件后，服务器才删除该文件，服务器只做临时中转。传输中断则文件保留，可重试。
- **自动取回 / 批量取回**：下载完成后自动把文件传到当前浏览器所在电脑（任务页顶部开关），也可勾选多个或“取回全部已完成”。
- 可选访问密码：`BILIDOWN_AUTH=用户名:密码`（HTTP Basic）。

### 环境变量

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `BILIDOWN_PORT` | 8098 | 监听端口 |
| `BILIDOWN_HOST` | 空 | 监听地址，空为所有网卡 |
| `BILIDOWN_AUTH` | 空 | `用户名:密码`，空则不启用认证 |
| `BILIDOWN_DB` | `./data.db` | 数据库路径（容器内为 `/data/data.db`） |
| `BILIDOWN_MIN_FREE_MB` | 1024 | 保留的最小剩余空间，0 关闭 |

### 部署（docker compose）

前提：机器上已装 Docker 和 docker compose 插件（1Panel 自带）。

#### 方式一：直接用现成镜像（推荐，不用下载源码）

每次推送到 main，GitHub Actions 会自动构建 `ghcr.io/lu8010252/bilidown:latest`（amd64 / arm64 通用）。把仓库里的 `docker-compose.yml` 内容贴到服务器（1Panel 的“编排”里新建即可），改掉访问密码后启动：

```bash
docker compose up -d
# 更新到新版本：
docker compose pull && docker compose up -d
```

#### 方式二：自己从源码构建

第一次构建需要联网，Dockerfile 默认使用国内镜像（npmmirror、goproxy.cn）。把 `docker-compose.yml` 里的 `image:` 一行注释掉、启用 `build: .` 后：

```bash
# 1. 下载本仓库
git clone https://github.com/lu8010252/bilidown.git
cd bilidown

# 2. 修改 docker-compose.yml（至少改掉访问密码）：
#      BILIDOWN_AUTH=admin:change-me   ->  BILIDOWN_AUTH=你的用户名:你的密码
#    下载目录默认是 ./download，想放到别的大容量磁盘就改 volumes 里的路径

# 3. 构建并启动（首次需要几分钟；机顶盒内存小，构建时可能偏慢）
docker compose up -d --build
```

没有 git 的话，把仓库下载成 zip 解压后进入目录，同样执行第 2、3 步即可。

启动后用浏览器打开 `http://机顶盒IP:8098`，输入上面设置的用户名密码，再在页面里扫码登录 B 站。

#### 怎么用

1. 「视频解析」页粘贴 B 站链接 → 选清晰度 / 命名方式 → 下载。
2. 「任务列表」页可以暂停 / 继续 / 取消下载；顶部开关“下载完成后自动取回”开启时，下载完的文件会自动传到你当前使用的电脑（浏览器下载文件夹），传完服务器上的文件自动删除。也可以勾选多个、或点“取回全部已完成”批量取回。
3. 第一次自动取回时，Chrome 可能提示“允许此网站下载多个文件”，点允许即可；否则会显示“浏览器没有开始下载”，允许后点 ↓ 重试。
4. 取回中途断了，服务器上的文件会保留，可以重新取回。

#### 常用命令

```bash
docker compose logs -f              # 看日志
docker compose restart              # 重启
docker compose down                 # 停止并删除容器（data、download 目录里的文件不会删）
git pull && docker compose up -d --build   # 更新到新版本
```

#### 注意

- `./data` 保存登录状态和任务记录，`./download` 是下载中转目录。服务器磁盘小，所以建议保持自动取回开启，下载一个传一个。
- 剩余空间低于 `BILIDOWN_MIN_FREE_MB`（默认 1024MB）时会拒绝新任务。
- 二维码加载失败多半是容器 DNS 问题，`docker-compose.yml` 里已指定公共 DNS，可按需修改。
- 访问密码只是 HTTP Basic，没有 HTTPS，不要直接暴露到公网；公网使用请放在反向代理（带 HTTPS）后面。
- 页面右上角可切换主题，设置中心有外观设置（背景配色、玻璃效果等）。

## 来源与许可

本项目基于 [iuroc/bilidown](https://github.com/iuroc/bilidown) v2.1.1 修改而来，沿用其 [Apache-2.0](./LICENSE) 许可证。
上面列出的内容是相对上游所做的修改；上游的原始说明、桌面版安装包和截图未随本仓库提供，请见上游项目主页。

## 说明

- 代码在开发环境里做过前端类型检查和界面模拟测试；完整的真机构建依赖在开发环境无法下载，需要在目标机器 / GitHub Actions 上执行。
- 许可证沿用上游 Apache-2.0，见 `LICENSE`。
