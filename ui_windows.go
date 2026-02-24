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

func getIconPath(name string) string {
	if name == "commit" {
		return "C:\\Windows\\System32\\imageres.dll,3"
	}

	execPath, _ := os.Executable()
	baseDir := filepath.Dir(execPath)
	iconPath := filepath.Join(baseDir, "icons", name+".ico")
	if _, err := os.Stat(iconPath); err == nil {
		return iconPath
	}
	// Fallback padrão se o ícone não existir
	return "C:\\Windows\\System32\\imageres.dll,3"
}

func updateDirectoryIcon(path, status string) {
	iniPath := filepath.Join(path, "desktop.ini")
	fullIconPath := getIconPath(nameMap[status])
	
	// Convert to absolute path if not already
	absIconPath, _ := filepath.Abs(fullIconPath)

	// Remove attributes to edit
	exec.Command("attrib", "-s", "-h", "-r", iniPath).Run()

	// Content with UTF-8 BOM, IconResource and IconIndex=0 (standard for .ico)
	content := fmt.Sprintf("\xef\xbb\xbf[.ShellClassInfo]\r\nIconResource=%s,0\r\nIconIndex=0\r\n", absIconPath)
	os.WriteFile(iniPath, []byte(content), 0644)

	// Set attributes: Hidden and System for desktop.ini
	exec.Command("attrib", "+s", "+h", iniPath).Run()
	// Set attribute: ReadOnly for the FOLDER
	exec.Command("attrib", "+r", path).Run()
}

var nameMap = map[string]string{
	"synced":       "synced",
	"commit":       "commit",
	"untracked":    "untracked",
	"pending_sync": "pending_sync",
	"no_remote":    "no_remote",
	"not_init":     "not_init",
}

func refreshUI(paths []string) {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	shChangeNotify := shell32.NewProc("SHChangeNotify")
	
	// Global refresh
	shChangeNotify.Call(0x08000000, 0, 0, 0)
	
	// Specific folder refresh
	for _, p := range paths {
		ptr, _ := syscall.UTF16PtrFromString(p)
		// SHCNE_UPDATEITEM = 0x00002000, SHCNF_PATHW = 0x0005
		shChangeNotify.Call(0x00002000, 0x0005, uintptr(unsafe.Pointer(ptr)), 0)
	}
}
