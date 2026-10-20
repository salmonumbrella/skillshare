//go:build !online

package integration

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func projectFixture(t *testing.T) (*testutil.Sandbox, string) {
	t.Helper()
	sb := testutil.NewSandbox(t)
	t.Cleanup(sb.Cleanup)
	sb.SetEnv("XDG_CACHE_HOME", filepath.Join(sb.Home, ".cache"))
	root := filepath.Join(sb.Home, "app")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	return sb, root
}
func projectJSON(t *testing.T, r *testutil.Result) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &v); err != nil {
		t.Fatalf("invalid JSON (%v): stdout=%s stderr=%s", err, r.Stdout, r.Stderr)
	}
	if r.Stderr != "" {
		t.Fatalf("JSON stderr: %s", r.Stderr)
	}
	return v
}
func projectTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if e.IsDir() {
			tree[rel] = "dir"
			return nil
		}
		if e.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			tree[rel] = "link:" + link
			return err
		}
		b, err := os.ReadFile(path)
		tree[rel] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}
func TestProject_RoundTrip(t *testing.T) {
	sb, root := projectFixture(t)
	r := sb.RunCLI("project", "--json", "add", root, "--target", "claude,codex", "--name", "work", "--agents", "--skills-include", "one", "--skills-mode", "copy")
	r.AssertSuccess(t)
	v := projectJSON(t, r)
	if v["action"] != "add" || v["applied"] != true {
		t.Fatalf("add: %v", v)
	}
	p := v["project"].(map[string]any)
	if p["name"] != "work" || len(p["targets"].([]any)) != 2 {
		t.Fatalf("item: %v", p)
	}
	for _, token := range []string{"work", root, "~/app"} {
		r = sb.RunCLI("project", "show", token, "--json")
		r.AssertSuccess(t)
		if projectJSON(t, r)["project"].(map[string]any)["root"] != root {
			t.Fatal(r.Stdout)
		}
	}
	r = sb.RunCLI("project", "edit", "work", "--name=", "--skills-include=", "--no-agents", "--json")
	r.AssertSuccess(t)
	p = projectJSON(t, r)["project"].(map[string]any)
	if p["name"] != "app" || p["agents"] != nil || len(p["skills"].(map[string]any)["include"].([]any)) != 0 {
		t.Fatal(p)
	}
	r = sb.RunCLI("project", "list", "--json")
	r.AssertSuccess(t)
	if len(projectJSON(t, r)["projects"].([]any)) != 1 {
		t.Fatal(r.Stdout)
	}
	sb.RunCLI("project", "show", "app").AssertSuccess(t)
	before := projectTree(t, sb.Home)
	r = sb.RunCLI("project", "edit", "app", "--json")
	r.AssertSuccess(t)
	v = projectJSON(t, r)
	if v["applied"] != false || v["unchanged"] != true {
		t.Fatal(v)
	}
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("no-op changed files")
	}
	r = sb.RunCLI("project", "remove", "app", "--json", "--dry-run")
	r.AssertSuccess(t)
	if projectJSON(t, r)["applied"] != false {
		t.Fatal(r.Stdout)
	}
	r = sb.RunCLI("project", "remove", "app", "--json")
	r.AssertFailure(t)
	projectJSON(t, r)
	r = sb.RunCLI("project", "remove", "app", "--yes", "--json")
	r.AssertSuccess(t)
	if projectJSON(t, r)["registration"] != "removed" {
		t.Fatal(r.Stdout)
	}
	r = sb.RunCLI("project", "--json")
	r.AssertSuccess(t)
	if len(projectJSON(t, r)["projects"].([]any)) != 0 {
		t.Fatal(r.Stdout)
	}
}
func TestProject_GlobalModeAndParsing(t *testing.T) {
	for _, style := range []string{".skillshare", "skillshare"} {
		t.Run(style, func(t *testing.T) {
			sb, root := projectFixture(t)
			os.MkdirAll(filepath.Join(root, style), 0755)
			os.WriteFile(filepath.Join(root, style, "config.yaml"), []byte("targets: [claude]\n"), 0644)
			r := sb.RunCLIInDir(root, "project", "add", "--name", "-p", "--target=claude", "--json", "--", ".")
			r.AssertSuccess(t)
			if projectJSON(t, r)["project"].(map[string]any)["name"] != "-p" {
				t.Fatal(r.Stdout)
			}
			sb.RunCLIInDir(root, "project", "list", "-g").AssertSuccess(t)
			r = sb.RunCLIInDir(root, "project", "list", "-p", "--json")
			r.AssertFailure(t)
			if !strings.Contains(projectJSON(t, r)["error"].(string), "projects belong in the global config") {
				t.Fatal(r.Stdout)
			}
		})
	}
	for _, args := range [][]string{{"edit", "x", "--bad"}, {"show"}, {"add", "x", "--target"}, {"add", "x", "--skills", "--no-skills"}, {"wat"}, {"add", "~user", "--target", "claude"}} {
		sb, _ := projectFixture(t)
		r := sb.RunCLI(append([]string{"project", "--json"}, args...)...)
		r.AssertFailure(t)
		if projectJSON(t, r)["error"] == nil {
			t.Fatal(r.Stdout)
		}
	}
	sb, _ := projectFixture(t)
	os.WriteFile(sb.ConfigPath, []byte("[bad"), 0644)
	r := sb.RunCLI("project", "--json")
	r.AssertFailure(t)
	projectJSON(t, r)
}
func TestProject_Confirmation(t *testing.T) {
	for _, input := range []string{"", "n\n", "y\n"} {
		t.Run(strings.TrimSpace(input), func(t *testing.T) {
			sb, root := projectFixture(t)
			sb.RunCLI("project", "add", root, "--target", "claude", "--json").AssertSuccess(t)
			r := sb.RunCLIWithInput(input, "project", "remove", "app")
			r.AssertSuccess(t)
			r = sb.RunCLI("project", "list", "--json")
			r.AssertSuccess(t)
			count := len(projectJSON(t, r)["projects"].([]any))
			if (input == "y\n" && count != 0) || (input != "y\n" && count != 1) {
				t.Fatal(r.Stdout)
			}
		})
	}
}
func TestProject_ReadOnlyLegacyWholeInvocation(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "skillshare")
	cmd := exec.Command("go", "build", "-ldflags", "-X main.version=1.0.0", "-o", binary, "./cmd/skillshare")
	cmd.Dir = findProjectRoot(t)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, b)
	}
	for _, args := range [][]string{{"list"}, {"show", "app"}, {"edit", "app", "--name", "preview", "--dry-run"}, {"list", "--json"}, {"edit", "app", "--json"}, {"edit", "app"}, {"edit", "app", "--skills-mode", "bad"}, {"edit", "app", "--skills-mode", "bad", "--json"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			sb, root := projectFixture(t)
			sb.BinaryPath = binary
			os.RemoveAll(filepath.Join(sb.Home, ".local"))
			for _, dir := range []string{"backups", "trash", "logs"} {
				p := filepath.Join(filepath.Dir(sb.ConfigPath), dir)
				os.MkdirAll(p, 0755)
				os.WriteFile(filepath.Join(p, "old"), []byte("keep"), 0644)
			}
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nskills:\n  - name: legacy\n    source: github.com/a/b\nprojects:\n  " + root + ":\n    targets: [claude]\n    skills: {}\n")
			before := projectTree(t, sb.Home)
			r := sb.RunCLI(append([]string{"project"}, args...)...)
			if args[0] == "edit" && strings.Contains(strings.Join(args, " "), "bad") {
				r.AssertFailure(t)
			} else {
				r.AssertSuccess(t)
			}
			if strings.Contains(strings.Join(args, " "), "--json") {
				projectJSON(t, r)
			}
			if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
				t.Fatalf("read-only changed filesystem: %s", r.Output())
			}
		})
	}
}
func TestProject_LegacyMutationAndAnchors(t *testing.T) {
	sb, root := projectFixture(t)
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nskills:\n  - name: legacy\n    source: github.com/a/b\n    custom: keep\nprojects:\n  " + root + ": &shared\n    targets: [claude]\n    skills: {}\n  ~/other: *shared\n")
	r := sb.RunCLI("project", "edit", "app", "--skills-mode", "copy", "--json")
	r.AssertSuccess(t)
	if len(projectJSON(t, r)["affected"].([]any)) != 2 {
		t.Fatal(r.Stdout)
	}
	b, _ := os.ReadFile(sb.ConfigPath)
	if !strings.Contains(string(b), "*shared") || !strings.Contains(string(b), "custom: keep") {
		t.Fatal(string(b))
	}
	extra := filepath.Join(sb.Home, "extra")
	os.MkdirAll(extra, 0755)
	r = sb.RunCLI("project", "add", extra, "--target", "claude", "--json")
	r.AssertSuccess(t)
	projectJSON(t, r)
	b, _ = os.ReadFile(sb.ConfigPath)
	if !strings.Contains(string(b), "*shared") || !strings.Contains(string(b), "custom: keep") {
		t.Fatal(string(b))
	}
	before := projectTree(t, sb.Home)
	r = sb.RunCLI("project", "edit", "app", "--name", "duplicate", "--json")
	r.AssertFailure(t)
	projectJSON(t, r)
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("invalid alias candidate changed files")
	}
}

