package util

import (
	"fmt"
	"os"
	"strconv"
)

// minFreeBytes 返回磁盘保护的保留空间，默认 1024MB，可用环境变量 BILIDOWN_MIN_FREE_MB 调整，设为 0 关闭保护。
func minFreeBytes() uint64 {
	mb := uint64(1024)
	if s := os.Getenv("BILIDOWN_MIN_FREE_MB"); s != "" {
		if v, err := strconv.ParseUint(s, 10, 64); err == nil {
			mb = v
		}
	}
	return mb << 20
}

// CheckDisk 检查 dir 所在分区在写入 need 字节后，是否仍能保留 BILIDOWN_MIN_FREE_MB 的空间。
// 平台不支持查询空间时视为通过。
func CheckDisk(dir string, need uint64) error {
	min := minFreeBytes()
	if min == 0 {
		return nil
	}
	free, err := FreeBytes(dir)
	if err != nil {
		return nil
	}
	if free < need+min {
		return fmt.Errorf("磁盘剩余空间不足：剩余 %d MB，本次需要约 %d MB，另需保留 %d MB", free>>20, need>>20, min>>20)
	}
	return nil
}
