//go:build windows

package server

import "golang.org/x/sys/windows"

func replaceFile(tempPath, fullPath string) error {
	return windows.Rename(tempPath, fullPath)
}
