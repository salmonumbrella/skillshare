package server

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"skillshare/internal/config"
)

func targetFilesRequest(t *testing.T, s *Server, method, path, body string, want int) *targetFilesResponse {
	t.Helper()
	rr := instructionsRequest(t, s, method, path, body)
	if rr.Code != want {
		t.Fatalf("%s %s: status %d, want %d: %s", method, path, rr.Code, want, rr.Body.String())
	}
	if want != http.StatusOK {
		return nil
	}
	got := decodeBody[targetFilesResponse](t, rr)
	return &got
}

func TestTargetFiles_ListPiBuiltin(t *testing.T) {
	s, home := newInstructionsServer(t, "pi")
	writeHome(t, home, ".pi/agent/APPEND_SYSTEM.md", "extra\n")

	got := targetFilesRequest(t, s, http.MethodGet, "/api/targets/pi/files", "", http.StatusOK)
	want := targetFile{Path: "APPEND_SYSTEM.md", Abs: filepath.Join(home, ".pi", "agent", "APPEND_SYSTEM.md"), Builtin: true, Exists: true, Size: 6}
	if got.Root != filepath.Join(home, ".pi", "agent") || len(got.Files) != 1 || got.Files[0] != want {
		t.Errorf("list = %+v", got)
	}
}

func TestTargetFiles_UnknownTarget404(t *testing.T) {
	s, _ := newInstructionsServer(t, "pi")
	rr := instructionsRequest(t, s, http.MethodGet, "/api/targets/nope/files", "")
	if got := decodeBody[map[string]any](t, rr); rr.Code != http.StatusNotFound || got["error_code"] != "target_files_not_found" {
		t.Errorf("status %d, body %v", rr.Code, got)
	}
}

func TestTargetFiles_ContentNotListed404(t *testing.T) {
	s, _ := newInstructionsServer(t, "pi")
	rr := instructionsRequest(t, s, http.MethodGet, "/api/targets/pi/files/content?path=SECRET.md", "")
	if got := decodeBody[map[string]any](t, rr); rr.Code != http.StatusNotFound || got["error_code"] != "target_file_not_listed" {
		t.Errorf("status %d, body %v", rr.Code, got)
	}
}

func TestTargetFiles_AddPersistsAndPutCreatesSubfolder(t *testing.T) {
	s, home := newInstructionsServer(t, "pi")
	got := targetFilesRequest(t, s, http.MethodPost, "/api/targets/pi/files", `{"path":"prompts/review.md"}`, http.StatusOK)
	if len(got.Files) != 2 || got.Files[1].Path != "prompts/review.md" || got.Files[1].Builtin {
		t.Fatalf("list after add = %+v", got)
	}
	cfg, err := config.Load()
	if err != nil || !slices.Equal(cfg.Targets["pi"].Files, []string{"prompts/review.md"}) {
		t.Fatalf("saved files = %v (%v)", cfg.Targets["pi"].Files, err)
	}

	rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/pi/files/content?path=prompts/review.md", `{"content":"review\n"}`)
	if c := decodeBody[targetFileContent](t, rr); rr.Code != http.StatusOK || !c.Exists || c.Content != "review\n" {
		t.Fatalf("put: %d %+v", rr.Code, c)
	}
	if got := readFile(t, filepath.Join(home, ".pi", "agent", "prompts", "review.md")); got != "review\n" {
		t.Errorf("file = %q", got)
	}
}

func TestTargetFiles_AddRejectsInvalidPath(t *testing.T) {
	s, _ := newInstructionsServer(t, "pi")
	for rel, reason := range map[string]string{"/etc/passwd": "absolute", "../x.md": "outside", "APPEND_SYSTEM.md": "listed"} {
		rr := instructionsRequest(t, s, http.MethodPost, "/api/targets/pi/files", `{"path":"`+rel+`"}`)
		got := decodeBody[map[string]any](t, rr)
		params, _ := got["error_params"].(map[string]any)
		if rr.Code != http.StatusBadRequest || got["error_code"] != "target_file_invalid_path" || params["reason"] != reason {
			t.Errorf("%s: status %d, body %v", rel, rr.Code, got)
		}
	}
}

func TestTargetFiles_PutFollowsSharedLink(t *testing.T) {
	s, home := newInstructionsServer(t, "pi")
	createShared(t, s, "team")
	addLocation(t, s, "team", `{"path":"~/.pi/agent","as":"APPEND_SYSTEM.md","mode":"symlink"}`)
	link := filepath.Join(home, ".pi", "agent", "APPEND_SYSTEM.md")
	source, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("not a link: %v", err)
	}

	rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/pi/files/content?path=APPEND_SYSTEM.md", `{"content":"edited\n"}`)
	c := decodeBody[targetFileContent](t, rr)
	if rr.Code != http.StatusOK || c.LinkTo != source || c.LinkShared != "team" || c.Content != "edited\n" {
		t.Errorf("put: %d %+v", rr.Code, c)
	}
	if got := readFile(t, source); got != "edited\n" {
		t.Errorf("shared source = %q", got)
	}
}

func TestTargetFiles_DeleteBuiltinRejected(t *testing.T) {
	s, _ := newInstructionsServer(t, "pi")
	rr := instructionsRequest(t, s, http.MethodDelete, "/api/targets/pi/files?path=APPEND_SYSTEM.md", "")
	if got := decodeBody[map[string]any](t, rr); rr.Code != http.StatusBadRequest || got["error_code"] != "target_file_builtin" {
		t.Errorf("status %d, body %v", rr.Code, got)
	}
}

func TestTargetFiles_DeleteKeepsFileOnDisk(t *testing.T) {
	s, home := newInstructionsServer(t, "pi")
	file := writeHome(t, home, ".pi/agent/SYSTEM.md", "keep\n")
	targetFilesRequest(t, s, http.MethodPost, "/api/targets/pi/files", `{"path":"SYSTEM.md"}`, http.StatusOK)

	got := targetFilesRequest(t, s, http.MethodDelete, "/api/targets/pi/files?path=SYSTEM.md", "", http.StatusOK)
	if len(got.Files) != 1 || got.Files[0].Path != "APPEND_SYSTEM.md" {
		t.Errorf("list after delete = %+v", got)
	}
	if readFile(t, file) != "keep\n" {
		t.Error("file changed")
	}
	cfg, _ := config.Load()
	if len(cfg.Targets["pi"].Files) != 0 {
		t.Errorf("saved files = %v", cfg.Targets["pi"].Files)
	}
}

func TestTargetFiles_ProjectMode(t *testing.T) {
	s, root := newTestProjectServerWithExtras(t, nil)
	s.projectCfg.Targets = []config.ProjectTargetEntry{{Name: "pi"}}
	s.mu.Lock()
	err := s.saveAndReloadConfig()
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	got := targetFilesRequest(t, s, http.MethodPost, "/api/targets/pi/files", `{"path":"SYSTEM.md"}`, http.StatusOK)
	if !got.Project || got.Root != filepath.Join(root, ".pi") || len(got.Files) != 2 || got.Files[1].Abs != filepath.Join(root, ".pi", "SYSTEM.md") {
		t.Errorf("project list = %+v", got)
	}
	pcfg, err := config.LoadProject(root)
	if err != nil || !slices.Equal(pcfg.Targets[0].Files, []string{"SYSTEM.md"}) {
		t.Errorf("project config = %+v (%v)", pcfg, err)
	}
}
