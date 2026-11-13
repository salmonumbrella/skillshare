package server

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

// newProjectSharedServer returns a project-mode server with the unattached
// shared file "team" (content "team\n").
func newProjectSharedServer(t *testing.T) (*Server, string) {
	t.Helper()
	s, root := newTestProjectServerWithExtras(t, nil)
	createShared(t, s, "team")
	return s, root
}

func TestProjectShared_ListShowsEveryTargetAsLocation(t *testing.T) {
	s, root := newProjectSharedServer(t)
	addLocation(t, s, "team", `{"path":"./","as":"CLAUDE.md","mode":"import"}`)

	got := decodeBody[struct {
		Files   []sharedInstructionsFile   `json:"files"`
		Targets []sharedInstructionsTarget `json:"targets"`
	}](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions", ""))
	want := sharedInstructionsLocation{Path: ".", File: filepath.Join(root, "CLAUDE.md"), As: "CLAUDE.md", Mode: "import", Status: "synced"}
	if len(got.Targets) != 0 || len(got.Files) != 1 || got.Files[0].Path != filepath.Join(root, ".skillshare", "extras", "team", "AGENTS.md") ||
		len(got.Files[0].Locations) != 1 || got.Files[0].Locations[0] != want {
		t.Errorf("list = %+v, want team with the ./CLAUDE.md location and no targets", got)
	}
}

func TestProjectShared_CreateKeepsGlobalConfig(t *testing.T) {
	s, root := newProjectSharedServer(t)

	pcfg, err := config.LoadProject(root)
	if err != nil || len(pcfg.Extras) != 1 || pcfg.Extras[0].File != "AGENTS.md" || len(s.cfg.Extras) != 0 {
		t.Errorf("project extras = %+v (%v), global extras = %+v", pcfg, err, s.cfg.Extras)
	}
}

func TestProjectShared_CreateFromTargetNeedsGlobal(t *testing.T) {
	s, _ := newTestProjectServerWithExtras(t, nil)

	rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"team","from_target":"claude"}`)
	if got := decodeBody[map[string]any](t, rr); rr.Code != http.StatusBadRequest || got["error_code"] != "instructions_global_required" {
		t.Errorf("status %d, body %v", rr.Code, got)
	}
}

func TestProjectShared_ContentRewritesCopies(t *testing.T) {
	s, root := newProjectSharedServer(t)
	addLocation(t, s, "team", `{"path":"docs","mode":"copy"}`)

	if rr := instructionsRequest(t, s, http.MethodPut, "/api/instructions/team/content", `{"content":"new\n"}`); rr.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rr.Code, rr.Body.String())
	}
	got := decodeBody[struct{ Content string }](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions/team/content", ""))
	if got.Content != "new\n" || readFile(t, filepath.Join(root, "docs", "AGENTS.md")) != "new\n" {
		t.Errorf("content = %q, copy = %q", got.Content, readFile(t, filepath.Join(root, "docs", "AGENTS.md")))
	}
}

func TestProjectShared_AddSymlinkIsRelative(t *testing.T) {
	s, root := newProjectSharedServer(t)
	addLocation(t, s, "team", `{"path":"./docs/ai/","as":"instructions.md"}`)

	file := filepath.Join(root, "docs", "ai", "instructions.md")
	dest, err := os.Readlink(file)
	pcfg, _ := config.LoadProject(root)
	if err != nil || filepath.IsAbs(dest) || readFile(t, file) != "team\n" || pcfg.Extras[0].Targets[0].Path != "docs/ai" {
		t.Errorf("link = %q, %v; stored targets = %+v", dest, err, pcfg.Extras[0].Targets)
	}
}

func TestProjectShared_AddImportLineIsRelative(t *testing.T) {
	s, root := newProjectSharedServer(t)
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("Be brief.\n"), 0644)
	addLocation(t, s, "team", `{"path":".","as":"CLAUDE.md","mode":"import"}`)

	if got := readFile(t, filepath.Join(root, "CLAUDE.md")); !strings.Contains(got, "@.skillshare/extras/team/AGENTS.md") || !strings.Contains(got, "Be brief.") {
		t.Errorf("CLAUDE.md = %q, want a relative import line and the own lines", got)
	}
}

func TestProjectShared_AddRejectsPathsOutsideProject(t *testing.T) {
	for _, path := range []string{"/tmp/notes", "~/notes", "../notes", "docs/../../notes"} {
		t.Run(path, func(t *testing.T) {
			s, _ := newProjectSharedServer(t)
			rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/team/locations", `{"path":"`+path+`"}`)
			if got := decodeBody[map[string]any](t, rr); rr.Code != http.StatusBadRequest || got["error_code"] != "instructions_location_outside_project" {
				t.Errorf("status %d, body %v", rr.Code, got)
			}
		})
	}
}

func TestProjectShared_ModeChange(t *testing.T) {
	s, root := newProjectSharedServer(t)
	addLocation(t, s, "team", `{"path":"docs"}`)

	rr := instructionsRequest(t, s, http.MethodPut, "/api/instructions/team/locations/mode", `{"path":"docs","mode":"copy"}`)
	file := filepath.Join(root, "docs", "AGENTS.md")
	info, err := os.Lstat(file)
	if rr.Code != http.StatusOK || err != nil || info.Mode()&os.ModeSymlink != 0 || readFile(t, file) != "team\n" {
		t.Errorf("mode: %d %s; want a copy at %s", rr.Code, rr.Body.String(), file)
	}
}

func TestProjectShared_DeleteRestoresFile(t *testing.T) {
	s, root := newProjectSharedServer(t)
	file := filepath.Join(root, "docs", "AGENTS.md")
	os.MkdirAll(filepath.Dir(file), 0755)
	os.WriteFile(file, []byte("mine\n"), 0644)
	addLocation(t, s, "team", `{"path":"docs"}`)

	preview := decodeBody[sharedRestorePreview](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions/team/locations/restore-preview?path=docs", ""))
	if preview.Kind != ssync.RestoreKindContent || preview.Content != "mine\n" || preview.Current != "team\n" {
		t.Errorf("preview = %+v, want the replaced file", preview)
	}
	rr := instructionsRequest(t, s, http.MethodDelete, "/api/instructions/team/locations?path="+url.QueryEscape("docs"), "")
	if rr.Code != http.StatusOK || readFile(t, file) != "mine\n" || len(listShared(t, s)[0].Locations) != 0 {
		t.Errorf("delete: %d %s; file = %q", rr.Code, rr.Body.String(), readFile(t, file))
	}
}

func TestProjectShared_ResolveCollectByPath(t *testing.T) {
	s, root := newProjectSharedServer(t)
	addLocation(t, s, "team", `{"path":"docs"}`)
	// A tool replaced the link with its own edited copy.
	file := filepath.Join(root, "docs", "AGENTS.md")
	os.Remove(file)
	os.WriteFile(file, []byte("edited\n"), 0644)

	rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/team/resolve", `{"path":"docs","action":"collect"}`)
	if rr.Code != http.StatusOK || readFile(t, listShared(t, s)[0].Path) != "edited\n" {
		t.Errorf("resolve: %d %s; want the edit collected into the shared file", rr.Code, rr.Body.String())
	}
}
