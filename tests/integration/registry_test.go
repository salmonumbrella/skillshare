//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// TestRegistryMigration_OldToNew verifies that registry.yaml is moved from the
// config directory to the source directory on first load after upgrading.
func TestRegistryMigration_OldToNew(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# My Skill",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Place registry.yaml in old location (config dir)
	configDir := filepath.Dir(sb.ConfigPath)
	oldRegistry := filepath.Join(configDir, "registry.yaml")
	registryContent := `skills:
  - name: my-skill
    source: github.com/example/repo
`
	if err := os.WriteFile(oldRegistry, []byte(registryContent), 0644); err != nil {
		t.Fatalf("failed to write old registry: %v", err)
	}

	// Run list to trigger config.Load() which calls MigrateRegistryToSource
	result := sb.RunCLI("list")
	result.AssertSuccess(t)

	// Old location should be removed
	if sb.FileExists(oldRegistry) {
		t.Errorf("registry.yaml should have been removed from old config dir: %s", oldRegistry)
	}

	// New location should exist in source dir
	newRegistry := filepath.Join(sb.SourcePath, "registry.yaml")
	if !sb.FileExists(newRegistry) {
		t.Errorf("registry.yaml should exist in source dir: %s", newRegistry)
	}
}

// TestRegistryMigration_BothExist verifies that when registry.yaml exists in both
// locations, the old file is kept (not deleted) and the new file is preserved as-is.
func TestRegistryMigration_BothExist(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("old-skill", map[string]string{
		"SKILL.md": "---\nname: old-skill\n---\n# Old Skill",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	configDir := filepath.Dir(sb.ConfigPath)
	oldRegistry := filepath.Join(configDir, "registry.yaml")
	newRegistry := filepath.Join(sb.SourcePath, "registry.yaml")

	oldContent := `skills:
  - name: old-skill
    source: github.com/example/old-repo
`
	newContent := `skills:
  - name: new-skill
    source: github.com/example/new-repo
`

	if err := os.WriteFile(oldRegistry, []byte(oldContent), 0644); err != nil {
		t.Fatalf("failed to write old registry: %v", err)
	}
	if err := os.WriteFile(newRegistry, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to write new registry: %v", err)
	}

	// Run list to trigger migration logic
	result := sb.RunCLI("list")
	result.AssertSuccess(t)

	// Old file should still exist (not deleted when both present)
	if !sb.FileExists(oldRegistry) {
		t.Errorf("old registry.yaml should NOT be deleted when both exist: %s", oldRegistry)
	}

	// New file content should be unchanged
	gotContent := sb.ReadFile(newRegistry)
	if gotContent != newContent {
		t.Errorf("new registry.yaml should be unchanged\nwant: %s\ngot:  %s", newContent, gotContent)
	}

	// Warning should appear in stderr
	result.AssertAnyOutputContains(t, "registry.yaml")
}

// TestRegistryDir_GlobalMode confirms that a fresh global-mode sandbox does not
// place registry.yaml in the config directory — it belongs in the source dir.
func TestRegistryDir_GlobalMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("sample-skill", map[string]string{
		"SKILL.md": "---\nname: sample-skill\n---\n# Sample",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Run list to trigger normal config.Load() path
	result := sb.RunCLI("list")
	result.AssertSuccess(t)

	// registry.yaml must NOT be in the config directory
	configDir := filepath.Dir(sb.ConfigPath)
	registryInConfigDir := filepath.Join(configDir, "registry.yaml")
	if sb.FileExists(registryInConfigDir) {
		t.Errorf("registry.yaml should NOT exist in config dir %s", registryInConfigDir)
	}
}
