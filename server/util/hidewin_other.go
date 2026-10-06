//go:build !windows

package util

import "os/exec"

// HideWindow 在非 Windows 系统上什么也不做
func HideWindow(cmd *exec.Cmd) {}
