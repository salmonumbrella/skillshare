//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// projectsSandbox has two skills, one agent and a project folder declared in the
// global config under projects.
func projectsSandbox(t *testing.T, project string) (*testutil.Sandbox, string) {
	t.Helper()
	sb := testutil.NewSandbox(t)
	sb.CreateSkill("team-lint", map[string]string{"SKILL.md": "---\nname: team-lint\n---\n# Lint"})
	sb.CreateSkill("draft", map[string]string{"SKILL.md": "---\nname: draft\n---\n# Draft"})
	sb.WriteFile(filepath.Join(filepath.Dir(sb.SourcePath), "agents", "reviewer.md"), "# Reviewer")
	root := filepath.Join(sb.Home, "work", "app")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nprojects:\n  " + root + ":\n" + project)
	return sb, root
}

func TestProjects_SyncWritesSkillsAndAgentsIntoTheFolder(t *testing.T) {
	sb, root := projectsSandbox(t, "    targets: [claude, codex]\n    skills:\n      mode: copy\n      include: [team-*]\n    agents: {}\n")
	defer sb.Cleanup()

	sb.RunCLI("sync", "--all").AssertSuccess(t)

	for _, path := range []string{".claude/skills/team-lint/SKILL.md", ".agents/skills/team-lint/SKILL.md", ".claude/agents/reviewer.md"} {
		if !sb.FileExists(filepath.Join(root, path)) {
			t.Errorf("%s was not written", path)
		}
	}
	if sb.FileExists(filepath.Join(root, ".claude", "skills", "draft")) {
		t.Error("draft is not included but was written")
	}
	if sb.FileExists(filepath.Join(root, ".skillshare")) {
		t.Error("the project folder got a .skillshare/")
	}
}

func TestProjects_CollectLeavesAProjectsOwnSkills(t *testing.T) {
	sb, root := projectsSandbox(t, "    targets: [claude]\n    skills: {}\n")
	defer sb.Cleanup()
	sb.RunCLI("sync").AssertSuccess(t)
	sb.WriteFile(filepath.Join(root, ".claude", "skills", "local-only", "SKILL.md"), "---\nname: local-only\n---\n")

	sb.RunCLI("collect", "--all", "--force")

	if sb.FileExists(filepath.Join(sb.SourcePath, "local-only")) {
		t.Error("collect pulled a skill out of a project")
	}
}

func TestProjects_TargetCommandDoesNotSeeProjectTargets(t *testing.T) {
	sb, _ := projectsSandbox(t, "    targets: [claude]\n    skills: {}\n")
	defer sb.Cleanup()

	result := sb.RunCLI("target", "remove", "app@claude")

	result.AssertFailure(t)
	if !sb.FileExists(sb.ConfigPath) {
		t.Fatal("config is gone")
	}
	sb.RunCLI("status").AssertAnyOutputContains(t, "app@claude")
}
