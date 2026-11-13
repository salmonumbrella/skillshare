package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func conflictedServerRepo(t *testing.T) (*Server, string, string) {
	t.Helper()
	s, src := newTestServer(t)
	initServerGitRepo(t, src)
	for _, path := range []string{"shared file.md", "removed.md"} {
		if err := os.WriteFile(filepath.Join(src, path), []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, ".metadata.json"), []byte(`{"version":1,"entries":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, src, "add", "-A")
	testutil.RunGit(t, src, "commit", "-m", "base files")
	remote := filepath.Join(t.TempDir(), "remote.git")
	testutil.RunGit(t, "", "init", "--bare", remote)
	testutil.RunGit(t, src, "remote", "add", "origin", remote)
	testutil.RunGit(t, src, "push", "-u", "origin", "HEAD")
	pushRemoteFile(t, remote, "shared file.md", "remote\n")
	pushRemoteFile(t, remote, "removed.md", "remote edit\n")
	pushRemoteFile(t, remote, "remote-only.md", "keep remote\n")
	pushRemoteFile(t, remote, ".metadata.json", `{"version":1,"entries":{"remote-only":{"source":"remote"}}}`)
	if err := os.WriteFile(filepath.Join(src, "shared file.md"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, src, "rm", "removed.md")
	if err := os.WriteFile(filepath.Join(src, "local-only.md"), []byte("keep local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ".metadata.json"), []byte(`{"version":1,"entries":{"local-only":{"source":"local"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, src, "add", "-A")
	testutil.RunGit(t, src, "commit", "-m", "local edits")
	return s, src, remote
}

func pullConflictParams(t *testing.T, s *Server) map[string]any {
	t.Helper()
	rr := postPull(s, `{}`)
	var resp struct {
		Code   string         `json:"error_code"`
		Params map[string]any `json:"error_params"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusConflict || resp.Code != "pull_conflict" {
		t.Fatalf("expected 409 pull_conflict, got %d: %s", rr.Code, rr.Body.String())
	}
	return resp.Params
}

func TestHandlePull_ConflictPreviewLeavesHistoryAndTreeIntact(t *testing.T) {
	s, src, _ := conflictedServerRepo(t)
	head := testutil.RunGit(t, src, "rev-parse", "HEAD")
	params := pullConflictParams(t, s)
	if params["localHash"] != head || params["remoteHash"] == "" {
		t.Fatalf("expected both revision hashes, got %v", params)
	}
	files := params["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("expected both conflict paths, got %v", files)
	}
	for _, raw := range files {
		file := raw.(map[string]any)
		if file["path"] == "shared file.md" && (file["local"].(map[string]any)["content"] != "local\n" || file["remote"].(map[string]any)["content"] != "remote\n") {
			t.Fatalf("expected exact version previews, got %v", file)
		}
	}
	if got := testutil.RunGit(t, src, "status", "--porcelain"); got != "" {
		t.Fatalf("preview left changes: %s", got)
	}
	if got := testutil.RunGit(t, src, "rev-parse", "HEAD"); got != head {
		t.Fatalf("preview moved HEAD: %s", got)
	}
}

func TestHandlePull_ResolveConflictsPreservesBothHistories(t *testing.T) {
	for _, side := range []string{"local", "remote"} {
		t.Run(side, func(t *testing.T) {
			s, src, _ := conflictedServerRepo(t)
			params := pullConflictParams(t, s)
			params["choices"] = map[string]string{"shared file.md": side, "removed.md": side}
			body, _ := json.Marshal(map[string]any{"resolution": params})
			rr := postPull(s, string(body))
			if rr.Code != http.StatusOK {
				t.Fatalf("expected resolved pull, got %d: %s", rr.Code, rr.Body.String())
			}
			if got, _ := os.ReadFile(filepath.Join(src, "shared file.md")); string(got) != side+"\n" {
				t.Fatalf("wrong chosen content: %q", got)
			}
			if _, err := os.Stat(filepath.Join(src, "removed.md")); (side == "local") != os.IsNotExist(err) {
				t.Fatalf("wrong modify/delete choice: %v", err)
			}
			for _, path := range []string{"local-only.md", "remote-only.md"} {
				if _, err := os.Stat(filepath.Join(src, path)); err != nil {
					t.Fatalf("lost non-conflicting file %s: %v", path, err)
				}
			}
			metadata, err := os.ReadFile(filepath.Join(src, ".metadata.json"))
			if err != nil || !strings.Contains(string(metadata), `"local-only"`) || !strings.Contains(string(metadata), `"remote-only"`) {
				t.Fatalf("expected metadata to merge alongside file choices: %s (%v)", metadata, err)
			}
			for _, hash := range []string{params["localHash"].(string), params["remoteHash"].(string)} {
				testutil.RunGit(t, src, "merge-base", "--is-ancestor", hash, "HEAD")
			}
			if got := testutil.RunGit(t, src, "status", "--porcelain"); got != "" {
				t.Fatalf("resolved merge left changes: %s", got)
			}
		})
	}
}

func TestHandlePull_BinaryAndLargeVersionsCanBeChosenWithoutPreview(t *testing.T) {
	for name, content := range map[string]string{"binary": "remote\x00bytes", "large": strings.Repeat("r", 16*1024+1)} {
		t.Run(name, func(t *testing.T) {
			s, src, remote := conflictedServerRepo(t)
			pushRemoteFile(t, remote, "shared file.md", content)
			params := pullConflictParams(t, s)
			for _, raw := range params["files"].([]any) {
				file := raw.(map[string]any)
				if file["path"] == "shared file.md" {
					version := file["remote"].(map[string]any)
					if version["noPreview"] != true || version["content"] != "" || version["deleted"] != false {
						t.Fatalf("expected an existing version without preview, got %v", version)
					}
				}
			}
			params["choices"] = map[string]string{"shared file.md": "remote", "removed.md": "local"}
			body, _ := json.Marshal(map[string]any{"resolution": params})
			if rr := postPull(s, string(body)); rr.Code != http.StatusOK {
				t.Fatalf("expected chosen version to apply, got %d: %s", rr.Code, rr.Body.String())
			}
			if got, _ := os.ReadFile(filepath.Join(src, "shared file.md")); string(got) != content {
				t.Fatalf("chosen bytes did not match")
			}
		})
	}
}

func TestHandlePull_RejectsStaleOrIncompleteChoices(t *testing.T) {
	for _, kind := range []string{"remote-changed", "missing", "extra", "invalid"} {
		t.Run(kind, func(t *testing.T) {
			s, src, remote := conflictedServerRepo(t)
			params := pullConflictParams(t, s)
			choices := map[string]string{"shared file.md": "remote", "removed.md": "local"}
			switch kind {
			case "remote-changed":
				pushRemoteFile(t, remote, "shared file.md", "new remote\n")
			case "missing":
				delete(choices, "removed.md")
			case "extra":
				choices["../outside"] = "remote"
			case "invalid":
				choices["shared file.md"] = "other"
			}
			params["choices"] = choices
			body, _ := json.Marshal(map[string]any{"resolution": params})
			if rr := postPull(s, string(body)); rr.Code != http.StatusConflict {
				t.Fatalf("expected fresh conflict review, got %d: %s", rr.Code, rr.Body.String())
			}
			if got := testutil.RunGit(t, src, "rev-parse", "HEAD"); got != params["localHash"] {
				t.Fatalf("rejected choices moved HEAD: %s", got)
			}
			if got := testutil.RunGit(t, src, "status", "--porcelain"); got != "" {
				t.Fatalf("rejected choices left changes: %s", got)
			}
		})
	}
}
