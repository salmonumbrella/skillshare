//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

// TestInstallBranch_PinnedSHA verifies --branch accepts a commit SHA and installs
// exactly that revision even after the branch has moved on.
func TestInstallBranch_PinnedSHA(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	remoteRepo := filepath.Join(sb.Root, "remote-repo.git")
	gitInit(t, remoteRepo, true)
	workDir := filepath.Join(sb.Root, "work")
	gitClone(t, remoteRepo, workDir)

	writeSkill := func(body string) {
		if err := os.WriteFile(filepath.Join(workDir, "SKILL.md"), []byte("---\nname: pinned\n---\n# "+body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeSkill("v1")
	gitAddCommit(t, workDir, "v1")
	tag := exec.Command("git", "tag", "v1")
	tag.Dir = workDir
	if out, err := tag.CombinedOutput(); err != nil {
		t.Fatalf("git tag: %s %v", out, err)
	}
	rev := exec.Command("git", "rev-parse", "HEAD")
	rev.Dir = workDir
	out, err := rev.Output()
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.TrimSpace(string(out))
	writeSkill("v2")
	gitAddCommit(t, workDir, "v2")
	gitPush(t, workDir)
	pushTags := exec.Command("git", "push", "origin", "--tags")
	pushTags.Dir = workDir
	if out, err := pushTags.CombinedOutput(); err != nil {
		t.Fatalf("git push --tags: %s %v", out, err)
	}

	// Tracked repos must stay on a branch to pull; a pinned tag or SHA is rejected.
	for _, ref := range []string{sha, "v1"} {
		tracked := sb.RunCLI("install", "file://"+remoteRepo, "--track", "--branch", ref, "--name", "tracked-"+ref[:2], "--skip-audit")
		tracked.AssertFailure(t)
		tracked.AssertAnyOutputContains(t, "must follow a branch")
	}

	result := sb.RunCLI("install", "file://"+remoteRepo, "--branch", sha, "--name", "pinned", "--skip-audit")
	result.AssertSuccess(t)

	content := sb.ReadFile(filepath.Join(sb.SourcePath, "pinned", "SKILL.md"))
	if !strings.Contains(content, "# v1") {
		t.Errorf("installed content should be the pinned v1 revision, got:\n%s", content)
	}

	store, err := install.LoadMetadata(sb.SourcePath)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	entry := store.Get("pinned")
	if entry == nil {
		t.Fatal("expected metadata entry for pinned")
	}
	if entry.Branch != sha {
		t.Errorf("metadata branch = %q, want %q", entry.Branch, sha)
	}
	if !strings.HasPrefix(sha, entry.Version) {
		t.Errorf("metadata version = %q, want prefix of %s", entry.Version, sha)
	}
}
