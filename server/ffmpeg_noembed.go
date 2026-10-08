//go:build !embedffmpeg && !headless

package main

// 没有内置 ffmpeg 的构建（自己本地编译）：什么也不做，由 ffmpeg.exe 放在 exe 旁边提供。
func extractEmbeddedFFmpeg() {}
