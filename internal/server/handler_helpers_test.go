package server

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

// newTestServer creates an isolated Server for handler testing.
// It sets up a temp source directory and config file, returning the server
// and the source directory path for test setup.
func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "skills")
	os.MkdirAll(sourceDir, 0755)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg-config"))

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	os.MkdirAll(filepath.Dir(cfgPath), 0755)

	raw := "source: " + sourceDir + "\nmode: merge\ntargets: {}\n"
	os.WriteFile(cfgPath, []byte(raw), 0644)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	s := New(cfg, "127.0.0.1:0", "", "")
	return s, sourceDir
}

// newTestServerWithTargets creates a test server with pre-configured targets.
func newTestServerWithTargets(t *testing.T, targets map[string]string) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "skills")
	os.MkdirAll(sourceDir, 0755)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg-config"))

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	os.MkdirAll(filepath.Dir(cfgPath), 0755)

	raw := "source: " + sourceDir + "\nmode: merge\ntargets:\n"
	for name, path := range targets {
		os.MkdirAll(path, 0755)
		raw += "  " + name + ":\n    path: " + path + "\n"
	}
	os.WriteFile(cfgPath, []byte(raw), 0644)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	s := New(cfg, "127.0.0.1:0", "", "")
	return s, sourceDir
}

// newManagedProjectServer creates a project-mode server with one configured target.
func newManagedProjectServer(t *testing.T, targetName string) (*Server, string, string, string) {
	t.Helper()

	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")
	sourceDir := filepath.Join(tmp, "source")
	targetPath := filepath.Join(tmp, "targets", targetName)

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))

	if err := os.MkdirAll(filepath.Join(projectRoot, ".skillshare"), 0755); err != nil {
		t.Fatalf("failed to create project config dir: %v", err)
	}
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}

	projectCfgPath := filepath.Join(projectRoot, ".skillshare", "config.yaml")
	raw := "targets:\n- name: " + targetName + "\n  path: " + targetPath + "\n"
	if err := os.WriteFile(projectCfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	projectCfg, err := config.LoadProject(projectRoot)
	if err != nil {
		t.Fatalf("failed to load project config: %v", err)
	}

	targets, err := config.ResolveProjectTargets(projectRoot, projectCfg)
	if err != nil {
		t.Fatalf("failed to resolve project targets: %v", err)
	}

	cfg := &config.Config{
		Source:  sourceDir,
		Targets: targets,
	}
	s := NewProject(cfg, projectCfg, projectRoot, "127.0.0.1:0", "", "")
	return s, projectRoot, sourceDir, targetPath
}

// addSkill creates a skill directory with SKILL.md in the source directory.
func addSkill(t *testing.T, sourceDir, name string) {
	t.Helper()
	skillDir := filepath.Join(sourceDir, name)
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n# "+name), 0644)
}

func addTrackedRepo(t *testing.T, sourceDir, relPath string) {
	t.Helper()
	repoDir := filepath.Join(sourceDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatalf("failed to create tracked repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("tracked repo"), 0644); err != nil {
		t.Fatalf("failed to seed tracked repo: %v", err)
	}
}

// addSkillMeta creates a .skillshare-meta.json for a skill (marks it as remotely installed).
func addSkillMeta(t *testing.T, sourceDir, name, source string) {
	t.Helper()
	meta := `{"source":"` + source + `"}`
	os.WriteFile(filepath.Join(sourceDir, name, ".skillshare-meta.json"), []byte(meta), 0644)
}
