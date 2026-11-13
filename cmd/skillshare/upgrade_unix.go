//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	versionpkg "skillshare/internal/version"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func needsSudo(path string) bool {
	return unix.Access(filepath.Dir(path), unix.W_OK) != nil
}

// execFunc runs sudo and waits for it. Overridden in tests.
var execFunc = func(argv0 string, argv []string, envv []string) error {
	cmd := exec.Command(argv0, argv[1:]...)
	cmd.Env = envv
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// stdinIsTTY reports whether a user can answer a sudo prompt. Overridden in tests.
var stdinIsTTY = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// upgradeBinaryWithSudo replaces the binary in a sudo child that does nothing
// else, so the skill, UI assets, caches, and logs stay owned by the user.
func upgradeBinaryWithSudo(execPath, targetVersion string) error {
	sudoPath, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("sudo not found, please run: sudo %s", strings.Join(os.Args, " "))
	}
	args := []string{"sudo"}
	if !stdinIsTTY() {
		// sudo prompts on /dev/tty, not stdin: without -n a dashboard-spawned
		// upgrade waits for a password nobody can see until it times out.
		if exec.Command(sudoPath, "-n", "true").Run() != nil {
			return fmt.Errorf("a password is needed to write to %s; %s%s", filepath.Dir(execPath), versionpkg.SudoUpgradeHintPrefix, versionpkg.SudoUpgradeHint)
		}
		args = append(args, "-n")
	}
	args = append(args, execPath, "upgrade", replaceBinaryFlag+"="+targetVersion)
	if err := execFunc(sudoPath, args, os.Environ()); err != nil {
		return fmt.Errorf("sudo upgrade failed: %w", err)
	}
	return nil
}