func projectLogs(t *testing.T, sb *testutil.Sandbox) []map[string]any {
	t.Helper()
	path := filepath.Join(sb.Home, ".local", "state", "skillshare", "logs", "operations.log")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	return entries
}
func TestProject_SyncScopePreviewAndRetry(t *testing.T) {
	sb, root := projectFixture(t)
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
	other := filepath.Join(sb.Home, "other")
	os.MkdirAll(other, 0755)
	sb.RunCLI("project", "add", other, "--target", "claude", "--json").AssertSuccess(t)
	// A blocked second skills parent makes the saved entry a partial sync.
	os.WriteFile(filepath.Join(root, ".agents"), []byte("blocked"), 0644)
	r := sb.RunCLI("project", "add", root, "--target", "claude,codex", "--sync", "--json")
	r.AssertFailure(t)
	v := projectJSON(t, r)
	if v["applied"] != true || v["project"] == nil || v["sync"] == nil {
		t.Fatal(v)
	}
	sync := v["sync"].(map[string]any)
	if len(sync["failed"].([]any)) == 0 || len(sync["results"].([]any)) == 0 {
		t.Fatal(sync)
	}
	for _, key := range []string{"warnings", "unmatched", "folder_conflicts", "path_overlap", "ignored_count", "ignored_skills", "ignore_root", "ignore_repos"} {
		if _, ok := sync[key]; !ok {
			t.Fatalf("missing %s: %v", key, sync)
		}
	}
	if sync["Skills"] != nil || sync["Results"] != nil {
		t.Fatal("internal fields leaked")
	}
	if _, err := os.Lstat(filepath.Join(other, ".claude", "skills", "alpha")); !os.IsNotExist(err) {
		t.Fatal("other project synced")
	}
	logs := projectLogs(t, sb)
	if len(logs) != 3 || logs[1]["cmd"] != "project" || logs[2]["cmd"] != "sync" || logs[2]["status"] != "error" {
		t.Fatal(logs)
	}
	os.Remove(filepath.Join(root, ".agents"))
	before := projectTree(t, sb.Home)
	r = sb.RunCLI("project", "edit", "app", "--name", "renamed", "--target", "cursor", "--sync", "--dry-run", "--json")
	r.AssertSuccess(t)
	v = projectJSON(t, r)
	rows := v["sync"].(map[string]any)["results"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["target"] != "renamed@cursor" {
		t.Fatal(v)
	}
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("sync preview wrote")
	}
	r = sb.RunCLI("project", "sync", "app", "--json")
	r.AssertSuccess(t)
	v = projectJSON(t, r)
	if v["action"] != "sync" || v["applied"] != nil || len(v["sync"].(map[string]any)["failed"].([]any)) != 0 {
		t.Fatal(v)
	}
	if _, err := os.Lstat(filepath.Join(root, ".agents", "skills", "alpha")); err != nil {
		t.Fatal(err)
	}
	if len(projectLogs(t, sb)) != 4 {
		t.Fatal(projectLogs(t, sb))
	}
	sb.CreateSkill("beta", map[string]string{"SKILL.md": "---\nname: beta\n---\n# Beta\n"})
	before = projectTree(t, sb.Home)
	r = sb.RunCLI("project", "sync", "app", "--dry-run", "--json")
	r.AssertSuccess(t)
	projectJSON(t, r)
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("standalone sync preview wrote")
	}
}
func TestProject_RemoveDetachPartialAndWarnings(t *testing.T) {
	sb, root := projectFixture(t)
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
	r := sb.RunCLI("project", "add", root, "--target", "claude,codex", "--sync", "--json")
	r.AssertSuccess(t)
	v := projectJSON(t, r)
	groups := v["project"].(map[string]any)["groups"].([]any)
	var second string
	for _, g := range groups {
		group := g.(map[string]any)
		if group["target"] == "app@codex" {
			second = group["skillsPath"].(string)
		}
	}
	if second == "" {
		t.Fatal(groups)
	}
	// A nonempty directory where the second manifest belongs produces a real
	// later-target detach failure without relying on root-sensitive permissions.
	manifest := filepath.Join(second, ".skillshare-manifest.json")
	os.Remove(manifest)
	os.MkdirAll(manifest, 0755)
	os.WriteFile(filepath.Join(manifest, "blocker"), []byte("x"), 0644)
	before := projectTree(t, sb.Home)
	r = sb.RunCLI("project", "remove", "app", "--dry-run", "--json")
	r.AssertSuccess(t)
	v = projectJSON(t, r)
	if v["registration"] != "kept" || v["applied"] != false || v["detach"] == nil {
		t.Fatal(v)
	}
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("detach preview wrote")
	}
	r = sb.RunCLI("project", "remove", "app", "--yes", "--json")
	r.AssertFailure(t)
	v = projectJSON(t, r)
	if v["registration"] != "kept" || v["detach"] == nil || v["error"] == nil {
		t.Fatal(v)
	}
	if len(v["detach"].(map[string]any)["removed"].([]any)) == 0 {
		t.Fatal(v)
	}
	logs := projectLogs(t, sb)
	if len(logs) != 3 || logs[2]["status"] != "partial" || logs[2]["cmd"] != "project" {
		t.Fatal(logs)
	}
	info, err := os.Lstat(filepath.Join(root, ".claude", "skills", "alpha"))
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("first detach did not preserve copy: %v %v", info, err)
	}
	os.Remove(filepath.Join(manifest, "blocker"))
	os.Remove(manifest)
	// Out-of-scope registrations are retained and explicitly warned about.
	b, _ := os.ReadFile(sb.ConfigPath)
	os.WriteFile(sb.ConfigPath, append(b, []byte("\nmcp:\n  projects:\n    "+root+"/: {servers: {}}\nhooks:\n  projects:\n    "+root+"/.: {entries: {}}\n")...), 0644)
	r = sb.RunCLI("project", "remove", "app", "--yes", "--json")
	r.AssertSuccess(t)
	v = projectJSON(t, r)
	if v["registration"] != "removed" || len(v["warnings"].([]any)) != 2 {
		t.Fatal(v)
	}
}
func TestProject_AmbiguityAndValidation(t *testing.T) {
	sb, root := projectFixture(t)
	collision := filepath.Join(sb.Home, "named")
	os.MkdirAll(collision, 0755)
	sb.RunCLI("project", "add", root, "--name", "named", "--target", "claude", "--json").AssertSuccess(t)
	sb.RunCLI("project", "add", collision, "--name", "other", "--target", "claude", "--json").AssertSuccess(t)
	r := sb.RunCLIInDir(sb.Home, "project", "show", "named", "--json")
	r.AssertFailure(t)
	v := projectJSON(t, r)
	if !strings.Contains(v["error"].(string), root) || !strings.Contains(v["error"].(string), collision) {
		t.Fatal(v)
	}
	sb.RunCLIInDir(sb.Home, "project", "show", "./named", "--json").AssertSuccess(t)
	os.MkdirAll(filepath.Join(sb.Home, "other-empty"), 0755)
	before := projectTree(t, sb.Home)
	for _, args := range [][]string{
		{"add", "~/app", "--target", "claude"},
		{"add", filepath.Join(sb.Home, "other-empty"), "--target="},
		{"add", filepath.Join(sb.Home, "missing"), "--target", "claude"},
		{"edit", root, "--target", "nonesuch"},
		{"edit", root, "--skills-mode", "bad"},
		{"edit", root, "--skills-naming", "bad"},
		{"edit", root, "--skills-include", "["},
		{"edit", root, "--agents-mode", "symlink", "--agents-include", "*"},
		{"edit", root, "--skills", "--no-skills"},
	} {
		r := sb.RunCLI(append(append([]string{"project"}, args...), "--json")...)
		r.AssertFailure(t)
		projectJSON(t, r)
	}
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("validation wrote")
	}
}

