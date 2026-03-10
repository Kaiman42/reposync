//go:build !windows
// +build !windows

package ui

import "syscall"

func GetSysProcAttr() *syscall.SysProcAttr {
	return nil
}
