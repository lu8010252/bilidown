package util

import (
	"os"
	"runtime"
)

// LocalMode 为 true 表示程序作为“本机版”运行（例如 Windows 上双击运行）：
// 默认只监听 127.0.0.1、自动打开浏览器，下载的文件直接留在本机，不需要“下载到本机”。
// 通过环境变量 BILIDOWN_MODE=local 开启（=server 强制关闭）；
// 未设置时：Windows 上默认为本机版，其他系统（Linux/Docker）为服务器模式。
func LocalMode() bool {
	switch os.Getenv("BILIDOWN_MODE") {
	case "local":
		return true
	case "server":
		return false
	}
	return runtime.GOOS == "windows"
}
