package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/instructions"
	ssync "skillshare/internal/sync"
)

// newInstructionsServer creates a global-mode server whose HOME is a temp dir
// holding the given built-in targets.
func newInstructionsServer(t *testing.T, targets ...string) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	sourceDir := filepath.Join(tmp, "skills")
	os.MkdirAll(sourceDir, 0755)
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	os.MkdirAll(filepath.Dir(cfgPath), 0755)

	raw := "source: " + sourceDir + "\nmode: merge\ntargets:\n"
	for _, name := range targets {
		dir := filepath.Join(tmp, "skills-"+name)
		os.MkdirAll(dir, 0755)
		raw += "  " + name + ":\n    path: " + dir + "\n"
	}
	os.WriteFile(cfgPath, []byte(raw), 0644)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return New(cfg, "127.0.0.1:0", "", ""), home
}

func instructionsRequest(t *testing.T, s *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	return rr
}

func decodeBody[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rr.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rr.Body.String(), err)
	}
	return v
}

func writeHome(t *testing.T, home, rel, content string) string {
	t.Helper()
	path := filepath.Join(home, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestTargetInstructions_GetClaude(t *testing.T) {
	s, home := newInstructionsServer(t, "claude")
	writeHome(t, home, ".claude/CLAUDE.md", "# Me\n@~/.claude/rules/a.md\n")

	rr := instructionsRequest(t, s, http.MethodGet, "/api/targets/claude/instructions", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	got := decodeBody[instructionsFileResponse](t, rr)
	if !got.Supported || !got.Exists || !got.Import || len(got.ImportLines) != 1 || got.ImportLines[0] != 2 {
		t.Errorf("response = %+v", got)
	}
	if strings.Join(got.Convert, ",") != "import,copy" || got.Blocked["rename"] != "global" {
		t.Errorf("convert = %v, blocked = %v", got.Convert, got.Blocked)
	}
}

func TestTargetInstructions_CursorHasNoGlobalFile(t *testing.T) {
	s, _ := newInstructionsServer(t, "cursor")
	got := decodeBody[instructionsFileResponse](t, instructionsRequest(t, s, http.MethodGet, "/api/targets/cursor/instructions", ""))
	if got.Supported {
		t.Errorf("cursor supported = true, want false")
	}
}

func TestTargetInstructions_PutWritesFile(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/codex/instructions", `{"content":"hello\n"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if got := readFile(t, filepath.Join(home, ".codex", "AGENTS.md")); got != "hello\n" {
		t.Errorf("AGENTS.md = %q", got)
	}
}

func TestTargetInstructions_ConvertShareAs(t *testing.T) {
	s, home := newInstructionsServer(t, "claude")
	claude := writeHome(t, home, ".claude/CLAUDE.md", "# Me\nBe brief.\n@~/.claude/rules/a.md\n")

	preview := instructionsRequest(t, s, http.MethodPost, "/api/targets/claude/instructions/convert", `{"method":"import","keep_tool_lines":true,"share_as":"personal"}`)
	if preview.Code != http.StatusOK || readFile(t, claude) != "# Me\nBe brief.\n@~/.claude/rules/a.md\n" {
		t.Fatalf("preview changed files or failed: %d %s", preview.Code, preview.Body.String())
	}
	rr := instructionsRequest(t, s, http.MethodPost, "/api/targets/claude/instructions/convert", `{"method":"import","keep_tool_lines":true,"share_as":"personal","apply":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("apply status %d: %s", rr.Code, rr.Body.String())
	}
	list := decodeBody[struct {
		Files   []sharedInstructionsFile   `json:"files"`
		Targets []sharedInstructionsTarget `json:"targets"`
	}](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions", ""))
	if len(list.Files) != 1 || readFile(t, list.Files[0].Path) != "# Me\nBe brief.\n" {
		t.Fatalf("files = %+v", list.Files)
	}
	if a := list.Targets[0].Assigned; len(a) != 1 || a[0].Name != "personal" || a[0].Status != "synced" {
		t.Errorf("claude assigned = %+v", a)
	}
	if got := readFile(t, claude); !strings.Contains(got, "@~/.claude/rules/a.md") || strings.Contains(got, "Be brief.") {
		t.Errorf("CLAUDE.md = %q", got)
	}
}

func TestTargetInstructions_ConvertShareIntoAppends(t *testing.T) {
	s, home := newInstructionsServer(t, "claude")
	claude := writeHome(t, home, ".claude/CLAUDE.md", "Be brief.\n@~/.claude/rules/a.md\n")
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"team","content":"# Team\n"}`); rr.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	req := `{"method":"import","keep_tool_lines":true,"share_into":"team"`

	preview := decodeBody[struct {
		Changes []instructions.Change `json:"changes"`
	}](t, instructionsRequest(t, s, http.MethodPost, "/api/targets/claude/instructions/convert", req+`}`))
	if len(preview.Changes) != 2 || preview.Changes[0].Status != "modified" || preview.Changes[0].Before != "# Team\n" || preview.Changes[0].After != "# Team\n\nBe brief.\n" {
		t.Fatalf("preview = %+v", preview.Changes)
	}
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/targets/claude/instructions/convert", req+`,"apply":true}`); rr.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rr.Code, rr.Body.String())
	}

	list := decodeBody[struct {
		Files   []sharedInstructionsFile   `json:"files"`
		Targets []sharedInstructionsTarget `json:"targets"`
	}](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions", ""))
	sharedPath := list.Files[0].Path
	if got := readFile(t, sharedPath); got != "# Team\n\nBe brief.\n" {
		t.Errorf("shared file = %q", got)
	}
	if a := list.Targets[0].Assigned; len(a) != 1 || a[0].Name != "team" || a[0].Mode != "import" || a[0].Status != "synced" {
		t.Errorf("claude assigned = %+v", a)
	}
	if got := readFile(t, claude); strings.Count(got, "@"+sharedPath) != 1 || strings.Contains(got, "Be brief.") {
		t.Errorf("CLAUDE.md = %q", got)
	}
	if !hasBackup(t, sharedPath, "# Team\n") {
		t.Error("shared file was not backed up before appending")
	}
	// Attached now, so a second convert into it is refused.
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/targets/claude/instructions/convert", req+`}`); rr.Code == http.StatusOK {
		t.Errorf("second convert into team = %d, want an error", rr.Code)
	}
}

// hasBackup reports whether the extras backup store holds content for path.
func hasBackup(t *testing.T, path, content string) bool {
	t.Helper()
	found := false
	filepath.WalkDir(os.Getenv("XDG_STATE_HOME"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".bak" {
			return nil
		}
		orig, _ := os.ReadFile(filepath.Join(filepath.Dir(p), "path"))
		data, _ := os.ReadFile(p)
		if string(orig) == filepath.Clean(path) && string(data) == content {
			found = true
		}
		return nil
	})
	return found
}

func TestSharedInstructions_CreateFromTargetMovesContent(t *testing.T) {
	s, home := newInstructionsServer(t, "claude")
	claude := writeHome(t, home, ".claude/CLAUDE.md", "# Me\nBe brief.\n@~/.claude/rules/a.md\n")

	if rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","from_target":"claude"}`); rr.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	// Attaching claude afterwards, as the UI allows, must not add it twice.
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude"],"extras":["personal"]}`)

	list := decodeBody[struct {
		Files   []sharedInstructionsFile   `json:"files"`
		Targets []sharedInstructionsTarget `json:"targets"`
	}](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions", ""))
	if len(list.Files) != 1 || readFile(t, list.Files[0].Path) != "# Me\nBe brief.\n" {
		t.Fatalf("shared file = %+v", list.Files)
	}
	if a := list.Targets[0].Assigned; len(a) != 1 || a[0].Mode != "import" || a[0].Status != "synced" {
		t.Errorf("claude assigned = %+v", a)
	}
	got := readFile(t, claude)
	if strings.Contains(got, "Be brief.") || strings.Count(got, "@"+list.Files[0].Path) != 1 || !strings.Contains(got, "@~/.claude/rules/a.md") {
		t.Errorf("CLAUDE.md = %q, want only its tool line and one import", got)
	}
}

func TestSharedInstructions_AssignAndRestore(t *testing.T) {
	s, home := newInstructionsServer(t, "claude", "codex")
	codex := writeHome(t, home, ".codex/AGENTS.md", "codex own\n")
	for _, name := range []string{"personal", "work"} {
		if rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"`+name+`","content":"`+name+`\n"}`); rr.Code != http.StatusOK {
			t.Fatalf("create %s: %d %s", name, rr.Code, rr.Body.String())
		}
	}

	rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["personal","work"]}`)
	if res := decodeBody[map[string]any](t, rr); rr.Code != http.StatusConflict || res["error_code"] != "instructions_target_held" {
		t.Errorf("codex with two files: %v, want conflict", res)
	}
	rr = instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude","codex"],"extras":["personal"]}`)
	if res := decodeBody[map[string]any](t, rr); res["success"] != true {
		t.Fatalf("assign: %v", res)
	}
	if readFile(t, codex) != "personal\n" {
		t.Errorf("codex AGENTS.md = %q, want the shared file", readFile(t, codex))
	}
	rr = instructionsRequest(t, s, http.MethodPost, "/api/instructions/personal/restore", `{"target":"codex"}`)
	if res := decodeBody[map[string]any](t, rr); res["success"] != true || readFile(t, codex) != "codex own\n" {
		t.Errorf("restore: %v, codex = %q", res, readFile(t, codex))
	}
}

func TestSharedInstructions_AntigravityLockedToGemini(t *testing.T) {
	s, _ := newInstructionsServer(t, "antigravity", "gemini")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"p\n"}`)
	list := decodeBody[struct {
		Targets []sharedInstructionsTarget `json:"targets"`
	}](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions", ""))
	if list.Targets[0].Name != "antigravity" || list.Targets[0].SameAs != "gemini" {
		t.Fatalf("targets = %+v", list.Targets)
	}
	res := decodeBody[map[string]any](t, instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["antigravity"],"extras":["personal"]}`))
	if res["error_code"] != "instructions_same_file" {
		t.Errorf("assigning antigravity directly: %v, want failure", res)
	}
}

