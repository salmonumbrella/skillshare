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
	managedhooks "skillshare/internal/resources/hooks"
	managedrules "skillshare/internal/resources/rules"
)

func TestHandleSync_MergeMode(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, src, "alpha")

	body := `{"dryRun":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []map[string]any `json:"results"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Results) != 3 {
		t.Fatalf("expected 3 sync results, got %d", len(resp.Results))
	}
	if resp.Results[0]["target"] != "claude" {
		t.Errorf("expected target 'claude', got %v", resp.Results[0]["target"])
	}
	if resp.Results[0]["resource"] != "skills" {
		t.Errorf("expected first resource to be skills, got %v", resp.Results[0]["resource"])
	}
}

func TestHandleSync_IgnoredSkillNotPrunedFromRegistry(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// Create a skill with install metadata (so it appears in registry)
	addSkill(t, src, "kept-skill")
	addSkillMeta(t, src, "kept-skill", "github.com/user/kept")

	// Create another skill that will be ignored
	addSkill(t, src, "ignored-skill")
	addSkillMeta(t, src, "ignored-skill", "github.com/user/ignored")

	// Add .skillignore to exclude the second skill
	os.WriteFile(filepath.Join(src, ".skillignore"), []byte("ignored-skill\n"), 0644)

	// Pre-populate registry with both entries and persist to disk
	// (server auto-reloads registry from disk on each request)
	s.registry = &config.Registry{
		Skills: []config.SkillEntry{
			{Name: "kept-skill", Source: "github.com/user/kept"},
			{Name: "ignored-skill", Source: "github.com/user/ignored"},
		},
	}
	if err := s.registry.Save(s.cfg.RegistryDir); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Run sync (non-dry-run)
	body := `{"dryRun":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Both entries should survive — ignored skill still exists on disk
	if len(s.registry.Skills) != 2 {
		names := make([]string, len(s.registry.Skills))
		for i, sk := range s.registry.Skills {
			names[i] = sk.Name
		}
		t.Fatalf("expected 2 registry entries after sync, got %d: %v", len(s.registry.Skills), names)
	}
}

func TestHandleSync_NoTargets(t *testing.T) {
	s, _ := newTestServer(t) // no targets configured

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []any `json:"results"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results for no targets, got %d", len(resp.Results))
	}
}

func TestHandleSync_InvalidJSONReturnsBadRequest(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, src, "alpha")

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"dryRun":`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Lstat(filepath.Join(tgtPath, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("expected no sync side effects on invalid JSON, got err=%v", err)
	}
}

func TestHandleSync_DefaultsToAllManagedResources(t *testing.T) {
	s, projectRoot, sourceDir, _ := newManagedProjectServer(t, "claude")
	addSkill(t, sourceDir, "alpha")

	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "claude/manual.md",
		Content: []byte("# Managed rule\n"),
	}); err != nil {
		t.Fatalf("put managed rule: %v", err)
	}

	hookStore := managedhooks.NewStore(projectRoot)
	if _, err := hookStore.Put(managedhooks.Save{
		ID:      "claude/pre-tool-use/bash.yaml",
		Tool:    "claude",
		Event:   "PreToolUse",
		Matcher: "Bash",
		Handlers: []managedhooks.Handler{{
			Type:    "command",
			Command: "./bin/check",
		}},
	}); err != nil {
		t.Fatalf("put managed hook: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"dryRun":false}`))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []struct {
			Target   string   `json:"target"`
			Resource string   `json:"resource"`
			Linked   []string `json:"linked"`
			Updated  []string `json:"updated"`
			Skipped  []string `json:"skipped"`
			Pruned   []string `json:"pruned"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Results) != 3 {
		t.Fatalf("expected 3 sync results, got %d: %#v", len(resp.Results), resp.Results)
	}

	byResource := make(map[string]struct {
		Linked  []string
		Updated []string
		Skipped []string
		Pruned  []string
	}, len(resp.Results))
	for _, result := range resp.Results {
		if result.Target != "claude" {
			t.Fatalf("result target = %q, want claude", result.Target)
		}
		byResource[result.Resource] = struct {
			Linked  []string
			Updated []string
			Skipped []string
			Pruned  []string
		}{
			Linked:  result.Linked,
			Updated: result.Updated,
			Skipped: result.Skipped,
			Pruned:  result.Pruned,
		}
	}

	if got := byResource["skills"].Linked; len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("skills linked = %#v, want [alpha]", got)
	}

	if got := byResource["rules"].Updated; len(got) != 1 || got[0] != filepath.Join(projectRoot, ".claude", "rules", "manual.md") {
		t.Fatalf("rules updated = %#v, want compiled rule path", got)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".claude", "rules", "manual.md")); err != nil {
		t.Fatalf("expected synced rule file: %v", err)
	}

	if got := byResource["hooks"].Updated; len(got) != 1 || got[0] != filepath.Join(projectRoot, ".claude", "settings.json") {
		t.Fatalf("hooks updated = %#v, want compiled hook path", got)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".claude", "settings.json")); err != nil {
		t.Fatalf("expected synced hook file: %v", err)
	}
}

func TestHandleSync_HooksOnlyMaterializesEmptyCarrier(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")

	settingsPath := filepath.Join(projectRoot, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("mkdir settings dir: %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"model":"sonnet"}`), 0o644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"resources":["hooks"]}`))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []struct {
			Target   string   `json:"target"`
			Resource string   `json:"resource"`
			Updated  []string `json:"updated"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d: %#v", len(resp.Results), resp.Results)
	}
	if resp.Results[0].Target != "claude" || resp.Results[0].Resource != "hooks" {
		t.Fatalf("result = %#v, want hooks result for claude", resp.Results[0])
	}
	if len(resp.Results[0].Updated) != 1 || resp.Results[0].Updated[0] != settingsPath {
		t.Fatalf("updated = %#v, want %q", resp.Results[0].Updated, settingsPath)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"model":"sonnet"`) {
		t.Fatalf("settings.json = %q, want existing key preserved", content)
	}
	if !strings.Contains(content, `"hooks":{}`) {
		t.Fatalf("settings.json = %q, want empty hooks carrier", content)
	}
}
