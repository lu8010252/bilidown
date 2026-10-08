//go:build !headless

package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"bilidown/router"
	"bilidown/util"

	"github.com/getlantern/systray"
	_ "modernc.org/sqlite"
)

const (
	DEFAULT_PORT = 8098             // 默认 HTTP 端口，可用环境变量 BILIDOWN_PORT 覆盖
	VERSION      = "v2.1.1-windows" // Windows 本机版
)

var port = DEFAULT_PORT
var urlLocal string

func main() {
	// 以程序所在目录为工作目录，data.db、download、日志都放在这里（双击运行时 cwd 不一定是程序目录）
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	// 以 GUI 方式运行时没有控制台，日志写到 bilidown.log
	if f, err := os.OpenFile("bilidown.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		log.SetOutput(f)
	}
	if s := os.Getenv("BILIDOWN_PORT"); s != "" {
		p, err := strconv.Atoi(s)
		if err != nil || p <= 0 || p > 65535 {
			showError("Bilidown", "BILIDOWN_PORT 不是合法端口："+s)
			os.Exit(1)
		}
		port = p
	}
	urlLocal = fmt.Sprintf("http://127.0.0.1:%d", port)

	if _, err := util.GetFFmpegPath(); err != nil {
		showError("Bilidown", "找不到 ffmpeg.exe。\n请把 ffmpeg.exe 放到 bilidown.exe 所在目录（或其 bin 子目录）后重新运行。")
		os.Exit(1)
	}
	mustInitTables()

	// 先占住端口：占用失败多半是程序已经在运行，直接打开它的页面
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		log.Printf("端口 %d 被占用，可能已在运行: %v", port, err)
		openBrowser(urlLocal)
		return
	}
	go serve(ln)

	// 托盘（阻塞，直到点“退出”）
	systray.Run(onReady, nil)
}

// serve 在已监听的端口上提供前端页面（已打包进 exe）和后端接口
func serve(ln net.Listener) {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(staticRoot())))
	mux.Handle("/api/", http.StripPrefix("/api", router.API()))
	log.Printf("Bilidown %s listening on %s", VERSION, ln.Addr())
	if err := http.Serve(ln, mux); err != nil {
		log.Fatal("http.Serve:", err)
	}
}

// onReady 托盘就绪：设置图标、菜单，并打开浏览器
func onReady() {
	systray.SetIcon(trayIcon)
	systray.SetTooltip(fmt.Sprintf("Bilidown 视频解析器 %s (port:%d)", VERSION, port))

	open := systray.AddMenuItem(fmt.Sprintf("打开主界面 (port:%d)", port), "在浏览器中打开")
	folder := systray.AddMenuItem("打开下载目录", "在资源管理器中打开")
	about := systray.AddMenuItem("Github 项目主页", "")
	systray.AddSeparator()
	quit := systray.AddMenuItem("退出应用", "")
	go func() {
		for {
			select {
			case <-open.ClickedCh:
				openBrowser(urlLocal)
			case <-folder.ClickedCh:
				openDownloadFolder()
			case <-about.ClickedCh:
				openBrowser("https://github.com/lu8010252/bilidown")
			case <-quit.ClickedCh:
				log.Printf("Bilidown has exited.")
				systray.Quit()
				return
			}
		}
	}()

	go func() {
		time.Sleep(600 * time.Millisecond)
		openBrowser(urlLocal)
	}()
}

// openDownloadFolder 用资源管理器打开当前下载目录
func openDownloadFolder() {
	db := util.MustGetDB()
	defer db.Close()
	dir, err := util.GetCurrentFolder(db)
	if err != nil {
		log.Printf("openDownloadFolder: %v", err)
		return
	}
	name := "explorer"
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "linux":
		name = "xdg-open"
	}
	_ = exec.Command(name, dir).Start()
}

