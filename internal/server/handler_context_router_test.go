package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/contextrouter"
)

type contextRouterResponse struct {
	Roots    []contextrouter.Root  `json:"roots"`
	Selected string                `json:"selected"`
	Router   *contextrouter.Report `json:"router"`
}

// getContextRouter goes through the served handler, middleware included.
func getContextRouter(t *testing.T, s *Server, root string, wantStatus int) contextRouterResponse {
	t.Helper()
	target := "/api/context/router"
	if root != "" {
		target += "?root=" + url.QueryEscape(root)
	}
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, target, nil))
	if rr.Code != wantStatus {
		t.Fatalf("GET %s: got %d, want %d: %s", target, rr.Code, wantStatus, rr.Body)
	}
	var resp contextRouterResponse
	if wantStatus == http.StatusOK {
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode %s: %v", rr.Body, err)
		}
	}
	return resp
}

// addRouter writes a minimal router into dir and returns its physical path.
func addRouter(t *testing.T, dir, kernel string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "wiki"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"AGENTS.md":            kernel,
		"wiki/page.md":         "# Page\nbody\n",
		"wiki/ai-context.json": `{"topics":{"page":{"description":"A page.","sources":[{"path":"wiki/page.md"}]}}}`,
	}
	for rel, text := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestContextRouter_GlobalNothingFound(t *testing.T) {
	s, _ := newTestServer(t)
	resp := getContextRouter(t, s, "", http.StatusOK)
	if resp.Roots == nil || len(resp.Roots) != 0 || resp.Selected != "" || resp.Router != nil {
		t.Fatalf("got %+v", resp)
	}
}

// The tab must not rewrite the user's configuration: the ordinary reload migrates a legacy one.
func TestContextRouter_ReadOnlyLegacyLoad(t *testing.T) {
	s, source := newTestServer(t)
	path := config.ConfigPath()
	raw := []byte("source: " + filepath.ToSlash(source) + "\nmode: merge\ntargets:\n  claude:\n    path: " + filepath.ToSlash(filepath.Join(filepath.Dir(source), "target")) + "\n")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	s.cfg = cfg
	getContextRouter(t, s, "", http.StatusOK)
	if got, _ := os.ReadFile(path); !bytes.Equal(got, raw) {
		t.Fatal("opening the Router tab migrated user configuration")
	}
	if _, err := os.Stat(filepath.Join(config.StateDir(), "projects")); !os.IsNotExist(err) {
		t.Fatalf("read created mutation lock: %v", err)
	}
}

func TestContextRouter_SourceAncestor(t *testing.T) {
	s, source := newTestServer(t)
	kernel := "# Kernel\n\n- `page`\n"
	root := addRouter(t, filepath.Dir(source), kernel)
	resp := getContextRouter(t, s, "", http.StatusOK)
	if len(resp.Roots) != 1 || resp.Roots[0].Path != root || resp.Roots[0].Origin != contextrouter.OriginSkillsSource || resp.Selected != root {
		t.Fatalf("roots: %+v selected %q", resp.Roots, resp.Selected)
	}
	if r := resp.Router; r == nil || r.Kernel == nil || r.Kernel.Bytes == nil || *r.Kernel.Bytes != len(kernel) || len(r.Topics) != 1 || r.Topics[0].Bytes == nil {
		t.Fatalf("router: %+v", resp.Router)
	}
}

func TestContextRouter_RegisteredProject(t *testing.T) {
	s, _ := newTestServer(t)
	project := addRouter(t, t.TempDir(), "`page`")
	addProject(t, s, project)
	resp := getContextRouter(t, s, project, http.StatusOK)
	if len(resp.Roots) != 1 || resp.Roots[0].Path != project || resp.Roots[0].Origin != contextrouter.OriginProject || resp.Selected != project || resp.Router == nil || resp.Router.Root != project {
		t.Fatalf("got %+v", resp)
	}
}

// ?root= chooses among the discovered folders; it is not a path to read.
func TestContextRouter_UnknownRoot(t *testing.T) {
	s, _ := newTestServer(t)
	getContextRouter(t, s, "/nope", http.StatusNotFound)
	unregistered := addRouter(t, t.TempDir(), "`page`")
	getContextRouter(t, s, unregistered, http.StatusNotFound)
}