func TestProject_MissingRegisteredFolderAndStoredName(t *testing.T) {
	sb, root := projectFixture(t)
	// Absolute spelling with an internal '..' stays as typed. The displayed
	// default name must also be a supported lookup token from another cwd.
	os.MkdirAll(filepath.Join(root, "child"), 0755)
	spelling := root + string(filepath.Separator) + "child" + string(filepath.Separator) + ".."
	r := sb.RunCLI("project", "add", spelling, "--target", "claude", "--json")
	r.AssertSuccess(t)
	item := projectJSON(t, r)["project"].(map[string]any)
	name := item["name"].(string)
	r = sb.RunCLIInDir(sb.Home, "project", "show", name, "--json")
	r.AssertSuccess(t)
	if projectJSON(t, r)["project"].(map[string]any)["root"] != spelling {
		t.Fatal(r.Stdout)
	}
	os.Rename(root, root+"-moved")
	r = sb.RunCLI("project", "edit", spelling, "--name", "missing", "--json")
	r.AssertSuccess(t)
	r = sb.RunCLI("project", "edit", "missing", "--name", "still-missing", "--sync", "--json")
	r.AssertFailure(t)
	v := projectJSON(t, r)
	if v["applied"] != true || v["project"].(map[string]any)["name"] != "still-missing" || !strings.Contains(v["error"].(string), "folder not found") {
		t.Fatal(v)
	}
}
func TestProject_SyncAgentIgnoreStatistics(t *testing.T) {
	sb, root := projectFixture(t)
	agents := filepath.Join(sb.Home, "agents")
	os.MkdirAll(agents, 0755)
	os.WriteFile(filepath.Join(agents, "keep.md"), []byte("# Keep\n"), 0644)
	os.WriteFile(filepath.Join(agents, "ignored.md"), []byte("# Ignored\n"), 0644)
	ignore := filepath.Join(agents, ".agentignore")
	os.WriteFile(ignore, []byte("ignored.md\n"), 0644)
	sb.WriteConfig("source: " + sb.SourcePath + "\nagents_source: " + agents + "\ntargets: {}\n")
	r := sb.RunCLI("project", "add", root, "--target", "claude", "--no-skills", "--agents", "--sync", "--json")
	r.AssertSuccess(t)
	sync := projectJSON(t, r)["sync"].(map[string]any)
	if sync["agent_ignore_root"] != ignore || sync["agent_ignored_count"] != float64(1) {
		t.Fatalf("agent statistics missing: %v", sync)
	}
	ignored, ok := sync["agent_ignored_skills"].([]any)
	if !ok || len(ignored) != 1 || ignored[0] != "ignored.md" {
		t.Fatal(sync)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude", "agents", "keep.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude", "agents", "ignored.md")); !os.IsNotExist(err) {
		t.Fatal("ignored agent synced")
	}
}

