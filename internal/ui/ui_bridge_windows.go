//go:build windows

package ui

import (
	"syscall"
	"unsafe"
)

func ShowMessage(title, message string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")

	tPtr, _ := syscall.UTF16PtrFromString(title)
	mPtr, _ := syscall.UTF16PtrFromString(message)

	messageBox.Call(0, uintptr(unsafe.Pointer(mPtr)), uintptr(unsafe.Pointer(tPtr)), 0)
}

func UpdateDirectoryIcon(path, status string) bool {
	return false
}

func RefreshUI(paths []string) {
	// Logic removed to prevent explorer icon refreshes
}