func TestContextRouter_ProjectMode(t *testing.T) {
	projectRoot := addRouter(t, t.TempDir(), "`page`")
	src := filepath.Join(projectRoot, ".skillshare", "skills")
	addSkill(t, src, "alpha")
	projectCfg := &config.ProjectConfig{Targets: []config.ProjectTargetEntry{{Name: "claude"}}}
	if err := projectCfg.Save(projectRoot); err != nil {
		t.Fatal(err)
	}
	other := addRouter(t, t.TempDir(), "`page`")
	global := &config.Config{Source: src, Mode: "merge", Projects: map[string]config.ManagedProject{other: {}}}
	s := NewProject(global, projectCfg, projectRoot, "127.0.0.1:0", "", "")

	resp := getContextRouter(t, s, "", http.StatusOK)
	if len(resp.Roots) != 1 || resp.Roots[0].Path != projectRoot || resp.Roots[0].Origin != contextrouter.OriginCurrent {
		t.Fatalf("roots: %+v", resp.Roots)
	}
	getContextRouter(t, s, other, http.StatusNotFound)
}

func TestContextRouter_InvalidConfig(t *testing.T) {
	s, source := newTestServer(t)
	root := addRouter(t, filepath.Dir(source), "`page`")
	if err := os.WriteFile(filepath.Join(root, "wiki", "ai-context.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := getContextRouter(t, s, "", http.StatusOK)
	if len(resp.Roots) != 1 || resp.Router == nil || resp.Router.Error == nil || resp.Router.Error.Code != contextrouter.ConfigInvalid || resp.Router.Kernel != nil {
		t.Fatalf("got roots %+v router %+v", resp.Roots, resp.Router)
	}
}

// The report says which version the folder's own script is and whose size rule was applied.
func TestContextRouter_ScriptVersion(t *testing.T) {
	s, source := newTestServer(t)
	root := addRouter(t, filepath.Dir(source), "`page`")
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/context/router", nil))
	var raw struct {
		Router map[string]any `json:"router"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if raw.Router["scriptVersion"] != "" || raw.Router["semanticsVersion"] != contextrouter.SemanticsVersion || raw.Router["versionStatus"] != contextrouter.VersionUnknown {
		t.Fatalf("no script: %v", raw.Router)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "ai-context.py"), []byte("VERSION = '0.2.1'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := getContextRouter(t, s, "", http.StatusOK)
	if resp.Router.ScriptVersion != "0.2.1" || resp.Router.SemanticsVersion != "0.2.1" || resp.Router.VersionStatus != contextrouter.VersionLegacy {
		t.Fatalf("0.2.1 script: %+v", resp.Router)
	}
}

// Without a home folder there is nothing to stop the walk above a source, so
// it is not made; a registered project is still found.
func TestContextRouter_NoHomeFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the home folder comes from USERPROFILE on Windows")
	}
	s, source := newTestServer(t)
	addRouter(t, filepath.Dir(source), "`page`")
	project := addRouter(t, t.TempDir(), "`page`")
	addProject(t, s, project)
	if resp := getContextRouter(t, s, "", http.StatusOK); len(resp.Roots) != 2 {
		t.Fatalf("with a home folder: %+v", resp.Roots)
	}
	t.Setenv("HOME", "")
	if _, err := os.UserHomeDir(); err == nil {
		t.Skip("a home folder is known without HOME")
	}
	resp := getContextRouter(t, s, "", http.StatusOK)
	if len(resp.Roots) != 1 || resp.Roots[0].Path != project || resp.Roots[0].Origin != contextrouter.OriginProject {
		t.Fatalf("without a home folder: %+v", resp.Roots)
	}
}

// With several routers, only the selected folder is measured. The others are
// listed from the presence of their config alone: here they cannot even be read.
func TestContextRouter_MeasuresOnlyTheSelectedFolder(t *testing.T) {
	s, _ := newTestServer(t)
	var roots []string
	for i := 0; i < 3; i++ {
		root := addRouter(t, t.TempDir(), "`page`")
		addProject(t, s, root)
		roots = append(roots, root)
	}
	selected := roots[1]
	for _, root := range roots {
		if root == selected {
			continue
		}
		// A directory where the config should be: present, so the folder is listed, and unreadable as a file.
		config := filepath.Join(root, "wiki", "ai-context.json")
		if err := os.Remove(config); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(config, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var measured []string
	original := measureContextRouter
	measureContextRouter = func(ctx context.Context, root string, lim contextrouter.Limits) (contextrouter.Report, error) {
		measured = append(measured, root)
		return original(ctx, root, lim)
	}
	t.Cleanup(func() { measureContextRouter = original })

	resp := getContextRouter(t, s, selected, http.StatusOK)
	if len(resp.Roots) != 3 || resp.Selected != selected || resp.Router == nil || resp.Router.Error != nil || resp.Router.Kernel == nil {
		t.Fatalf("got roots %+v, router %+v", resp.Roots, resp.Router)
	}
	if len(measured) != 1 || measured[0] != selected {
		t.Fatalf("measured %v, want only %s", measured, selected)
	}
}