func TestExtrasDelete_RestoresSingleFileTargets(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	codex := writeHome(t, home, ".codex/AGENTS.md", "codex own\n")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"p\n"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["personal"]}`)

	if rr := instructionsRequest(t, s, http.MethodDelete, "/api/extras/personal", ""); rr.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rr.Code, rr.Body.String())
	}
	if got := readFile(t, codex); got != "codex own\n" {
		t.Errorf("codex AGENTS.md = %q, want restored", got)
	}
}

func TestExtrasRemoveTarget_RestoresSingleFileTarget(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	codex := writeHome(t, home, ".codex/AGENTS.md", "codex own\n")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"p\n"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["personal"]}`)

	rr := instructionsRequest(t, s, http.MethodDelete, "/api/extras/personal/targets", `{"path":"`+filepath.Dir(codex)+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("remove target: %d %s", rr.Code, rr.Body.String())
	}
	if got := readFile(t, codex); got != "codex own\n" {
		t.Errorf("codex AGENTS.md = %q, want restored", got)
	}
}

func TestProjectInstructions_ReachAndShim(t *testing.T) {
	s, root := newTestProjectServerWithExtras(t, nil)
	s.projectCfg.Targets = []config.ProjectTargetEntry{{Name: "claude"}, {Name: "gemini"}, {Name: "codex"}}
	if err := s.projectCfg.Save(root); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("a\n"), 0644)
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("own\n"), 0644)

	got := decodeBody[struct {
		Targets []struct {
			Target, How, Shim string
			Reads             bool
		} `json:"targets"`
	}](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions/project", ""))
	if len(got.Targets) != 3 || got.Targets[0].Target != "claude" || got.Targets[0].Shim != "import" || got.Targets[2].Shim != "link" {
		t.Fatalf("targets = %+v", got.Targets)
	}
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/project/shim", `{"target":"gemini"}`); rr.Code != http.StatusOK {
		t.Fatalf("shim: %d %s", rr.Code, rr.Body.String())
	}
	if dest, err := os.Readlink(filepath.Join(root, "GEMINI.md")); err != nil || dest != "AGENTS.md" {
		t.Errorf("GEMINI.md link = %q, %v", dest, err)
	}
}

