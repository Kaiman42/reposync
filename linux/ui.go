//go:build !windows

package linux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var IconsLinux = map[string]string{
	"not_init":     "folder-black",
	"commit":       "folder-yellow",
	"untracked":    "folder-red",
	"no_remote":    "folder-orange",
	"synced":       "folder-green",
	"pending_sync": "folder-violet",
}

func UpdateDirectoryIcon(path, status string) bool {
	dotDir := filepath.Join(path, ".directory")
	icon := IconsLinux[status]
	if icon == "" {
		icon = "folder"
	}

	content := fmt.Sprintf("[Desktop Entry]\nIcon=%s\n", icon)

	// Verifica se mudou
	old, _ := os.ReadFile(dotDir)
	if string(old) == content {
		return false
	}

	os.WriteFile(dotDir, []byte(content), 0644)

	// Touch directory to refresh view
	exec.Command("touch", path).Run()
	return true
}

func RefreshUI(paths []string) {
	// Try Dolphin refresh via qdbus
	for _, bin := range []string{"qdbus", "qdbus6"} {
		if path, err := exec.LookPath(bin); err == nil {
			exec.Command(path, "org.kde.dolphin", "/dolphin/Dolphin_1", "refresh").Run()
		}
	}
}
