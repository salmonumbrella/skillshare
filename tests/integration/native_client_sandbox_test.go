//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"skillshare/internal/testutil"
)

// Inherited native-client overrides must never escape either fixture home.
func TestNativeClientSentinelUntouched(t *testing.T) {
	keys := []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME", "COPILOT_HOME", "OPENCODE_CONFIG_DIR", "PI_CODING_AGENT_DIR"}
	sentinel := t.TempDir()
	before := map[string]os.FileInfo{}
	for _, key := range keys {
		dir := filepath.Join(sentinel, key)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(dir, "sentinel")
		if err := os.WriteFile(file, []byte("fixture only"), 0600); err != nil {
			t.Fatal(err)
		}
		before[key], _ = os.Stat(file)
		t.Setenv(key, dir)
	}
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	// A second home proves construction never retains the previous sandbox's overrides.
	other := testutil.NewSandbox(t)
	for _, key := range keys {
		rel, err := filepath.Rel(other.Home, os.Getenv(key))
		if err != nil || rel == ".." || filepath.IsAbs(rel) || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
			t.Fatalf("%s escaped second home", key)
		}
	}
	other.Cleanup()
	for _, key := range keys {
		rel, err := filepath.Rel(sb.Home, os.Getenv(key))
		if err != nil || rel == ".." || filepath.IsAbs(rel) {
			t.Fatalf("%s escaped first home", key)
		}
	}
	sb.CreateSkill("fixture", map[string]string{"SKILL.md": "# Fixture"})
	source := filepath.Join(sb.Root, "rules")
	sb.WriteFile(filepath.Join(source, "fixture.md"), "fixture rule")
	target := filepath.Join(sb.Home, "rules")
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nextras:\n  - name: rules\n    source: " + source + "\n    targets:\n      - path: " + target + "\n")
	sb.RunCLI("sync", "--all", "-g").AssertSuccess(t)
	if !sb.FileExists(filepath.Join(target, "fixture.md")) {
		t.Fatal("extra was not applied")
	}
	for _, key := range keys {
		entries, err := os.ReadDir(filepath.Join(sentinel, key))
		if err != nil || len(entries) != 1 || entries[0].Name() != "sentinel" {
			t.Fatalf("%s sentinel changed", key)
		}
		after, err := os.Stat(filepath.Join(sentinel, key, "sentinel"))
		if err != nil || !reflect.DeepEqual(before[key].ModTime(), after.ModTime()) || after.Size() != before[key].Size() {
			t.Fatalf("%s sentinel modified", key)
		}
	}
}
