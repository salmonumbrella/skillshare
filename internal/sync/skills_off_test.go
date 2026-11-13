package sync

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"skillshare/internal/config"
)

type skillsOffFixture struct {
	source, target string
}

func newSkillsOffFixture(t *testing.T) skillsOffFixture {
	t.Helper()
	root := t.TempDir()
	f := skillsOffFixture{source: filepath.Join(root, "source"), target: filepath.Join(root, "target")}
	for _, skill := range []string{"alpha", "beta"} {
		mustWrite(t, filepath.Join(f.source, skill, "SKILL.md"), "# "+skill)
	}
	return f
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustLink(t *testing.T, dest, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dest, link); err != nil {
		t.Fatal(err)
	}
}

func onlyTarget(path string) map[string]config.TargetConfig {
	return map[string]config.TargetConfig{"gemini": {Skills: &config.ResourceTargetConfig{Path: path}}}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestDetachSkills_MergeRemovesSourceLinksOnly(t *testing.T) {
	f := newSkillsOffFixture(t)
	external := filepath.Join(filepath.Dir(f.source), "elsewhere")
	mustWrite(t, filepath.Join(external, "SKILL.md"), "# ext")
	mustLink(t, filepath.Join(f.source, "alpha"), filepath.Join(f.target, "alpha"))
	mustLink(t, filepath.Join(f.source, "beta"), filepath.Join(f.target, "beta"))
	mustLink(t, external, filepath.Join(f.target, "ext"))
	mustWrite(t, filepath.Join(f.target, "local", "SKILL.md"), "# local")
	if err := WriteManifest(f.target, &Manifest{Managed: map[string]string{"alpha": "symlink", "beta": "symlink"}}); err != nil {
		t.Fatal(err)
	}

	res, err := DetachSkills(onlyTarget(f.target), "gemini", f.source, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Removed, []string{"alpha", "beta"}) {
		t.Errorf("Removed = %v", res.Removed)
	}
	if !slices.Equal(res.Kept, []string{"ext", "local"}) {
		t.Errorf("Kept = %v", res.Kept)
	}
	if exists(filepath.Join(f.target, "alpha")) || !exists(filepath.Join(f.target, "ext")) || !exists(filepath.Join(f.target, "local", "SKILL.md")) {
		t.Error("wrong entries removed")
	}
	if !exists(filepath.Join(f.source, "alpha", "SKILL.md")) {
		t.Error("source must stay")
	}
	if exists(filepath.Join(f.target, ManifestFile)) {
		t.Error("manifest with no entries left should be removed")
	}
}

func TestDetachSkills_CopyKeepsCopiesAndManifest(t *testing.T) {
	f := newSkillsOffFixture(t)
	mustWrite(t, filepath.Join(f.target, "alpha", "SKILL.md"), "# alpha")
	if err := WriteManifest(f.target, &Manifest{Managed: map[string]string{"alpha": "abc"}}); err != nil {
		t.Fatal(err)
	}

	res, err := DetachSkills(onlyTarget(f.target), "gemini", f.source, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Removed) != 0 || len(res.Kept) != 0 || !slices.Equal(res.Copies, []string{"alpha"}) {
		t.Errorf("res = %+v", res)
	}
	m, _ := ReadManifest(f.target)
	if m.Managed["alpha"] != "abc" {
		t.Errorf("copy manifest entry lost: %v", m.Managed)
	}
}

func TestDetachSkills_RootLinkUnlinkedNotFollowed(t *testing.T) {
	f := newSkillsOffFixture(t)
	mustLink(t, f.source, f.target)

	res, err := DetachSkills(onlyTarget(f.target), "gemini", f.source, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Removed, []string{f.target}) {
		t.Errorf("Removed = %v", res.Removed)
	}
	if exists(f.target) {
		t.Error("root link should be gone")
	}
	if !exists(filepath.Join(f.source, "alpha", "SKILL.md")) {
		t.Error("source content must stay")
	}
}

func TestDetachSkills_ExternalRootLinkKept(t *testing.T) {
	f := newSkillsOffFixture(t)
	other := filepath.Join(filepath.Dir(f.source), "other")
	mustLink(t, filepath.Join(f.source, "alpha"), filepath.Join(other, "alpha"))
	mustLink(t, other, f.target)

	res, err := DetachSkills(onlyTarget(f.target), "gemini", f.source, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Removed) != 0 || !slices.Equal(res.Kept, []string{f.target}) {
		t.Errorf("res = %+v", res)
	}
	if !exists(filepath.Join(other, "alpha")) {
		t.Error("must not descend into an external root link")
	}
}

func TestDetachSkills_SharedWithEnabledTargetIsNoop(t *testing.T) {
	f := newSkillsOffFixture(t)
	mustLink(t, filepath.Join(f.source, "alpha"), filepath.Join(f.target, "alpha"))
	targets := map[string]config.TargetConfig{
		"codex":     {Skills: &config.ResourceTargetConfig{Path: f.target}},
		"universal": {Skills: &config.ResourceTargetConfig{Path: f.target}},
	}

	res, err := DetachSkills(targets, "codex", f.source, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.SharedWith != "universal" || len(res.Removed) != 0 || len(res.Kept) != 0 {
		t.Errorf("res = %+v", res)
	}
	if !exists(filepath.Join(f.target, "alpha")) {
		t.Error("shared folder must stay untouched")
	}
}

func TestDetachSkills_DryRunWritesNothing(t *testing.T) {
	f := newSkillsOffFixture(t)
	mustLink(t, filepath.Join(f.source, "alpha"), filepath.Join(f.target, "alpha"))
	mustWrite(t, filepath.Join(f.target, "local", "SKILL.md"), "# local")
	if err := WriteManifest(f.target, &Manifest{Managed: map[string]string{"alpha": "symlink"}}); err != nil {
		t.Fatal(err)
	}

	res, err := DetachSkills(onlyTarget(f.target), "gemini", f.source, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Removed, []string{"alpha"}) || !slices.Equal(res.Kept, []string{"local"}) {
		t.Errorf("res = %+v", res)
	}
	if !exists(filepath.Join(f.target, "alpha")) || !exists(filepath.Join(f.target, ManifestFile)) {
		t.Error("dry run must not write")
	}
}

func TestDetachSkills_MissingFolder(t *testing.T) {
	f := newSkillsOffFixture(t)
	res, err := DetachSkills(onlyTarget(f.target), "gemini", f.source, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Removed) != 0 || len(res.Kept) != 0 {
		t.Errorf("res = %+v", res)
	}
}

func disabledTarget(path string) config.TargetConfig {
	off := false
	return config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: path, Enabled: &off}}
}

