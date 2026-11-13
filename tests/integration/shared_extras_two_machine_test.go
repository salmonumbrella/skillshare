//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestSharedExtrasTwoMachinePullAndRepair(t *testing.T) {
	a := testutil.NewSandbox(t)
	defer a.Cleanup()
	b := testutil.NewSandbox(t)
	defer b.Cleanup()
	activate := func(sb *testutil.Sandbox) {
		sb.SetEnv("HOME", sb.Home)
		sb.SetEnv("USERPROFILE", sb.Home)
		sb.SetEnv("SKILLSHARE_CONFIG", sb.ConfigPath)
		sb.SetEnv("XDG_CONFIG_HOME", filepath.Join(sb.Home, ".config"))
		sb.SetEnv("XDG_DATA_HOME", filepath.Join(sb.Home, ".local", "share"))
		sb.SetEnv("XDG_STATE_HOME", filepath.Join(sb.Home, ".local", "state"))
		sb.IsolateNativeClients()
	}
	for _, sb := range []*testutil.Sandbox{a, b} {
		sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\ngit_root: root\nshared: {path: ./config.shared.yaml}\ntargets: {}\n")
		sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), ".gitignore"), "config.yaml\n")
	}
	remote := testutil.SetupBareRemoteRepo(t, t.TempDir())
	aRoot, bRoot := filepath.Dir(a.ConfigPath), filepath.Dir(b.ConfigPath)
	testutil.RunGit(t, aRoot, "init")
	testutil.ConfigureGitUser(t, aRoot)
	testutil.RunGit(t, aRoot, "remote", "add", "origin", remote)
	a.WriteFile(filepath.Join(aRoot, "extras", "rules", "fixture.md"), "shared rules\n")
	a.WriteFile(filepath.Join(aRoot, "config.shared.yaml"), "version: 1\nextras: [{name: rules, source: ./extras/rules, targets: [{path: ~/placed, mode: copy}]}]\n")
	activate(a)
	a.RunCLI("push").AssertSuccess(t)
	branch := testutil.RunGit(t, aRoot, "branch", "--show-current")
	testutil.RunGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/"+branch)
	testutil.RunGit(t, bRoot, "init")
	testutil.ConfigureGitUser(t, bRoot)
	testutil.RunGit(t, bRoot, "add", ".gitignore")
	testutil.RunGit(t, bRoot, "commit", "-m", "fixture local root scaffold")
	testutil.RunGit(t, bRoot, "remote", "add", "origin", remote)
	activate(b)
	b.RunCLI("pull").AssertSuccess(t)
	if b.ReadFile(filepath.Join(b.Home, "placed", "fixture.md")) != "shared rules\n" {
		t.Fatal("pull did not apply shared extra")
	}
	b.RunCLI("pull").AssertSuccess(t) // up-to-date pull also reads current sharing
	// A remotely introduced unusable declaration must still be fetchable.
	a.WriteFile(filepath.Join(aRoot, "config.shared.yaml"), "version: 2\n")
	testutil.RunGit(t, aRoot, "add", "config.shared.yaml")
	testutil.RunGit(t, aRoot, "commit", "-m", "fixture newer shared version")
	testutil.RunGit(t, aRoot, "push")
	before := testutil.RunGit(t, bRoot, "rev-parse", "HEAD")
	activate(b)
	result := b.RunCLI("pull")
	if result.ExitCode != 3 || !strings.Contains(result.Stdout+result.Stderr, "shared_unavailable") {
		t.Fatalf("unusable pull=%+v", result)
	}
	if testutil.RunGit(t, bRoot, "rev-parse", "HEAD") == before {
		t.Fatal("unavailable sharing prevented fetch")
	}
	// A corrected remote file removes the last declaration, and pull detaches it.
	a.WriteFile(filepath.Join(aRoot, "config.shared.yaml"), "version: 1\nextras: []\n")
	testutil.RunGit(t, aRoot, "add", "config.shared.yaml")
	testutil.RunGit(t, aRoot, "commit", "-m", "fixture repaired removal")
	testutil.RunGit(t, aRoot, "push")
	activate(b)
	b.RunCLI("pull").AssertSuccess(t)
	if _, err := os.Lstat(filepath.Join(b.Home, "placed", "fixture.md")); !os.IsNotExist(err) {
		t.Fatal("last extra remained after pull")
	}
	b.RunCLI("sync", "--all", "--json", "-g").AssertSuccess(t)
}
