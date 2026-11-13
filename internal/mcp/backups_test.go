package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupsReportServersAndTime(t *testing.T) {
	s := testService(t)
	if _, err := s.Apply(""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate(Mutation{Name: "docs", Remove: true}, "", true); err != nil {
		t.Fatal(err)
	}
	backups, err := s.Backups()
	if err != nil || len(backups) == 0 {
		t.Fatalf("backups = %v, %v", backups, err)
	}
	changes := map[string]bool{}
	for _, b := range backups {
		if _, err := time.Parse(time.RFC3339, b.Time); err != nil {
			t.Fatalf("time %q: %v", b.Time, err)
		}
		if len(b.Servers) != 1 || b.Servers[0].Name != "docs" {
			t.Fatalf("servers = %+v", b.Servers)
		}
		changes[b.Servers[0].Change] = true
	}
	if !changes["added"] || !changes["removed"] {
		t.Fatalf("changes = %v, want added and removed", changes)
	}
}

func TestBackupsOfProjectConfigStayInProjectScope(t *testing.T) {
	global := testService(t)
	root := t.TempDir()
	projectConfig := filepath.Join(root, ".skillshare", "config.yaml")
	os.MkdirAll(filepath.Dir(projectConfig), 0755)
	if err := os.WriteFile(projectConfig, []byte("mcp:\n  targets: [cursor]\n  servers:\n    local:\n      command: tool\n"), 0600); err != nil {
		t.Fatal(err)
	}
	project := &Service{ConfigPath: projectConfig, ProjectRoot: root, Home: global.Home, StateDir: global.StateDir, Platform: "linux"}
	if _, err := project.Apply(""); err != nil {
		t.Fatal(err)
	}
	backups, err := project.Backups()
	if err != nil || len(backups) != 1 || backups[0].Servers[0].Name != "local" {
		t.Fatalf("project backups = %+v, %v", backups, err)
	}
	if backups, _ := global.Backups(); len(backups) != 0 {
		t.Fatalf("global scope lists project backups: %+v", backups)
	}
}
