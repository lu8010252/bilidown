package main

import (
	"log"
	"os/exec"
	"runtime"
	"time"
)

// openBrowser 用系统默认浏览器打开页面（仅本机模式使用）。
func openBrowser(url string) {
	time.Sleep(600 * time.Millisecond) // 等 HTTP 服务开始监听
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("无法自动打开浏览器，请手动访问 %s : %v", url, err)
	}
}
