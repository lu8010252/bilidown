//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// showError 弹出一个错误对话框（程序以 GUI 方式运行，没有控制台可看）
func showError(title, msg string) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(msg)
	proc := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	proc.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x10) // MB_ICONERROR
}