func TestSyncWriters_SkipDisabledTarget(t *testing.T) {
	f := newSkillsOffFixture(t)
	tc := disabledTarget(f.target)
	skills, err := DiscoverSourceSkills(f.source)
	if err != nil {
		t.Fatal(err)
	}

	if err := SyncTarget("gemini", tc, f.source, false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncTargetMergeWithSkills("gemini", tc, skills, f.source, false, false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncTargetCopyWithSkillsOptions("gemini", tc, skills, f.source, false, false, nil, CopyOptions{}); err != nil {
		t.Fatal(err)
	}
	if exists(f.target) {
		t.Error("a target with skills off must not get a skills folder")
	}
}

func TestCleanMovedProjectDirs_DisabledTargetDoesNotSweep(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".skillshare", "skills")
	mustWrite(t, filepath.Join(source, "alpha", "SKILL.md"), "# alpha")
	// goose's old default .goose/skills is in its also_scans; a leftover link there
	// would be swept for an enabled goose.
	leftover := filepath.Join(root, ".goose", "skills", "alpha")
	mustLink(t, filepath.Join(source, "alpha"), leftover)

	targets := []MovedTarget{{Name: "goose", Path: filepath.Join(root, ".agents", "skills"), Disabled: true}}
	if msgs := CleanMovedProjectDirs(root, source, targets, false); len(msgs) != 0 {
		t.Errorf("messages = %v", msgs)
	}
	if !exists(leftover) {
		t.Error("disabled target must not start a legacy sweep")
	}
}