func TestProject_GlobalOplogWithConfigOverride(t *testing.T) {
	sb, root := projectFixture(t)
	sb.ConfigPath = filepath.Join(sb.Home, "shared", "skillshare", "config.yaml")
	os.MkdirAll(filepath.Dir(sb.ConfigPath), 0755)
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	r := sb.RunCLI("project", "add", root, "--target", "claude", "--json")
	r.AssertSuccess(t)
	projectJSON(t, r)
	if len(projectLogs(t, sb)) != 1 {
		t.Fatal("global project operation not logged to global state")
	}
	read := sb.RunCLI("log", "-g", "--cmd", "project", "--json")
	read.AssertSuccess(t)
	if !strings.Contains(read.Stdout, `"cmd":"project"`) {
		t.Fatalf("global log reader missed project add: %s", read.Output())
	}
	sb.RunCLI("log", "-g", "--stats").AssertOutputContains(t, "project")
	sb.RunCLI("log", "-g", "--clear").AssertSuccess(t)
	cleared := sb.RunCLI("log", "-g", "--cmd", "project", "--json")
	cleared.AssertSuccess(t)
	if cleared.Stdout != "" {
		t.Fatalf("global log clear missed project add: %s", cleared.Stdout)
	}
	raw, err := os.ReadFile(filepath.Join(sb.Home, ".local", "state", "skillshare", "logs", "operations.log"))
	if err != nil || len(raw) != 0 {
		t.Fatalf("canonical log not cleared: %q %v", raw, err)
	}
	for _, name := range []string{"logs", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(sb.ConfigPath), name)); !os.IsNotExist(err) {
			t.Fatalf("global config treated as a project: %s (%v)", name, err)
		}
	}
}

