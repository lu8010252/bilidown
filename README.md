# Bilidown Windows 版

基于上游 [iuroc/bilidown](https://github.com/iuroc/bilidown) v2.1.1 的改动，面向 Windows 本机使用（服务器/机顶盒用的 Docker 版在另一个包里，互不相干）：

- 和上游一样是独立程序：双击 `bilidown.exe` 即用，系统托盘图标（打开主界面 / 打开下载目录 / 退出），自动打开浏览器；页面和图标已打包进 exe，只需 `bilidown.exe` + `ffmpeg.exe` 两个文件。默认只监听 127.0.0.1。
- 下载可 **暂停 / 继续 / 取消**（HTTP Range 断点续传，网络中断自动从断点重试）。
- 外观设置：浅色/深色/日出日落/跟随系统、6 种背景配色、玻璃效果。
- 命名方式：默认用解析时显示的名字，也可用完整名或自定义模板；解析时显示每个视频/音频的预估大小，任务列表显示真实大小。
- 磁盘保护：空间不足时拒绝任务；失败任务自动清理临时文件。
- 安全修复：移除原 `/api/downloadVideo`（可读取任意路径）和应用内播放预览。

## 构建

任选一种，产物在 `dist-windows`：

```bash
# 有 Docker（Docker Desktop）：不用装 Go / Node
docker build -f Dockerfile.windows --output dist-windows .
```

或在装有 Node.js 和 Go 1.23+ 的 Windows 上运行 `windows\build-windows.bat`。

然后把 `ffmpeg.exe` 放到 `bilidown.exe` 旁边（或其 `bin` 子目录），双击 `bilidown.exe`。详见 `windows/README.txt`。

## 环境变量（可选）

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `BILIDOWN_PORT` | 8098 | 监听端口 |
| `BILIDOWN_DB` | `./data.db` | 数据库路径 |
| `BILIDOWN_MIN_FREE_MB` | 1024 | 保留的最小剩余空间，0 关闭 |

## 来源与许可

本项目基于 [iuroc/bilidown](https://github.com/iuroc/bilidown) v2.1.1 修改而来，沿用其 [Apache-2.0](./LICENSE) 许可证。
