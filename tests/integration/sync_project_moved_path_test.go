//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// goose and openhands moved their default project path to .agents/skills. A
// project config only stores the target name, so the next sync writes to the new
// path — the entries skillshare left in the old one have to go, or the runtime
// reads both and shows every skill twice.

// setupMovedGooseProject creates a project that already synced goose into the
// legacy .goose/skills, then drops the pinned path so the default applies again.
func setupMovedGooseProject(t *testing.T, sb *testutil.Sandbox, mode string) string {
	t.Helper()
	projectRoot := sb.SetupProjectDir()
	sb.CreateProjectSkill(projectRoot, "legacy-skill", map[string]string{"SKILL.md": "# Legacy Skill"})

	legacy := "targets:\n  - name: goose\n    path: .goose/skills\n"
	if mode != "" {
		legacy += "    mode: " + mode + "\n"
	}
	sb.WriteProjectConfig(projectRoot, legacy)
	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	// A folder the user made by hand, which sync must never touch.
	localDir := filepath.Join(projectRoot, ".goose", "skills", "my-local")
	os.MkdirAll(localDir, 0755)
	os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte("# Local"), 0644)
	// Hand-made folders whose names merely look like skillshare's flat names.
	for _, name := range []string{"_drafts", "team__notes"} {
		os.MkdirAll(filepath.Join(projectRoot, ".goose", "skills", name), 0755)
	}

	sb.WriteProjectConfig(projectRoot, "targets:\n  - goose\n")
	return projectRoot
}

func TestSyncProject_MovedDefaultPath_CleansLegacyDir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := setupMovedGooseProject(t, sb, "")

	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(projectRoot, ".agents", "skills", "legacy-skill")) {
		t.Error("skill should be linked into the new .agents/skills")
	}
	if sb.FileExists(filepath.Join(projectRoot, ".goose", "skills", "legacy-skill")) {
		t.Error("leftover link in .goose/skills should be removed")
	}
	if !sb.FileExists(filepath.Join(projectRoot, ".goose", "skills", "my-local", "SKILL.md")) {
		t.Error("user-created local folder should be preserved")
	}
	for _, name := range []string{"_drafts", "team__notes"} {
		if !sb.FileExists(filepath.Join(projectRoot, ".goose", "skills", name)) {
			t.Errorf("hand-made folder %q is not tracked by skillshare and must be preserved", name)
		}
	}
}

func TestSyncProject_MovedDefaultPath_DryRunPreviewsOnly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := setupMovedGooseProject(t, sb, "")

	result := sb.RunCLIInDir(projectRoot, "sync", "-p", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Would clean")

	if !sb.FileExists(filepath.Join(projectRoot, ".goose", "skills", "legacy-skill")) {
		t.Error("dry run must not remove the leftover link")
	}
}

func TestSyncProject_MovedDefaultPath_CopyMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := setupMovedGooseProject(t, sb, "copy")

	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	if !sb.FileExists(filepath.Join(projectRoot, ".agents", "skills", "legacy-skill", "SKILL.md")) {
		t.Error("skill should be copied into the new .agents/skills")
	}
	if sb.FileExists(filepath.Join(projectRoot, ".goose", "skills", "legacy-skill")) {
		t.Error("leftover copy in .goose/skills should be removed")
	}
	if !sb.FileExists(filepath.Join(projectRoot, ".goose", "skills", "my-local", "SKILL.md")) {
		t.Error("user-created local folder should be preserved")
	}
}

func TestSyncProject_MovedDefaultPath_ExplicitPathOptsOut(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir()
	sb.CreateProjectSkill(projectRoot, "pinned-skill", map[string]string{"SKILL.md": "# Pinned Skill"})
	sb.WriteProjectConfig(projectRoot, "targets:\n  - name: goose\n    path: .goose/skills\n")

	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)
	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(projectRoot, ".goose", "skills", "pinned-skill")) {
		t.Error("a pinned path must keep syncing into .goose/skills")
	}
}

func TestSyncProject_MovedDefaultPath_KeepsDirOwnedByAnotherTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir()
	sb.CreateProjectSkill(projectRoot, "shared-skill", map[string]string{"SKILL.md": "# Shared Skill"})
	// .claude/skills is in goose's also_scans, but claude writes there.
	sb.WriteProjectConfig(projectRoot, "targets:\n  - claude\n  - goose\n")

	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(projectRoot, ".claude", "skills", "shared-skill")) {
		t.Error("a directory another configured target writes to must not be cleaned")
	}
}
