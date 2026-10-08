//go:build !linux

package util

import "errors"

// FreeBytes 在非 Linux 平台暂不支持，调用方应忽略该错误（即不做磁盘保护）。
func FreeBytes(dir string) (uint64, error) {
	return 0, errors.New("free space check not supported on this platform")
}
