//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

func showMessage(title, message string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")

	tPtr, _ := syscall.UTF16PtrFromString(title)
	mPtr, _ := syscall.UTF16PtrFromString(message)

	messageBox.Call(0, uintptr(unsafe.Pointer(mPtr)), uintptr(unsafe.Pointer(tPtr)), 0)
}

func getIconPath(name string) string {
	execPath, _ := os.Executable()
	baseDir := filepath.Dir(execPath)

	paths := []string{
		filepath.Join(baseDir, "icons", name+".ico"),
		filepath.Join(baseDir, "..", "icons", name+".ico"),
		filepath.Join(baseDir, "..", "..", "icons", name+".ico"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return "C:\\Windows\\System32\\imageres.dll,3"
}

var NameMap = map[string]string{
	"synced":       "synced",
	"commit":       "commit",
	"untracked":    "untracked",
	"pending_sync": "pending_sync",
	"no_remote":    "no_remote",
	"not_init":     "not_init",
}

func updateDirectoryIcon(path, status string) bool {
	iniPath := filepath.Join(path, "desktop.ini")
	fullIconPath := getIconPath(NameMap[status])
	absIconPath, _ := filepath.Abs(fullIconPath)

	newContent := fmt.Sprintf("\xef\xbb\xbf[.ShellClassInfo]\r\nIconResource=\"%s\",0\r\nIconIndex=0\r\n", absIconPath)

	oldBytes, err := os.ReadFile(iniPath)
	if err == nil && string(oldBytes) == newContent {
		return false
	}

	exec.Command("attrib", "-s", "-h", "-r", iniPath).Run()
	os.WriteFile(iniPath, []byte(newContent), 0644)
	exec.Command("attrib", "+s", "+h", iniPath).Run()

	exec.Command("attrib", "-r", "-s", path).Run()
	exec.Command("attrib", "+r", "+s", path).Run()

	return true
}

func refreshUI(paths []string) {
	if len(paths) == 0 {
		return
	}

	shell32 := syscall.NewLazyDLL("shell32.dll")
	shChangeNotify := shell32.NewProc("SHChangeNotify")

	for _, p := range paths {
		ptr, _ := syscall.UTF16PtrFromString(p)
		shChangeNotify.Call(0x00002000, 0x0005, uintptr(unsafe.Pointer(ptr)), 0)
	}
	shChangeNotify.Call(0x08000000, 0, 0, 0)
}
