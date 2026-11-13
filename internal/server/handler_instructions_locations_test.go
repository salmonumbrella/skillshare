package server

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	ssync "skillshare/internal/sync"
)

func createShared(t *testing.T, s *Server, name string) {
	t.Helper()
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"`+name+`","content":"`+name+`\n"}`); rr.Code != http.StatusOK {
		t.Fatalf("create %s: %d %s", name, rr.Code, rr.Body.String())
	}
}

func addLocation(t *testing.T, s *Server, name, body string) {
	t.Helper()
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/"+name+"/locations", body); rr.Code != http.StatusOK {
		t.Fatalf("add location: %d %s", rr.Code, rr.Body.String())
	}
}

func listShared(t *testing.T, s *Server) []sharedInstructionsFile {
	t.Helper()
	return decodeBody[struct {
		Files []sharedInstructionsFile `json:"files"`
	}](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions", "")).Files
}

func TestSharedInstructionsLocations_ListExcludesToolFiles(t *testing.T) {
	s, home := newInstructionsServer(t, "claude")
	attachShared(t, s, "claude")
	addLocation(t, s, "team", `{"path":"~/notes","as":"NOTES.md"}`)

	files := listShared(t, s)
	want := sharedInstructionsLocation{Path: filepath.Join(home, "notes"), File: filepath.Join(home, "notes", "NOTES.md"), As: "NOTES.md", Mode: "symlink", Status: "synced"}
	if len(files) != 1 || len(files[0].Locations) != 1 || files[0].Locations[0] != want {
		t.Errorf("files = %+v, want only the notes location", files)
	}
}

func TestSharedInstructionsLocations_AddSyncsRightAway(t *testing.T) {
	for _, mode := range []string{"symlink", "copy"} {
		t.Run(mode, func(t *testing.T) {
			s, home := newInstructionsServer(t)
			createShared(t, s, "team")
			addLocation(t, s, "team", `{"path":"~/notes","as":"NOTES.md","mode":"`+mode+`"}`)

			file := filepath.Join(home, "notes", "NOTES.md")
			info, err := os.Lstat(file)
			if err != nil || readFile(t, file) != "team\n" || (info.Mode()&os.ModeSymlink != 0) != (mode == "symlink") {
				t.Errorf("%s: info = %v, err = %v", file, info, err)
			}
		})
	}
}

func TestSharedInstructionsLocations_AddRejects(t *testing.T) {
	cases := []struct {
		name, code string
		setup      func(t *testing.T, s *Server, home string)
		body       string
	}{
		{"existing", "instructions_location_exists", func(t *testing.T, s *Server, home string) {
			addLocation(t, s, "team", `{"path":"~/notes"}`)
		}, `{"path":"~/notes","as":"OTHER.md"}`},
		{"tool file", "instructions_location_is_target", nil, `{"path":"~/.claude","as":"CLAUDE.md"}`},
		{"connected tool file", "instructions_location_is_target", func(t *testing.T, s *Server, home string) {
			if rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude"],"extras":["team"]}`); rr.Code != http.StatusOK {
				t.Fatalf("assign: %d %s", rr.Code, rr.Body.String())
			}
		}, `{"path":"~/.claude","as":"CLAUDE.md"}`},
		{"held by other shared", "instructions_target_held", func(t *testing.T, s *Server, home string) {
			createShared(t, s, "other")
			addLocation(t, s, "other", `{"path":"~/notes"}`)
		}, `{"path":"~/notes"}`},
		{"directory in the way", "instructions_location_directory", func(t *testing.T, s *Server, home string) {
			os.MkdirAll(filepath.Join(home, "notes", "AGENTS.md"), 0755)
		}, `{"path":"~/notes"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, home := newInstructionsServer(t, "claude")
			createShared(t, s, "team")
			if tc.setup != nil {
				tc.setup(t, s, home)
			}
			before := len(s.cfg.Extras[0].Targets)

			rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/team/locations", tc.body)
			got := decodeBody[map[string]any](t, rr)
			if rr.Code != http.StatusConflict || got["error_code"] != tc.code || len(s.cfg.Extras[0].Targets) != before {
				t.Errorf("status %d, body %v, targets %d; want 409 %s and nothing saved", rr.Code, got, len(s.cfg.Extras[0].Targets), tc.code)
			}
		})
	}
}

func TestSharedInstructionsLocations_ModeChange(t *testing.T) {
	s, home := newInstructionsServer(t)
	createShared(t, s, "team")
	addLocation(t, s, "team", `{"path":"~/notes"}`)

	rr := instructionsRequest(t, s, http.MethodPut, "/api/instructions/team/locations/mode", `{"path":"`+filepath.Join(home, "notes")+`","mode":"copy"}`)
	file := filepath.Join(home, "notes", "AGENTS.md")
	info, err := os.Lstat(file)
	if rr.Code != http.StatusOK || err != nil || info.Mode()&os.ModeSymlink != 0 || readFile(t, file) != "team\n" {
		t.Errorf("mode: %d %s; want a copy at %s", rr.Code, rr.Body.String(), file)
	}
}

func TestSharedInstructionsLocations_DeleteRestoresFile(t *testing.T) {
	s, home := newInstructionsServer(t)
	file := writeHome(t, home, "notes/AGENTS.md", "mine\n")
	createShared(t, s, "team")
	addLocation(t, s, "team", `{"path":"~/notes"}`)

	rr := instructionsRequest(t, s, http.MethodDelete, "/api/instructions/team/locations?path="+url.QueryEscape(filepath.Join(home, "notes")), "")
	if rr.Code != http.StatusOK || readFile(t, file) != "mine\n" || len(listShared(t, s)[0].Locations) != 0 {
		t.Errorf("delete: %d %s; file = %q", rr.Code, rr.Body.String(), readFile(t, file))
	}
}

func TestSharedInstructionsLocations_DeleteUnknownPath(t *testing.T) {
	s, _ := newInstructionsServer(t)
	createShared(t, s, "team")

	rr := instructionsRequest(t, s, http.MethodDelete, "/api/instructions/team/locations?path=/nowhere", "")
	if got := decodeBody[map[string]any](t, rr); rr.Code != http.StatusNotFound || got["error_code"] != "instructions_location_not_found" {
		t.Errorf("status %d, body %v", rr.Code, got)
	}
}

func TestSharedInstructionsLocations_RestorePreview(t *testing.T) {
	s, home := newInstructionsServer(t)
	writeHome(t, home, "notes/AGENTS.md", "mine\n")
	createShared(t, s, "team")
	addLocation(t, s, "team", `{"path":"~/notes"}`)

	got := decodeBody[sharedRestorePreview](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions/team/locations/restore-preview?path="+url.QueryEscape(filepath.Join(home, "notes")), ""))
	if got.Kind != ssync.RestoreKindContent || got.Content != "mine\n" || got.Current != "team\n" {
		t.Errorf("preview = %+v, want the replaced file", got)
	}
}

func TestExtrasAddTarget_StoresAs(t *testing.T) {
	s, home := newInstructionsServer(t)
	createShared(t, s, "team")

	rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/team/targets", `{"path":"`+filepath.Join(home, "notes")+`","as":"NOTES.md"}`)
	if rr.Code != http.StatusOK || len(s.cfg.Extras[0].Targets) != 1 || s.cfg.Extras[0].Targets[0].As != "NOTES.md" {
		t.Errorf("add target: %d %s; targets = %+v", rr.Code, rr.Body.String(), s.cfg.Extras[0].Targets)
	}
}

func TestExtrasAddTarget_RefusesFileHeldByOtherShared(t *testing.T) {
	s, home := newInstructionsServer(t)
	createShared(t, s, "team")
	createShared(t, s, "other")
	dir := filepath.Join(home, "notes")
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/team/targets", `{"path":"`+dir+`","mode":"symlink"}`); rr.Code != http.StatusOK {
		t.Fatalf("first target: %d %s", rr.Code, rr.Body.String())
	}

	rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/other/targets", `{"path":"`+dir+`","mode":"symlink"}`)
	if rr.Code != http.StatusConflict || len(s.cfg.Extras[1].Targets) != 0 {
		t.Errorf("status %d, body %s, other targets = %+v; want 409 and nothing saved", rr.Code, rr.Body.String(), s.cfg.Extras[1].Targets)
	}
}

func TestSharedInstructionsLocations_AddRestoresFileWhenSaveFails(t *testing.T) {
	s, home := newInstructionsServer(t)
	file := writeHome(t, home, "notes/AGENTS.md", "mine\n")
	createShared(t, s, "team")
	// A folder at the config path makes the save fail after the file is written.
	cfgPath := os.Getenv("SKILLSHARE_CONFIG")
	os.Remove(cfgPath)
	os.Mkdir(cfgPath, 0755)

	rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/team/locations", `{"path":"~/notes"}`)
	info, err := os.Lstat(file)
	if rr.Code != http.StatusInternalServerError || err != nil || info.Mode()&os.ModeSymlink != 0 || readFile(t, file) != "mine\n" || len(s.cfg.Extras[0].Targets) != 0 {
		t.Errorf("status %d %s; want 500 with %s restored and no target", rr.Code, rr.Body.String(), file)
	}
}

func TestSharedInstructionsLocations_ResolveCollect(t *testing.T) {
	s, home := newInstructionsServer(t)
	createShared(t, s, "team")
	addLocation(t, s, "team", `{"path":"~/notes"}`)
	// A tool replaced the link with its own edited copy.
	file := filepath.Join(home, "notes", "AGENTS.md")
	os.Remove(file)
	writeHome(t, home, "notes/AGENTS.md", "edited\n")

	rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/team/resolve", `{"path":"~/notes","action":"collect"}`)
	if rr.Code != http.StatusOK || readFile(t, listShared(t, s)[0].Path) != "edited\n" {
		t.Errorf("resolve: %d %s; want the edit collected into the shared file", rr.Code, rr.Body.String())
	}
}