func TestProject_TextDefaultsAndRelativeKey(t *testing.T) {
	sb, _ := projectFixture(t)
	r := sb.RunCLIInDir(sb.Home, "project", "add", "app", "--target", "claude", "--target", "codex")
	r.AssertSuccess(t)
	r.AssertRowContains(t, "Project", "app")
	r.AssertRowContains(t, "Root", "~/app")
	r = sb.RunCLI("project", "edit", "app", "--name", "text", "--agents", "--skills-exclude", "skip")
	r.AssertSuccess(t)
	r.AssertRowContains(t, "Project", "text")
	before := projectTree(t, sb.Home)
	r = sb.RunCLI("project", "sync", "text", "--dry-run")
	r.AssertSuccess(t)
	r.AssertOutputContains(t, "Dry run")
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("text sync preview wrote")
	}
	r = sb.RunCLI("project", "remove", "text", "--yes")
	r.AssertSuccess(t)
	r.AssertRowContains(t, "Registration", "removed")
	sb.RunCLI("project").AssertOutputContains(t, "No projects registered")
	logs := projectLogs(t, sb)
	if len(logs) != 3 || logs[0]["args"].(map[string]any)["action"] != "add" || logs[1]["args"].(map[string]any)["action"] != "update" || logs[2]["args"].(map[string]any)["action"] != "remove" {
		t.Fatal(logs)
	}
	for _, entry := range logs {
		if entry["cmd"] != "project" || entry["args"].(map[string]any)["scope"] != "cli" {
			t.Fatal(logs)
		}
	}
}

func TestProject_TextUnmatchedSync(t *testing.T) {
	sb, root := projectFixture(t)
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
	for _, args := range [][]string{
		{"project", "add", root, "--target", "claude", "--skills-include", "typo", "--sync"},
		{"project", "sync", "app", "--dry-run"},
		{"project", "edit", "app", "--skills-include", "other-typo", "--sync", "--dry-run"},
	} {
		r := sb.RunCLI(args...)
		r.AssertSuccess(t)
		r.AssertOutputContains(t, "matches no skill in the source")
	}
}
