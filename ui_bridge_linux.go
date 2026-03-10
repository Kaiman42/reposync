//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func getSysProcAttr() *syscall.SysProcAttr {
	return nil
}

var IconsLinux = map[string]string{
	"not_init":     "folder-black",
	"commit":       "folder-yellow",
	"untracked":    "folder-red",
	"no_remote":    "folder-orange",
	"synced":       "folder-green",
	"pending_sync": "folder-violet",
}

func updateDirectoryIcon(path, status string) bool {
	dotDir := filepath.Join(path, ".directory")
	icon := IconsLinux[status]
	if icon == "" {
		icon = "folder"
	}

	content := fmt.Sprintf("[Desktop Entry]\nIcon=%s\n", icon)

	old, _ := os.ReadFile(dotDir)
	if string(old) == content {
		return false
	}

	os.WriteFile(dotDir, []byte(content), 0644)
	exec.Command("touch", path).Run()
	return true
}

func refreshUI(_ []string) {
	for _, bin := range []string{"qdbus", "qdbus6"} {
		if path, err := exec.LookPath(bin); err == nil {
			exec.Command(path, "org.kde.dolphin", "/dolphin/Dolphin_1", "refresh").Run()
		}
	}
}

func showMessage(title, message string) {
	if path, err := exec.LookPath("zenity"); err == nil {
		exec.Command(path, "--error", "--title="+title, "--text="+message).Run()
	} else {
		fmt.Printf("[%s] %s\n", title, message)
	}
}