func TestProjectInstructions_RenameBlockedByManagedBlock(t *testing.T) {
	s, root := newTestProjectServerWithExtras(t, nil)
	s.projectCfg.Targets = []config.ProjectTargetEntry{{Name: "claude"}}
	if err := s.projectCfg.Save(root); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("<!-- skillshare:instructions:begin -->\n@.skillshare/extras/team/AGENTS.md\n<!-- skillshare:instructions:end -->\n\nBe brief.\n"), 0644)

	got := decodeBody[instructionsFileResponse](t, instructionsRequest(t, s, http.MethodGet, "/api/targets/claude/instructions", ""))
	if strings.Contains(strings.Join(got.Convert, ","), "rename") || got.Blocked["rename"] != "shared" {
		t.Errorf("convert = %v, blocked = %v, want rename blocked as shared", got.Convert, got.Blocked)
	}
	rr := instructionsRequest(t, s, http.MethodPost, "/api/targets/claude/instructions/convert", `{"method":"rename","apply":true}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("rename convert: %d %s, want 400", rr.Code, rr.Body.String())
	}
}

func TestTargetInstructions_PutWritesThroughOwnSymlink(t *testing.T) {
	s, home := newInstructionsServer(t, "claude")
	real := writeHome(t, home, "dotfiles/CLAUDE.md", "mine\n")
	link := filepath.Join(home, ".claude", "CLAUDE.md")
	os.MkdirAll(filepath.Dir(link), 0755)
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	got := decodeBody[instructionsFileResponse](t, instructionsRequest(t, s, http.MethodGet, "/api/targets/claude/instructions", ""))
	if got.LinkShared != "" {
		t.Errorf("link_shared = %q, want empty for a user's own link", got.LinkShared)
	}
	rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/claude/instructions", `{"content":"edited\n"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 || readFile(t, real) != "edited\n" {
		t.Errorf("link kept = %v, dotfile = %q", err == nil && info.Mode()&os.ModeSymlink != 0, readFile(t, real))
	}
}

func TestTargetInstructions_PutRefusesSharedLink(t *testing.T) {
	s, _ := newInstructionsServer(t, "codex")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"p\n"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["personal"]}`)

	got := decodeBody[instructionsFileResponse](t, instructionsRequest(t, s, http.MethodGet, "/api/targets/codex/instructions", ""))
	if got.LinkShared != "personal" {
		t.Errorf("link_shared = %q, want personal", got.LinkShared)
	}
	if rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/codex/instructions", `{"content":"x\n"}`); rr.Code != http.StatusConflict {
		t.Errorf("status %d, want 409", rr.Code)
	}
}

// useAgentsSkills points universal at ~/.agents/skills, the folder its riders
// (Codex, Goose) read, and saves the config.
func useAgentsSkills(t *testing.T, s *Server, home string) {
	t.Helper()
	s.cfg.Targets["universal"] = config.TargetConfig{Path: filepath.Join(home, ".agents", "skills")}
	if err := s.saveAndReloadConfig(); err != nil {
		t.Fatal(err)
	}
}

func TestTargetInstructions_RiderGetAndPut(t *testing.T) {
	s, home := newInstructionsServer(t, "universal")
	useAgentsSkills(t, s, home)
	writeHome(t, home, ".codex/AGENTS.md", "codex own\n")

	got := decodeBody[instructionsFileResponse](t, instructionsRequest(t, s, http.MethodGet, "/api/targets/codex/instructions", ""))
	if !got.Supported || got.Path != filepath.Join(home, ".codex", "AGENTS.md") || got.Content != "codex own\n" || got.RiderOf != "universal" {
		t.Fatalf("codex = %+v", got)
	}
	if rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/codex/instructions", `{"content":"edited\n"}`); rr.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rr.Code, rr.Body.String())
	}
	if got := readFile(t, filepath.Join(home, ".codex", "AGENTS.md")); got != "edited\n" {
		t.Errorf("AGENTS.md = %q", got)
	}
	// A rider has no config entry to hold a custom file.
	if rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/codex/instructions/setup", `{"path":"~/x/AGENTS.md"}`); rr.Code != http.StatusNotFound {
		t.Errorf("setup: %d, want 404", rr.Code)
	}
}

func TestTargetInstructions_UnknownTargetNotFound(t *testing.T) {
	s, _ := newInstructionsServer(t, "universal")
	// codex is not installed here, so it is no rider.
	for _, name := range []string{"codex", "nope"} {
		if rr := instructionsRequest(t, s, http.MethodGet, "/api/targets/"+name+"/instructions", ""); rr.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", name, rr.Code)
		}
	}
}

func TestTargetInstructions_UniversalListsRidersAndReaders(t *testing.T) {
	s, home := newInstructionsServer(t, "universal")
	useAgentsSkills(t, s, home)
	os.MkdirAll(filepath.Join(home, ".codex"), 0755)

	got := decodeBody[instructionsFileResponse](t, instructionsRequest(t, s, http.MethodGet, "/api/targets/universal/instructions", ""))
	if len(got.Riders) != 1 || got.Riders[0].Name != "codex" || got.Riders[0].Path != filepath.Join(home, ".codex", "AGENTS.md") || got.Riders[0].Exists {
		t.Errorf("riders = %+v", got.Riders)
	}
	if strings.Join(got.ReadBy, ",") != "cline,jazz,warp" {
		t.Errorf("read_by = %v", got.ReadBy)
	}
}

func TestSharedInstructions_AssignAndRestoreRider(t *testing.T) {
	s, home := newInstructionsServer(t, "universal")
	useAgentsSkills(t, s, home)
	codex := writeHome(t, home, ".codex/AGENTS.md", "codex own\n")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"p\n"}`)

	list := decodeBody[struct {
		Targets []sharedInstructionsTarget `json:"targets"`
	}](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions", ""))
	if len(list.Targets) != 2 || list.Targets[0].Name != "codex" || list.Targets[0].RiderOf != "universal" {
		t.Fatalf("targets = %+v", list.Targets)
	}
	res := decodeBody[map[string]any](t, instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["personal"]}`))
	if res["success"] != true || readFile(t, codex) != "p\n" {
		t.Fatalf("assign: %v, codex = %q", res, readFile(t, codex))
	}
	res = decodeBody[map[string]any](t, instructionsRequest(t, s, http.MethodPost, "/api/instructions/personal/restore", `{"target":"codex"}`))
	if res["success"] != true || readFile(t, codex) != "codex own\n" {
		t.Errorf("restore: %v, codex = %q", res, readFile(t, codex))
	}
}

func TestSharedInstructions_ReportsCopyWithoutFileLinks(t *testing.T) {
	t.Cleanup(ssync.SetFileLinksForTest(false))
	s, home := newInstructionsServer(t, "codex")
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"team","content":"team\n"}`); rr.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	if res := decodeBody[map[string]any](t, instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["team"]}`)); res["success"] != true {
		t.Fatalf("assign: %v", res)
	}

	list := decodeBody[struct {
		Targets []sharedInstructionsTarget `json:"targets"`
	}](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions", ""))

	info, err := os.Lstat(filepath.Join(home, ".codex", "AGENTS.md"))
	if a := list.Targets[0].Assigned; len(a) != 1 || a[0].Mode != "copy" || a[0].Status != "synced" || err != nil || !info.Mode().IsRegular() {
		t.Errorf("codex assigned = %+v, want a synced copy", a)
	}
}

func TestSharedInstructions_OnlyAgentsFileExtras(t *testing.T) {
	s, _ := newInstructionsServer(t, "claude")
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"p\n"}`); rr.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	s.cfg.Extras = append(s.cfg.Extras, config.ExtraConfig{Name: "prompt", File: "system.md", Targets: []config.ExtraTargetConfig{}})

	got := decodeBody[struct {
		Files []sharedInstructionsFile `json:"files"`
	}](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions", ""))
	if len(got.Files) != 1 || got.Files[0].Name != "personal" {
		t.Errorf("files = %+v, want only personal", got.Files)
	}
	rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude"],"extras":["prompt"]}`)
	if rr.Code != http.StatusNotFound {
		t.Errorf("assign prompt: %d %s, want 404", rr.Code, rr.Body.String())
	}
}
