package main

import (
	"crypto/subtle"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"bilidown/router"
	"bilidown/util"

	_ "modernc.org/sqlite"
)

const (
	DEFAULT_PORT = 8098              // 默认 HTTP 端口，可用环境变量 BILIDOWN_PORT 覆盖
	VERSION      = "v2.1.1-headless" // 无托盘、无头运行的分支版本
)

func main() {
	checkFFmpeg()
	// 初始化数据表
	mustInitTables()
	// 配置并启动 HTTP 服务器（阻塞）
	mustRunServer()
}

// checkFFmpeg 检测 ffmpeg 的安装情况，如果未安装则打印提示信息并退出。
func checkFFmpeg() {
	if _, err := util.GetFFmpegPath(); err != nil {
		log.Fatalln("🚨 FFmpeg is missing. Install it (e.g. apt install ffmpeg) or place it in ./bin, then restart the application.")
	}
}

// listenAddr 返回监听地址。BILIDOWN_HOST 默认为空（监听所有网卡），BILIDOWN_PORT 默认 8098。
func listenAddr() string {
	port := DEFAULT_PORT
	if s := os.Getenv("BILIDOWN_PORT"); s != "" {
		p, err := strconv.Atoi(s)
		if err != nil || p <= 0 || p > 65535 {
			log.Fatalln("BILIDOWN_PORT 不是合法端口:", s)
		}
		port = p
	}
	return fmt.Sprintf("%s:%d", os.Getenv("BILIDOWN_HOST"), port)
}

// withAuth 在设置了环境变量 BILIDOWN_AUTH=用户名:密码 时启用 HTTP Basic 认证，未设置则不做任何限制。
func withAuth(next http.Handler) http.Handler {
	cred := os.Getenv("BILIDOWN_AUTH")
	if cred == "" {
		return next
	}
	user, pass, ok := strings.Cut(cred, ":")
	if !ok || user == "" {
		log.Fatalln("BILIDOWN_AUTH 格式应为 用户名:密码")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, has := r.BasicAuth()
		okUser := subtle.ConstantTimeCompare([]byte(u), []byte(user)) == 1
		okPass := subtle.ConstantTimeCompare([]byte(p), []byte(pass)) == 1
		if !has || !okUser || !okPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Bilidown"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// 配置和启动 HTTP 服务器
func mustRunServer() {
	mux := http.NewServeMux()
	// 前端打包文件
	mux.Handle("/", http.FileServer(http.Dir("static")))
	// 后端接口服务
	mux.Handle("/api/", http.StripPrefix("/api", router.API()))

	addr := listenAddr()
	log.Printf("Bilidown %s listening on %s", VERSION, addr)
	if err := http.ListenAndServe(addr, withAuth(mux)); err != nil {
		log.Fatal("http.ListenAndServe:", err)
	}
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
