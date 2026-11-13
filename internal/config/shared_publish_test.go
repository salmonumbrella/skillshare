package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestSharedPublicationWorktreeAllScopes(t *testing.T) {
	for _, scope := range []string{"skills", "agents", "extras", "root"} {
		t.Run(scope, func(t *testing.T) {
			sb, cfg := sharedFixture(t, "git_root: "+scope+"\ntargets: {}\n", "version: 1\n")
			gitDir := cfg.EffectiveGitRoot()
			sb.WriteFile(filepath.Join(gitDir, "fixture.txt"), "fixture")
			testutil.RunGit(t, gitDir, "init")
			sb.WriteFile(filepath.Join(gitDir, "config.shared.yaml"), "version: 2\n")
			before := testutil.RunGit(t, gitDir, "status", "--porcelain=v1")
			err := cfg.CheckSharedWorktree(gitDir)
			var shared *SharedError
			if !errors.As(err, &shared) || shared.Code != SharedPublication {
				t.Fatalf("worktree=%v", err)
			}
			if testutil.RunGit(t, gitDir, "status", "--porcelain=v1") != before {
				t.Fatal("refusal changed index")
			}
		})
	}
}

func TestSharedOutgoingChecksOldPointerAndFirstCommit(t *testing.T) {
	sb, cfg := sharedFixture(t, "targets: {}\n", "version: 1\n")
	dir := cfg.EffectiveGitRoot()
	testutil.RunGit(t, dir, "init")
	testutil.ConfigureGitUser(t, dir)
	path := filepath.Join(dir, "old-team.yaml")
	// Assemble synthetic secret-shaped content at runtime; never commit a token.
	key := strings.Join([]string{"api", "key"}, "_")
	value := "sk-" + strings.Repeat("x", 48)
	sb.WriteFile(path, "version: 1\n# "+key+" = '"+value+"'\n")
	testutil.RunGit(t, dir, "add", "old-team.yaml")
	testutil.RunGit(t, dir, "commit", "-m", "unsafe declaration")
	first := strings.TrimSpace(testutil.RunGit(t, dir, "rev-parse", "--short", "HEAD"))
	sb.WriteFile(path, "version: 1\n")
	testutil.RunGit(t, dir, "add", "old-team.yaml")
	testutil.RunGit(t, dir, "commit", "-m", "clean declaration")
	state, err := LoadSharedState(sb.ConfigPath, dir, filepath.Dir(cfg.SharedStatus().Path))
	if err != nil {
		t.Fatal(err)
	}
	state.RecordPointer(path)
	state.RecordPointer(cfg.SharedStatus().Path)
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	paths, err := cfg.SharedGitPaths(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(paths, "old-team.yaml") || !slices.Contains(paths, "config.shared.yaml") {
		t.Fatal(paths)
	}
	err = cfg.CheckSharedOutgoing(dir, install.ParseOptions{})
	var shared *SharedError
	if !errors.As(err, &shared) || shared.Code != SharedPublication || !strings.Contains(err.Error(), first) || strings.Contains(err.Error(), value) {
		t.Fatalf("outgoing=%v", err)
	}
}

func TestSharedNoPointerPublicationDoesNoSharedStateIO(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\ntargets: {}\n")
	cfg, err := LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	path, err := SharedStatePath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(path, "invalid state")
	if err := cfg.CheckSharedWorktree(sb.SourcePath); err != nil {
		t.Fatal(err)
	}
	if err := cfg.CheckSharedOutgoing(sb.SourcePath, install.ParseOptions{}); err != nil {
		t.Fatal(err)
	}
	if sb.ReadFile(path) != "invalid state" {
		t.Fatal("no-pointer touched state")
	}
}

func TestSharedPublicationRefusesMalformedStateBeforeAnyWrite(t *testing.T) {
	sb, cfg := sharedFixture(t, "targets: {}\n", "version: 1\n")
	path, err := SharedStatePath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(path, "version: [invalid\n")
	before := sb.ReadFile(path)
	for _, check := range []func() error{
		func() error { _, err := cfg.SharedGitPaths(sb.SourcePath); return err },
		func() error { return cfg.CheckSharedWorktree(sb.SourcePath) },
		func() error { return cfg.CheckSharedOutgoing(sb.SourcePath, install.ParseOptions{}) },
		func() error { return cfg.RecordSharedPointer(sb.SourcePath, true) },
	} {
		var shared *SharedError
		if err := check(); !errors.As(err, &shared) || shared.Code != SharedPublication {
			t.Fatalf("check=%v", err)
		}
		if sb.ReadFile(path) != before {
			t.Fatal("state changed")
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.SharedGitPaths(sb.SourcePath); err != nil {
		t.Fatal("fresh state must be valid absence", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("read created state")
	}
}
