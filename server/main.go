package main

import (
	"database/sql"
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
				openBrowser("https://github.com/lu8010252/bilidown/tree/windows")
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

// mustInitTables 初始化数据表
func mustInitTables() {
	db := util.MustGetDB()
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS "field" (
		"name" TEXT PRIMARY KEY NOT NULL,
		"value" TEXT
	)`); err != nil {
		log.Fatalln("create table field:", err)
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS "log" (
		"id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,
		"content" TEXT NOT NULL,
		"create_at" text NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		log.Fatalln("create table log:", err)
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS "task" (
		"id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,
		"bvid" text NOT NULL,
		"cid" integer NOT NULL,
		"format" integer NOT NULL,
		"title" text NOT NULL,
		"owner" text NOT NULL,
		"cover" text NOT NULL,
		"status" text NOT NULL,
		"folder" text NOT NULL,
		"duration" integer NOT NULL,
		"download_type" text NOT NULL DEFAULT 'merge',
		"create_at" text NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		log.Fatalln("create table task:", err)
	}

	if _, err := util.GetCurrentFolder(db); err != nil {
		log.Fatalln("util.GetCurrentFolder:", err)
	}

	if err := initHistoryTask(db); err != nil {
		log.Fatalln("initHistoryTask:", err)
	}

	// 添加可能缺失的列（用于数据库迁移）
	if err := addMissingColumns(db); err != nil {
		log.Fatalln("addMissingColumns:", err)
	}
}

// addMissingColumns 添加可能缺失的列（用于数据库迁移）
func addMissingColumns(db *sql.DB) error {
	// 检查download_type列是否存在，如果不存在则添加
	// SQLite没有直接的方法检查列是否存在，我们尝试添加列并忽略错误
	// 使用事务确保操作原子性
	util.SqliteLock.Lock()
	_, _ = db.Exec(`ALTER TABLE "task" ADD COLUMN "download_type" TEXT DEFAULT 'merge'`)
	// 将现有记录中的NULL值更新为默认值'merge'
	_, _ = db.Exec(`UPDATE "task" SET "download_type" = 'merge' WHERE "download_type" IS NULL`)
	util.SqliteLock.Unlock()

	// 忽略错误，因为列可能已经存在
	// SQLite错误码为1表示列已存在
	return nil
}

// initHistoryTask 将上一次程序运行时未完成的任务进度全部变为 error
func initHistoryTask(db *sql.DB) error {
	util.SqliteLock.Lock()
	_, err := db.Exec(`UPDATE "task" SET "status" = 'error' WHERE "status" IN ('waiting', 'running')`)
	util.SqliteLock.Unlock()
	return err
}
