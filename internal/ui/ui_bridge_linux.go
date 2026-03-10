//go:build !windows

package ui

import (
	"fmt"
	"os/exec"
)

func UpdateDirectoryIcon(path, status string) bool {
	return false
}

func RefreshUI(_ []string) {
	// Logic removed
}

func ShowMessage(title, message string) {
	if path, err := exec.LookPath("zenity"); err == nil {
		exec.Command(path, "--error", "--title="+title, "--text="+message).Run()
	} else {
		fmt.Printf("[%s] %s\n", title, message)
	}
}
