//go:build windows

package mcp

import "os/exec"

func startInGroup(*exec.Cmd) {}

// Windows has no signals to escalate through; the server is terminated at once.
func terminateGroup(cmd *exec.Cmd) { _ = cmd.Process.Kill() }

func killGroup(cmd *exec.Cmd) { _ = cmd.Process.Kill() }
