//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/backup"
	"skillshare/internal/sync"
	"skillshare/internal/testutil"
)

func r13Sandbox(t *testing.T) *testutil.Sandbox {
	t.Helper()
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
		t.Setenv(key, "")
	}
	sb := testutil.NewSandbox(t)
	t.Cleanup(sb.Cleanup)
	return sb
}

// This records baseline behavior; it does not assert profile preservation.
func TestProfiles_BaselinePruneRisk(t *testing.T) {
	sb := r13Sandbox(t)
	sb.CreateSkill("keep-me", map[string]string{"SKILL.md": "# Keep"})
	sb.CreateSkill("edit-me", map[string]string{"SKILL.md": "# Source"})
	target := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + target + "\n    mode: copy\n")
	sb.RunCLI("sync").AssertSuccess(t)
	entry := filepath.Join(target, "edit-me")
	if sb.IsSymlink(entry) || !sb.FileExists(filepath.Join(entry, "SKILL.md")) {
		t.Fatal("initial sync did not create a real copy")
	}
	manifest, err := sync.ReadManifest(target)
	if err != nil || manifest.Managed["edit-me"] == "" || manifest.Managed["edit-me"] == "symlink" {
		t.Fatalf("copy not recorded in manifest: %+v, %v", manifest, err)
	}
	if err := os.WriteFile(filepath.Join(entry, "SKILL.md"), []byte("# Target edit"), 0644); err != nil {
		t.Fatal(err)
	}
	sb.RunCLI("target", "claude", "--mode", "merge", "--add-exclude", "edit-me").AssertSuccess(t)
	sb.RunCLI("sync").AssertSuccess(t)
	if _, err := os.Lstat(entry); !os.IsNotExist(err) {
		t.Fatalf("edited target entry was not removed: %v", err)
	}
	manifest, err = sync.ReadManifest(target)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manifest.Managed["edit-me"]; ok {
		t.Fatal("removed entry remains in manifest")
	}
	if !sb.FileExists(filepath.Join(target, "keep-me", "SKILL.md")) || sb.ReadFile(filepath.Join(sb.SourcePath, "edit-me", "SKILL.md")) != "# Source" {
		t.Fatal("remaining target or source changed unexpectedly")
	}
}

// Separate homes make the text run a positive control for the JSON run.
func TestProfiles_BaselineJSONBackupGap(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		name := "text"
		if jsonOutput {
			name = "json"
		}
		t.Run(name, func(t *testing.T) {
			sb := r13Sandbox(t)
			sb.CreateSkill("fixture", map[string]string{"SKILL.md": "# Fixture"})
			target := sb.CreateTarget("claude")
			if err := os.WriteFile(filepath.Join(target, "sentinel"), []byte("original"), 0644); err != nil {
				t.Fatal(err)
			}
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + target + "\n    mode: copy\n")
			backupDir := filepath.Join(sb.Home, ".local", "share", "skillshare", "backups")
			before, err := backup.ListInDir(backupDir)
			if err != nil || len(before) != 0 {
				t.Fatalf("expected no previous backups: %v, %v", before, err)
			}
			args := []string{"sync"}
			if jsonOutput {
				args = append(args, "--json")
			}
			sb.RunCLI(args...).AssertSuccess(t)
			if !sb.FileExists(filepath.Join(target, "fixture", "SKILL.md")) {
				t.Fatal("skill was not synced")
			}
			backups, err := backup.ListInDir(backupDir)
			if err != nil {
				t.Fatal(err)
			}
			paths, err := filepath.Glob(filepath.Join(backupDir, "*", "claude", "sentinel"))
			if err != nil {
				t.Fatal(err)
			}
			if jsonOutput {
				if len(backups) != 0 || len(paths) != 0 {
					t.Fatalf("JSON sync made a backup: %+v, %v", backups, paths)
				}
			} else if len(backups) != 1 || len(paths) != 1 || sb.ReadFile(paths[0]) != "original" {
				t.Fatalf("text sync did not back up sentinel: %+v, %v", backups, paths)
			}
		})
	}
}
