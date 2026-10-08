//go:build embedffmpeg && !headless

package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"bilidown/util"
)

// 带 embedffmpeg 标签构建时（GitHub Actions 的 Windows 构建），把精简版 ffmpeg.exe 打进 bilidown.exe。
// 构建前需要先把它放到 server/ffmpeg/ffmpeg.exe（目录里的 .keep 只是占位，让没放 ffmpeg 的 go mod tidy 也不报错）。

//go:embed all:ffmpeg
var ffmpegFS embed.FS

// extractEmbeddedFFmpeg 把内置的 ffmpeg.exe 释放到磁盘（程序要启动子进程，只能从文件运行）。
// 优先放在 exe 旁边的 bin 目录，不可写时（如装在 Program Files）退到系统临时目录。
func extractEmbeddedFFmpeg() {
	data, err := ffmpegFS.ReadFile("ffmpeg/ffmpeg.exe")
	if err != nil {
		log.Printf("读取内置 ffmpeg 失败: %v", err)
		return
	}
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "bin"))
	}
	dirs = append(dirs, filepath.Join(os.TempDir(), "bilidown-ffmpeg"))
	for _, dir := range dirs {
		path := filepath.Join(dir, "ffmpeg.exe")
		if info, err := os.Stat(path); err == nil && info.Size() == int64(len(data)) {
			util.EmbeddedFFmpegPath = path
			return
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		// 先写临时文件再改名，避免中途失败留下半个文件
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o755); err != nil {
			continue
		}
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			continue
		}
		util.EmbeddedFFmpegPath = path
		return
	}
	log.Printf("无法释放内置 ffmpeg")
}
