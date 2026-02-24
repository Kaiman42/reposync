//go:build !windows
package main

import "github.com/Kaiman42/reposync/linux"

func updateDirectoryIcon(path, status string) bool {
	return linux.UpdateDirectoryIcon(path, status)
}

func refreshUI(paths []string) {
	linux.RefreshUI(paths)
}
