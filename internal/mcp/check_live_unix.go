//go:build !windows

package mcp

import (
	"os/exec"
	"syscall"
)

// startInGroup gives the server its own process group, so stopping it reaches its children.
func startInGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateGroup(cmd *exec.Cmd) { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }

func killGroup(cmd *exec.Cmd) { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
