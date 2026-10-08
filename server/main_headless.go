//go:build headless

package main

import (
	"crypto/subtle"
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
	VERSION      = "v2.1.1-headless" // 无托盘、无头运行（Docker / 服务器）
)

func main() {
	router.Headless = true // 无头模式：任务页提供“取回到本机”，不提供“打开位置”
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

