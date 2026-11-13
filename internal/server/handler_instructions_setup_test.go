package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func getTargetInstructions(t *testing.T, s *Server, name string) instructionsFileResponse {
	t.Helper()
	return decodeBody[instructionsFileResponse](t, instructionsRequest(t, s, http.MethodGet, "/api/targets/"+name+"/instructions", ""))
}

func TestInstructionsSetup_CustomTargetBecomesSupported(t *testing.T) {
	s, home := newInstructionsServer(t, "myagent")
	if got := getTargetInstructions(t, s, "myagent"); got.Supported {
		t.Fatalf("before setup: %+v, want unsupported", got)
	}

	rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/myagent/instructions/setup", `{"path":"~/.myagent/AGENTS.md","import":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", rr.Code, rr.Body.String())
	}
	got := getTargetInstructions(t, s, "myagent")
	if !got.Supported || !got.Custom || !got.Import || got.Path != filepath.Join(home, ".myagent", "AGENTS.md") || got.Setup.Path != "~/.myagent/AGENTS.md" {
		t.Errorf("after setup: %+v", got)
	}
}

func TestInstructionsSetup_RejectsRelativeGlobalPath(t *testing.T) {
	s, _ := newInstructionsServer(t, "myagent")
	rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/myagent/instructions/setup", `{"path":"AGENTS.md"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("setup relative: %d %s, want 400", rr.Code, rr.Body.String())
	}
}

func TestInstructionsSetup_CustomTargetCanUseSharedFile(t *testing.T) {
	s, home := newInstructionsServer(t, "myagent")
	instructionsRequest(t, s, http.MethodPut, "/api/targets/myagent/instructions/setup", `{"path":"~/.myagent/AGENTS.md","import":true}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"p\n"}`)

	list := decodeBody[struct {
		Targets []sharedInstructionsTarget `json:"targets"`
	}](t, instructionsRequest(t, s, http.MethodGet, "/api/instructions", ""))
	if len(list.Targets) != 1 || list.Targets[0].Name != "myagent" {
		t.Fatalf("grouping targets = %+v, want myagent", list.Targets)
	}
	res := decodeBody[map[string]any](t, instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["myagent"],"extras":["personal"]}`))
	if res["success"] != true {
		t.Fatalf("assign: %v", res)
	}
	if got := readFile(t, filepath.Join(home, ".myagent", "AGENTS.md")); !strings.Contains(got, "@") {
		t.Errorf("AGENTS.md = %q, want an import of the shared file", got)
	}
}

func TestInstructionsSetup_RemoveRefusedWhileShared(t *testing.T) {
	s, _ := newInstructionsServer(t, "myagent")
	instructionsRequest(t, s, http.MethodPut, "/api/targets/myagent/instructions/setup", `{"path":"~/.myagent/AGENTS.md","import":true}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"p\n"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["myagent"],"extras":["personal"]}`)

	rr := instructionsRequest(t, s, http.MethodDelete, "/api/targets/myagent/instructions/setup", "")
	if res := decodeBody[map[string]any](t, rr); rr.Code != http.StatusConflict || res["error_code"] != errInstructionsInUse {
		t.Fatalf("remove while shared: %d %v, want 409 %s", rr.Code, res, errInstructionsInUse)
	}
	rr = instructionsRequest(t, s, http.MethodPut, "/api/targets/myagent/instructions/setup", `{"path":"~/.other/AGENTS.md"}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("move while shared: %d %s, want 409", rr.Code, rr.Body.String())
	}

	instructionsRequest(t, s, http.MethodPost, "/api/instructions/personal/restore", `{"target":"myagent"}`)
	if rr := instructionsRequest(t, s, http.MethodDelete, "/api/targets/myagent/instructions/setup", ""); rr.Code != http.StatusOK {
		t.Fatalf("remove: %d %s", rr.Code, rr.Body.String())
	}
	if got := getTargetInstructions(t, s, "myagent"); got.Supported || got.Custom {
		t.Errorf("after remove: %+v, want unsupported", got)
	}
}

func TestInstructionsSetup_ProjectPathIsRelativeToRoot(t *testing.T) {
	s, root := newTestProjectServerWithExtras(t, nil)
	s.projectCfg.Targets = []config.ProjectTargetEntry{{Name: "myagent", Skills: &config.ResourceTargetConfig{Path: ".myagent/skills"}}}
	if err := s.projectCfg.Save(root); err != nil {
		t.Fatal(err)
	}
	if err := s.reloadConfig(); err != nil {
		t.Fatal(err)
	}

	rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/myagent/instructions/setup", `{"path":".myagent/AGENTS.md"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", rr.Code, rr.Body.String())
	}
	if got := getTargetInstructions(t, s, "myagent"); !got.Custom || got.Path != filepath.Join(root, ".myagent", "AGENTS.md") {
		t.Errorf("project setup: %+v", got)
	}
	data, err := os.ReadFile(config.ProjectConfigPath(root))
	if err != nil || !strings.Contains(string(data), "instructions:") {
		t.Errorf("project config = %q, %v; want instructions saved", data, err)
	}
}

// The dashboard prefills its "change" form from the raw JSON keys.
func TestInstructionsSetup_ResponseUsesJSONKeys(t *testing.T) {
	s, _ := newInstructionsServer(t, "myagent")
	instructionsRequest(t, s, http.MethodPut, "/api/targets/myagent/instructions/setup", `{"path":"~/.myagent/AGENTS.md","import":true}`)
	body := instructionsRequest(t, s, http.MethodGet, "/api/targets/myagent/instructions", "").Body.String()
	if !strings.Contains(body, `"setup":{"path":"~/.myagent/AGENTS.md","import":true}`) {
		t.Errorf("setup JSON = %s", body)
	}
}
