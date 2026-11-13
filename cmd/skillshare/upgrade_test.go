//go:build !windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNeedsSudo_WritableDir(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "skillshare")
	if needsSudo(fakeBin) {
		t.Error("expected needsSudo=false for writable temp dir")
	}
}

func TestNeedsSudo_NonWritableDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}

	dir := t.TempDir()
	os.Chmod(dir, 0555)
	t.Cleanup(func() { os.Chmod(dir, 0755) })

	fakeBin := filepath.Join(dir, "skillshare")
	if !needsSudo(fakeBin) {
		t.Error("expected needsSudo=true for read-only dir")
	}
}

func TestNeedsSudo_NonExistentDir(t *testing.T) {
	fakeBin := filepath.Join(t.TempDir(), "nonexistent", "skillshare")
	if !needsSudo(fakeBin) {
		t.Error("expected needsSudo=true for non-existent dir")
	}
}

func TestUpgradeBinaryWithSudo_NoSudoInPath(t *testing.T) {
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	err := upgradeBinaryWithSudo("/usr/local/bin/skillshare", "0.22.0")
	if err == nil {
		t.Fatal("expected error when sudo is not in PATH")
	}
	if got := err.Error(); !strings.Contains(got, "sudo not found") {
		t.Errorf("expected 'sudo not found' in error, got: %s", got)
	}
}

func TestUpgradeBinaryWithSudo_ExecArgs(t *testing.T) {
	// Capture what execFunc receives
	var gotPath string
	var gotArgs []string

	orig := execFunc
	execFunc = func(argv0 string, argv []string, envv []string) error {
		gotPath = argv0
		gotArgs = argv
		return nil
	}
	defer func() { execFunc = orig }()

	origTTY := stdinIsTTY
	stdinIsTTY = func() bool { return true }
	defer func() { stdinIsTTY = origTTY }()

	// Fake sudo in PATH
	dir := t.TempDir()
	fakeSudo := filepath.Join(dir, "sudo")
	os.WriteFile(fakeSudo, []byte("#!/bin/sh\n"), 0755)
	t.Setenv("PATH", dir)

	// The user's flags must not reach root: only the binary replacement runs there.
	origArgs := os.Args
	os.Args = []string{"skillshare", "upgrade", "--force"}
	defer func() { os.Args = origArgs }()

	err := upgradeBinaryWithSudo("/usr/local/bin/skillshare", "0.22.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != fakeSudo {
		t.Errorf("exec path = %q, want %q", gotPath, fakeSudo)
	}
	wantArgs := []string{"sudo", "/usr/local/bin/skillshare", "upgrade", "--replace-binary=0.22.0"}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", gotArgs, wantArgs)
	}
	for i, w := range wantArgs {
		if gotArgs[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, gotArgs[i], w)
		}
	}
}

func TestUpgradeBinaryWithSudo_NonInteractiveWithoutTTY(t *testing.T) {
	var gotArgs []string
	orig := execFunc
	execFunc = func(_ string, argv []string, _ []string) error {
		gotArgs = argv
		return nil
	}
	defer func() { execFunc = orig }()

	origTTY := stdinIsTTY
	stdinIsTTY = func() bool { return false }
	defer func() { stdinIsTTY = origTTY }()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sudo"), []byte("#!/bin/sh\n"), 0755)
	t.Setenv("PATH", dir)

	if err := upgradeBinaryWithSudo("/usr/local/bin/skillshare", "0.22.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotArgs) < 2 || gotArgs[1] != "-n" {
		t.Errorf("args = %v, want sudo -n so a missing TTY fails fast", gotArgs)
	}
}

func TestUpgradeBinaryWithSudo_PasswordNeededWithoutTTY(t *testing.T) {
	orig := execFunc
	execFunc = func(string, []string, []string) error {
		t.Error("must not exec sudo when it would need a password")
		return nil
	}
	defer func() { execFunc = orig }()

	origTTY := stdinIsTTY
	stdinIsTTY = func() bool { return false }
	defer func() { stdinIsTTY = origTTY }()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sudo"), []byte("#!/bin/sh\nexit 1\n"), 0755)
	t.Setenv("PATH", dir)

	err := upgradeBinaryWithSudo("/usr/local/bin/skillshare", "0.22.0")
	if err == nil || !strings.Contains(err.Error(), "run in a terminal: skillshare upgrade") {
		t.Errorf("err = %v, want the terminal command hint without a sudo prefix", err)
	}
}

func TestUpgradeBinaryWithSudo_ChildFailureIsReturned(t *testing.T) {
	orig := execFunc
	execFunc = func(string, []string, []string) error { return errors.New("exit status 1") }
	defer func() { execFunc = orig }()

	origTTY := stdinIsTTY
	stdinIsTTY = func() bool { return true }
	defer func() { stdinIsTTY = origTTY }()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sudo"), []byte("#!/bin/sh\n"), 0755)
	t.Setenv("PATH", dir)

	if err := upgradeBinaryWithSudo("/usr/local/bin/skillshare", "0.22.0"); err == nil {
		t.Error("expected the sudo child's failure to stop the upgrade")
	}
}

// A skill left owned by root by an older sudo upgrade cannot be replaced; the error should
// say how to take the files back instead of only "permission denied".
func TestSkillPermissionHint_NamesTheChownCommand(t *testing.T) {
	err := &os.PathError{Op: "unlinkat", Path: "/home/me/skills/skillshare/references/trash.md", Err: os.ErrPermission}
	got := skillPermissionHint(errors.Join(errors.New("failed to remove existing skill"), err), "/home/me/it's skills").Error()
	if !strings.Contains(got, `sudo chown -R "$(id -un)" '/home/me/it'\''s skills'`) {
		t.Errorf("hint = %q", got)
	}
	if !strings.Contains(got, "permission denied") {
		t.Errorf("hint dropped the original error: %q", got)
	}
}

func TestSkillPermissionHint_LeavesOtherErrorsAlone(t *testing.T) {
	err := errors.New("network is unreachable")
	if got := skillPermissionHint(err, "/x"); got != err {
		t.Errorf("got %v, want the error unchanged", got)
	}
}
