//go:build windows
package main

import "github.com/Kaiman42/reposync/windows"

func updateDirectoryIcon(path, status string) bool {
	return windows.UpdateDirectoryIcon(path, status)
}

func refreshUI(paths []string) {
	windows.RefreshUI(paths)
}
