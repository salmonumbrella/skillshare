package server

import (
	"net/http"
	"os"
	"path/filepath"
	"skillshare/internal/config"
	syncpkg "skillshare/internal/sync"
	"strings"
	"testing"
)

func TestInstructionsSetupRejectsExistingDirectory(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	os.MkdirAll(filepath.Join(home, ".codex"), 0755)
	for _, path := range []string{"~/.codex", "~/.codex/"} {
		rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/codex/instructions/setup", `{"path":"`+path+`"}`)
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "must name a file, not a directory") {
			t.Fatalf("%s: %d %s", path, rr.Code, rr.Body.String())
		}
	}
}

func TestInstructionsModeReturnsWarnings(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	attachShared(t, s, "codex")
	instructionsRequest(t, s, http.MethodPut, "/api/instructions/team/targets/codex/mode", `{"mode":"copy"}`)
	os.WriteFile(filepath.Join(home, ".codex", "AGENTS.md"), []byte("edited"), 0644)
	rr := instructionsRequest(t, s, http.MethodPut, "/api/instructions/team/targets/codex/mode", `{"mode":"symlink"}`)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"code":"backed_up"`) {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	warnings := decodeBody[struct {
		Warnings []syncpkg.FileWarning `json:"warnings"`
	}](t, rr).Warnings
	path := filepath.Join(home, ".codex", "AGENTS.md")
	if len(warnings) != 1 || warnings[0].Params["path"] != path || warnings[0].Message != "backed up "+path+" before replacing it" {
		t.Fatalf("warnings: %+v", warnings)
	}

}

func TestInstructionsAssignRejectsMixedOwnership(t *testing.T) {
	for _, mode := range []string{"symlink", "copy"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := newInstructionsServer(t, "claude")
			attachShared(t, s, "claude")
			instructionsRequest(t, s, http.MethodPut, "/api/instructions/team/targets/claude/mode", `{"mode":"`+mode+`"}`)
			instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"other","content":"other"}`)
			rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude"],"extras":["team","other"]}`)
			if rr.Code != 409 || !strings.Contains(rr.Body.String(), "team") || !strings.Contains(rr.Body.String(), `"error_code":"instructions_target_held"`) {
				t.Fatalf("%d %s", rr.Code, rr.Body.String())
			}
			extra, _ := s.sharedExtra("team")
			if got := readFile(t, filepath.Join(s.extrasSourceDir(extra), extra.File)); got != "team\n" {
				t.Fatalf("source corrupted: %q", got)
			}
		})
	}
}

func TestInstructionsAssignRejectsUntrackedSharedLink(t *testing.T) {
	s, home := newInstructionsServer(t, "claude")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"personal"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"other","content":"other"}`)
	extra, _ := s.sharedExtra("personal")
	os.MkdirAll(filepath.Join(home, ".claude"), 0755)
	os.Symlink(filepath.Join(s.extrasSourceDir(extra), extra.File), filepath.Join(home, ".claude", "CLAUDE.md"))
	rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude"],"extras":["other"]}`)
	if rr.Code != 409 || !strings.Contains(rr.Body.String(), "personal") {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	list := instructionsRequest(t, s, http.MethodGet, "/api/instructions", "")
	if !strings.Contains(list.Body.String(), `"linked_shared":"personal"`) {
		t.Fatal(list.Body.String())
	}
}

func TestInstructionsModeConflictNamesOtherSharedFile(t *testing.T) {
	s, _ := newInstructionsServer(t, "claude")
	attachShared(t, s, "claude")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"other","content":"other"}`)
	rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude"],"extras":["team","other"]}`)
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	for _, name := range []string{"team", "other"} {
		rr = instructionsRequest(t, s, http.MethodPut, "/api/instructions/"+name+"/targets/claude/mode", `{"mode":"copy"}`)
		body := decodeBody[struct {
			Code   string            `json:"error_code"`
			Params map[string]string `json:"error_params"`
		}](t, rr)
		other := "team"
		if name == "team" {
			other = "other"
		}
		if rr.Code != 409 || body.Code != "instructions_target_held" || body.Params["name"] != other || body.Params["target"] != "claude" {
			t.Fatalf("%d %s", rr.Code, rr.Body.String())
		}
	}
}

func TestInstructionsAssignIncludesWarnings(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	writeHome(t, home, ".codex/AGENTS.md", "mine")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"team","content":"team"}`)
	rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["team"]}`)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"code":"backed_up"`) {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	warnings := decodeBody[struct {
		Warnings []syncpkg.FileWarning `json:"warnings"`
	}](t, rr).Warnings
	path := filepath.Join(home, ".codex", "AGENTS.md")
	if len(warnings) != 1 || warnings[0].Params["path"] != path || warnings[0].Message != "backed up "+path+" before replacing it" {
		t.Fatalf("warnings: %+v", warnings)
	}

	rr = instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["team"]}`)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"warnings":[]`) {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	rr = instructionsRequest(t, s, http.MethodPut, "/api/instructions/team/targets/codex/mode", `{"mode":"symlink"}`)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"warnings":[]`) {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestInstructionsDeleteKeepsConfigOnRestoreFailure(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	writeHome(t, home, ".codex/AGENTS.md", "original")
	attachShared(t, s, "codex")
	entries, err := os.ReadDir(filepath.Join(config.StateDir(), "extras", "backups"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		path := filepath.Join(config.StateDir(), "extras", "backups", e.Name(), "restore")
		if _, err := os.Stat(path); err == nil {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	before := readFile(t, config.ConfigPath())
	rr := instructionsRequest(t, s, http.MethodDelete, "/api/extras/team", "")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	if got := readFile(t, config.ConfigPath()); got != before {
		t.Fatal("failed restore removed config")
	}
}

func TestInstructionsCodedErrors(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	cases := []struct{ method, url, body, code, key, value string }{
		{"PUT", "/api/targets/codex/instructions/setup", `{"path":"~/.codex/"}`, "instructions_path_directory", "path", "~/.codex/"},
		{"PUT", "/api/targets/codex/instructions/setup", `{"path":"relative.md"}`, "instructions_path_absolute", "path", "relative.md"},
		{"PUT", "/api/instructions/team/targets/missing/mode", `{"mode":"copy"}`, "instructions_target_not_found", "target", "missing"},
		{"PUT", "/api/instructions/team/targets/codex/mode", `{"mode":"invalid"}`, "instructions_invalid_mode", "mode", "invalid"},
		{"POST", "/api/instructions/team/restore", `{}`, "instructions_target_required", "", ""},
		{"POST", "/api/targets/codex/instructions/convert", `{"method":"invalid"}`, "instructions_method_unavailable", "method", "invalid"},
	}
	for _, c := range cases {
		rr := instructionsRequest(t, s, c.method, c.url, c.body)
		body := decodeBody[struct {
			Code   string            `json:"error_code"`
			Params map[string]string `json:"error_params"`
		}](t, rr)
		if rr.Code < 400 || body.Code != c.code || c.key != "" && body.Params[c.key] != c.value {
			t.Fatalf("%s: %d %s", c.url, rr.Code, rr.Body.String())
		}
	}
	attachShared(t, s, "codex")
	path := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	rr := instructionsRequest(t, s, "PUT", "/api/instructions/team/targets/codex/mode", `{"mode":"copy"}`)
	body := decodeBody[struct {
		Code   string            `json:"error_code"`
		Params map[string]string `json:"error_params"`
	}](t, rr)
	if rr.Code != 409 || body.Code != "instructions_target_directory" || body.Params["path"] != path {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}
