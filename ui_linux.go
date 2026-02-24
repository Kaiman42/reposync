//go:build !windows
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var iconsLinux = map[string]string{
	"not_init":     "folder-black",
	"commit":       "folder-yellow",
	"untracked":    "folder-red",
	"no_remote":    "folder-orange",
	"synced":       "folder-green",
	"pending_sync": "folder-violet",
}

func updateDirectoryIcon(path, status string) {
	dotDir := filepath.Join(path, ".directory")
	icon := iconsLinux[status]
	if icon == "" {
		icon = "folder"
	}

	content := fmt.Sprintf("[Desktop Entry]\nIcon=%s\n", icon)
	os.WriteFile(dotDir, []byte(content), 0644)
	
	// Touch directory to refresh view
	exec.Command("touch", path).Run()
}

func refreshUI() {
	// Try Dolphin refresh via qdbus
	for _, bin := range []string{"qdbus", "qdbus6"} {
		if path, err := exec.LookPath(bin); err == nil {
			exec.Command(path, "org.kde.dolphin", "/dolphin/Dolphin_1", "refresh").Run()
		}
	}
}
