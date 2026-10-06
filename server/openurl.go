package main

import (
	"log"
	"os/exec"
	"runtime"
)

// openBrowser 用系统默认浏览器打开页面。
func openBrowser(url string) {
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
