//go:build linux

package util

import "syscall"

// FreeBytes 返回目录所在分区对普通用户的可用字节数。
func FreeBytes(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
