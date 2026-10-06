package main

import (
	"embed"
	"io/fs"
)

// 前端页面和托盘图标都打进 exe，所以只需要 bilidown.exe + ffmpeg.exe 两个文件。
// 构建前必须先构建前端（输出到 server/static），见 windows/build-windows.bat。

//go:embed all:static
var staticFS embed.FS

//go:embed tray.ico
var trayIcon []byte

func staticRoot() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
