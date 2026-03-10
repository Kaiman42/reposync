//go:build !windows

package main

import (
	"fmt"
	"os/exec"
)


func updateDirectoryIcon(path, status string) bool {
	return false
}

func refreshUI(_ []string) {
	// Logic removed
}

func showMessage(title, message string) {
	if path, err := exec.LookPath("zenity"); err == nil {
		exec.Command(path, "--error", "--title="+title, "--text="+message).Run()
	} else {
		fmt.Printf("[%s] %s\n", title, message)
	}
}
