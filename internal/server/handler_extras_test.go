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
	"skillshare/internal/oplog"
)

// newTestServerWithExtras creates a test server with pre-configured extras
// persisted in the config file on disk.
// It creates source directories so handlers don't fail on "source not found".
func newTestServerWithExtras(t *testing.T, extras []config.ExtraConfig, extrasSource string) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "skills")
	os.MkdirAll(sourceDir, 0755)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	os.MkdirAll(filepath.Dir(cfgPath), 0755)

	// Build a config with extras and save it, so the auto-reload middleware
	// can read extras back from disk.
	raw := "source: " + sourceDir + "\nmode: merge\ntargets: {}\n"
	if extrasSource != "" {
		raw += "extras_source: " + extrasSource + "\n"
	}
	os.WriteFile(cfgPath, []byte(raw), 0644)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	cfg.Extras = extras

	// Persist extras to disk so auto-reload picks them up.
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save config with extras: %v", err)
	}

	// Create source directories for each extra so DiscoverExtraFiles succeeds.
	for _, extra := range extras {
		dir := config.ResolveExtrasSourceDir(extra, extrasSource, sourceDir)
		os.MkdirAll(dir, 0755)
	}

	s := New(cfg, "127.0.0.1:0", "", "")
	return s, sourceDir
}

// newTestProjectServerWithExtras creates a project-mode test server with extras.
func newTestProjectServerWithExtras(t *testing.T, extras []config.ExtraConfig) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()

	// Global config (required by server)
	sourceDir := filepath.Join(tmp, "skills")
	os.MkdirAll(sourceDir, 0755)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	os.WriteFile(cfgPath, []byte("source: "+sourceDir+"\nmode: merge\ntargets: {}\n"), 0644)

	globalCfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load global config: %v", err)
	}

	// Project root with .skillshare/config.yaml
	projectRoot := filepath.Join(tmp, "project")
	projectCfgDir := filepath.Join(projectRoot, ".skillshare")
	os.MkdirAll(projectCfgDir, 0755)

	projectCfg := &config.ProjectConfig{
		Extras: extras,
	}
	if err := projectCfg.Save(projectRoot); err != nil {
		t.Fatalf("failed to save project config: %v", err)
	}

	// Create source directories for project extras.
	for _, extra := range extras {
		dir := config.ResolveExtrasSourceDirProject(extra, projectCfg.EffectiveExtrasSource(projectRoot), projectRoot)
		os.MkdirAll(dir, 0755)
	}

	s := NewProject(globalCfg, projectCfg, projectRoot, "127.0.0.1:0", "", "")
	return s, projectRoot
}

// TestHandleExtras_IncludesSourceType verifies GET /api/extras returns
// the source_type field with correct values based on per-extra source
// and global extras_source settings.
func TestHandleExtras_IncludesSourceType(t *testing.T) {
	customSourceDir := filepath.Join(t.TempDir(), "custom-rules")

	extras := []config.ExtraConfig{
		{
			Name:   "rules",
			Source: customSourceDir,
			Targets: []config.ExtraTargetConfig{
				{Path: t.TempDir(), Mode: "merge"},
			},
		},
		{
			Name: "commands",
			// No per-extra source — should fall back to default
			Targets: []config.ExtraTargetConfig{
				{Path: t.TempDir(), Mode: "merge"},
			},
		},
	}

	s, _ := newTestServerWithExtras(t, extras, "")

	req := httptest.NewRequest(http.MethodGet, "/api/extras", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Extras []struct {
			Name       string `json:"name"`
			SourceType string `json:"source_type"`
		} `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Extras) != 2 {
		t.Fatalf("expected 2 extras, got %d", len(resp.Extras))
	}

	// "rules" has per-extra Source set -> "per-extra"
	if resp.Extras[0].Name != "rules" {
		t.Errorf("expected first extra name 'rules', got %q", resp.Extras[0].Name)
	}
	if resp.Extras[0].SourceType != "per-extra" {
		t.Errorf("expected source_type 'per-extra' for rules, got %q", resp.Extras[0].SourceType)
	}

	// "commands" has no per-extra Source and no extras_source -> "default"
	if resp.Extras[1].Name != "commands" {
		t.Errorf("expected second extra name 'commands', got %q", resp.Extras[1].Name)
	}
	if resp.Extras[1].SourceType != "default" {
		t.Errorf("expected source_type 'default' for commands, got %q", resp.Extras[1].SourceType)
	}
}

// TestHandleExtras_ExtensionTargetUsesCopyMode verifies that a target carrying a
// transform extension with an empty mode is reported with copy semantics, not the
// generic merge default. Previously EffectiveMode("") returned "merge" for these
// targets, so a valid extension config always looked like drift in the dashboard.
func TestHandleExtras_ExtensionTargetUsesCopyMode(t *testing.T) {
	extras := []config.ExtraConfig{
		{
			Name: "rules",
			Targets: []config.ExtraTargetConfig{
				// Extension set, mode left empty — must resolve to copy.
				{Path: filepath.Join(t.TempDir(), "nope"), Extension: "gemini-commands"},
				// Plain target, mode left empty — must stay merge.
				{Path: filepath.Join(t.TempDir(), "nope2")},
			},
		},
	}

	s, _ := newTestServerWithExtras(t, extras, "")

	req := httptest.NewRequest(http.MethodGet, "/api/extras", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Extras []struct {
			Name    string `json:"name"`
			Targets []struct {
				Mode      string `json:"mode"`
				Extension string `json:"extension"`
				Status    string `json:"status"`
			} `json:"targets"`
		} `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Extras) != 1 || len(resp.Extras[0].Targets) != 2 {
		t.Fatalf("unexpected shape: %+v", resp.Extras)
	}

	ext := resp.Extras[0].Targets[0]
	if ext.Mode != "copy" {
		t.Errorf("extension target: mode = %q, want copy", ext.Mode)
	}
	if ext.Status == "drift" {
		t.Errorf("extension target with empty mode should not report drift, got %q", ext.Status)
	}

	plain := resp.Extras[0].Targets[1]
	if plain.Mode != "merge" {
		t.Errorf("plain target: mode = %q, want merge", plain.Mode)
	}
}

// TestHandleExtras_IncludesSourceType_ExtrasSource verifies that when
// extras_source is set globally and the extra has no per-extra source,
// source_type is "extras_source".
func TestHandleExtras_IncludesSourceType_ExtrasSource(t *testing.T) {
	extrasSourceDir := filepath.Join(t.TempDir(), "shared-extras")

	extras := []config.ExtraConfig{
		{
			Name: "hooks",
			Targets: []config.ExtraTargetConfig{
				{Path: t.TempDir(), Mode: "merge"},
			},
		},
	}

	s, _ := newTestServerWithExtras(t, extras, extrasSourceDir)

	req := httptest.NewRequest(http.MethodGet, "/api/extras", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Extras []struct {
			Name       string `json:"name"`
			SourceType string `json:"source_type"`
		} `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Extras) != 1 {
		t.Fatalf("expected 1 extra, got %d", len(resp.Extras))
	}

	if resp.Extras[0].SourceType != "extras_source" {
		t.Errorf("expected source_type 'extras_source', got %q", resp.Extras[0].SourceType)
	}
}

// TestHandleExtrasCreate_WithSource verifies POST /api/extras accepts
// a "source" field and persists it in the created ExtraConfig.
func TestHandleExtrasCreate_WithSource(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")

	customSource := filepath.Join(t.TempDir(), "custom-src")
	targetDir := t.TempDir()
	body := `{"name":"rules","source":"` + customSource + `","targets":[{"path":"` + targetDir + `","mode":"merge"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool   `json:"success"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}

	// Verify the extra was added to config with Source set.
	s.mu.RLock()
	extras := s.cfg.Extras
	s.mu.RUnlock()

	found := false
	for _, e := range extras {
		if e.Name == "rules" {
			found = true
			if e.Source != customSource {
				t.Errorf("expected Source %q, got %q", customSource, e.Source)
			}
			break
		}
	}
	if !found {
		t.Error("extra 'rules' not found in config after create")
	}
}

// TestHandleExtrasCreate_WithExtension verifies that creating an extra with a
// target extension persists the extension and forces the target into copy mode,
// even when the request asks for a different mode.
func TestHandleExtrasCreate_WithExtension(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")

	targetDir := t.TempDir()
	// Request mode "merge" but with an extension — should be coerced to copy.
	body := `{"name":"agents","targets":[{"path":"` + targetDir + `","mode":"merge","extension":"codex-agents"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	s.mu.RLock()
	extras := s.cfg.Extras
	s.mu.RUnlock()

	var found bool
	for _, e := range extras {
		if e.Name != "agents" {
			continue
		}
		found = true
		if len(e.Targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(e.Targets))
		}
		tt := e.Targets[0]
		if tt.Extension != "codex-agents" {
			t.Errorf("extension = %q, want codex-agents", tt.Extension)
		}
		if tt.Mode != "copy" {
			t.Errorf("mode = %q, want copy (extension implies copy)", tt.Mode)
		}
	}
	if !found {
		t.Error("extra 'agents' not found in config after create")
	}
}

// TestHandleExtrasCreate_WithoutSource verifies POST /api/extras works
// without a "source" field (backward compatibility).
func TestHandleExtrasCreate_WithoutSource(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")

	targetDir := t.TempDir()
	body := `{"name":"commands","targets":[{"path":"` + targetDir + `","mode":"merge"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}

	// Verify the extra was added with empty Source.
	s.mu.RLock()
	extras := s.cfg.Extras
	s.mu.RUnlock()

	found := false
	for _, e := range extras {
		if e.Name == "commands" {
			found = true
			if e.Source != "" {
				t.Errorf("expected empty Source, got %q", e.Source)
			}
			break
		}
	}
	if !found {
		t.Error("extra 'commands' not found in config after create")
	}
}

// TestHandleExtras_ProjectMode_DefaultSourceType verifies that in project
// mode, extras without per-extra Source have source_type "default", since
// project mode does not use extras_source. An extra with Source set will
// still report "per-extra" because ResolveExtrasSourceType checks that first.
func TestHandleExtras_ProjectMode_DefaultSourceType(t *testing.T) {
	extras := []config.ExtraConfig{
		{
			Name: "rules",
			// No per-extra source -> "default" in project mode
			Targets: []config.ExtraTargetConfig{
				{Path: t.TempDir(), Mode: "merge"},
			},
		},
		{
			Name: "hooks",
			Targets: []config.ExtraTargetConfig{
				{Path: t.TempDir(), Mode: "merge"},
			},
		},
	}

	s, _ := newTestProjectServerWithExtras(t, extras)

	req := httptest.NewRequest(http.MethodGet, "/api/extras", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Extras []struct {
			Name       string `json:"name"`
			SourceType string `json:"source_type"`
		} `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Extras) != 2 {
		t.Fatalf("expected 2 extras, got %d", len(resp.Extras))
	}

	for _, entry := range resp.Extras {
		if entry.SourceType != "default" {
			t.Errorf("expected source_type 'default' for %q in project mode, got %q", entry.Name, entry.SourceType)
		}
	}
}

// writeExtension creates a directory-form extension under dir/<name>/.
func writeExtension(t *testing.T, dir, name string) {
	t.Helper()
	extDir := filepath.Join(dir, name)
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"), []byte("run: [\"cat\"]\noutput_ext: toml\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestHandleExtrasExtensions verifies GET /api/extras/extensions lists the
// extensions installed under the global extensions directory.
func TestHandleExtrasExtensions(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")

	extRoot := filepath.Join(filepath.Dir(config.ConfigPath()), "extensions")
	writeExtension(t, extRoot, "codex-agents")
	writeExtension(t, extRoot, "gemini-commands")

	req := httptest.NewRequest(http.MethodGet, "/api/extras/extensions", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Extensions []string `json:"extensions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp.Extensions) != 2 || resp.Extensions[0] != "codex-agents" || resp.Extensions[1] != "gemini-commands" {
		t.Errorf("got %v, want [codex-agents gemini-commands]", resp.Extensions)
	}
}

// TestHandleExtrasExtensions_ProjectMode verifies the endpoint reads the
// project extensions directory (.skillshare/extensions) in project mode.
func TestHandleExtrasExtensions_ProjectMode(t *testing.T) {
	s, projectRoot := newTestProjectServerWithExtras(t, nil)

	extRoot := filepath.Join(projectRoot, ".skillshare", "extensions")
	writeExtension(t, extRoot, "codex-agents")

	req := httptest.NewRequest(http.MethodGet, "/api/extras/extensions", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Extensions []string `json:"extensions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp.Extensions) != 1 || resp.Extensions[0] != "codex-agents" {
		t.Errorf("got %v, want [codex-agents]", resp.Extensions)
	}
}

// TestHandleExtrasMode_ExtensionImpliesCopy verifies that setting an extension
// on a target persists it and forces the mode to copy.
func TestHandleExtrasMode_ExtensionImpliesCopy(t *testing.T) {
	targetDir := t.TempDir()
	extras := []config.ExtraConfig{{
		Name:    "agents",
		Targets: []config.ExtraTargetConfig{{Path: targetDir, Mode: "merge"}},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	body := `{"target":"` + targetDir + `","extension":"codex-agents"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/extras/agents/mode", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	s.mu.RLock()
	tt := s.cfg.Extras[0].Targets[0]
	s.mu.RUnlock()
	if tt.Extension != "codex-agents" {
		t.Errorf("extension = %q, want codex-agents", tt.Extension)
	}
	if tt.Mode != "copy" {
		t.Errorf("mode = %q, want copy (extension implies copy)", tt.Mode)
	}
}

// TestHandleExtrasMode_ExtensionRejectsNonCopy verifies that an explicit
// non-copy mode combined with an extension is rejected.
func TestHandleExtrasMode_ExtensionRejectsNonCopy(t *testing.T) {
	targetDir := t.TempDir()
	extras := []config.ExtraConfig{{
		Name:    "agents",
		Targets: []config.ExtraTargetConfig{{Path: targetDir, Mode: "merge"}},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	body := `{"target":"` + targetDir + `","mode":"symlink","extension":"codex-agents"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/extras/agents/mode", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatalf("expected non-200 for extension + symlink mode, got 200")
	}
}

// TestHandleExtras_IncludesExtension verifies GET /api/extras surfaces the
// per-target extension value.
func TestHandleExtras_IncludesExtension(t *testing.T) {
	targetDir := t.TempDir()
	extras := []config.ExtraConfig{{
		Name:    "agents",
		Targets: []config.ExtraTargetConfig{{Path: targetDir, Mode: "copy", Extension: "codex-agents"}},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	req := httptest.NewRequest(http.MethodGet, "/api/extras", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Extras []struct {
			Targets []struct {
				Extension string `json:"extension"`
			} `json:"targets"`
		} `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp.Extras) != 1 || len(resp.Extras[0].Targets) != 1 {
		t.Fatalf("unexpected shape: %s", rr.Body.String())
	}
	if resp.Extras[0].Targets[0].Extension != "codex-agents" {
		t.Errorf("extension = %q, want codex-agents", resp.Extras[0].Targets[0].Extension)
	}
}

// TestHandleExtrasCreate_PreservesEmptyExtrasSource verifies that POST /api/extras
// does NOT silently write the legacy extras_source field when the user has
// not explicitly set it. The runtime fallback is provided by
// Config.EffectiveExtrasSource(), so backfilling on save would surprise
// users with an unexpected diff in their config.yaml.
func TestHandleExtrasCreate_PreservesEmptyExtrasSource(t *testing.T) {
	// Create server with NO extras_source set
	s, sourceDir := newTestServerWithExtras(t, nil, "")

	// Verify extras_source is empty before create
	if s.cfg.ExtrasSource != "" {
		t.Fatalf("precondition: expected empty ExtrasSource, got %s", s.cfg.ExtrasSource)
	}

	targetDir := t.TempDir()
	body := `{"name":"rules","targets":[{"path":"` + targetDir + `","mode":"merge"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Reload config from disk and verify extras_source is still empty —
	// no silent backfill — and that EffectiveExtrasSource() resolves to
	// the derived default.
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if cfg.ExtrasSource != "" {
		t.Errorf("expected empty ExtrasSource (no silent backfill), got %q", cfg.ExtrasSource)
	}
	if cfg.Sources.Extras != "" {
		t.Errorf("expected empty Sources.Extras (no silent backfill), got %q", cfg.Sources.Extras)
	}
	expected := config.ExtrasParentDir(sourceDir)
	if got := cfg.EffectiveExtrasSource(); got != expected {
		t.Errorf("expected EffectiveExtrasSource %q (derived), got %q", expected, got)
	}
}

// TestHandleExtrasSync_AppliesExtension is a regression test: the web UI sync
// endpoint must resolve a target's transform extension and apply it, instead of
// copying files verbatim. The transform renames .md output to the spec's
// output_ext, so seeing api-architect.toml (and no .md) proves the spec ran.
func TestHandleExtrasSync_AppliesExtension(t *testing.T) {
	targetDir := t.TempDir()
	extras := []config.ExtraConfig{{
		Name:    "agents",
		Targets: []config.ExtraTargetConfig{{Path: targetDir, Mode: "copy", Extension: "to-toml"}},
	}}
	s, sourceDir := newTestServerWithExtras(t, extras, "")

	// Install a transform extension that pipes through cat (identity) and
	// renames output to .toml.
	extRoot := filepath.Join(filepath.Dir(config.ConfigPath()), "extensions")
	writeExtensionWithDescription(t, extRoot, "to-toml", "identity to toml")

	// Seed a markdown source file in the extra's source directory.
	srcDir := config.ResolveExtrasSourceDir(extras[0], "", sourceDir)
	if err := os.WriteFile(filepath.Join(srcDir, "api-architect.md"), []byte("# Agent\n"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/extras/sync",
		strings.NewReader(`{"name":"agents","force":true}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(filepath.Join(targetDir, "api-architect.toml")); err != nil {
		t.Errorf("expected transformed api-architect.toml in target, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "api-architect.md")); err == nil {
		t.Error("api-architect.md was copied verbatim — extension transform was not applied")
	}
}

func TestHandleExtrasSync_ProjectRelativeTargetResolvesProjectRoot(t *testing.T) {
	extras := []config.ExtraConfig{{
		Name: "rules",
		Targets: []config.ExtraTargetConfig{
			{Path: ".cursor/rules", Mode: "copy"},
		},
	}}
	s, projectRoot := newTestProjectServerWithExtras(t, extras)

	sourceDir := filepath.Join(projectRoot, ".skillshare", "extras", "rules")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "rule.md"), []byte("rule"), 0644); err != nil {
		t.Fatal(err)
	}
	otherCWD := t.TempDir()
	oldCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(otherCWD); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldCWD) })

	req := httptest.NewRequest(http.MethodPost, "/api/extras/sync", strings.NewReader(`{"name":"rules","force":true}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".cursor", "rules", "rule.md")); err != nil {
		t.Fatalf("expected project target file, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(otherCWD, ".cursor", "rules", "rule.md")); !os.IsNotExist(err) {
		t.Fatalf("server cwd target should not be written, stat err = %v", err)
	}
}

// postExtrasSync posts body to /api/extras/sync and decodes the response.
func postExtrasSync(t *testing.T, s *Server, body string) []map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/extras/sync", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Extras []struct {
			Targets []map[string]any `json:"targets"`
		} `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Extras) != 1 {
		t.Fatalf("expected 1 extra, got %d: %s", len(resp.Extras), rr.Body.String())
	}
	return resp.Extras[0].Targets
}

// Like the CLI, the server skips an "agents" extra target that the agents sync
// already manages.
func TestHandleExtrasSync_SkipsAgentOverlapTarget(t *testing.T) {
	agentsTarget := t.TempDir()
	extras := []config.ExtraConfig{{
		Name:    "agents",
		Targets: []config.ExtraTargetConfig{{Path: agentsTarget, Mode: "copy"}},
	}}
	s, sourceDir := newTestServerWithExtras(t, extras, "")

	agentsSource := filepath.Join(filepath.Dir(sourceDir), "agents")
	os.MkdirAll(agentsSource, 0755)
	os.WriteFile(filepath.Join(agentsSource, "helper.md"), []byte("# Helper"), 0644)
	s.cfg.AgentsSource = agentsSource
	s.cfg.Targets = map[string]config.TargetConfig{"claude": {
		Skills: &config.ResourceTargetConfig{Path: t.TempDir()},
		Agents: &config.ResourceTargetConfig{Path: agentsTarget},
	}}
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}
	srcDir := config.ResolveExtrasSourceDir(extras[0], "", sourceDir)
	os.WriteFile(filepath.Join(srcDir, "extra-agent.md"), []byte("# Extra"), 0644)

	targets := postExtrasSync(t, s, `{"name":"agents"}`)

	if got := targets[0]["skipped_by"]; got != "agents" {
		t.Errorf("skipped_by = %v, want %q (full entry %v)", got, "agents", targets[0])
	}
	if _, err := os.Stat(filepath.Join(agentsTarget, "extra-agent.md")); !os.IsNotExist(err) {
		t.Errorf("extra-agent.md must not be synced into the agents target, stat err = %v", err)
	}
}

// In project mode, a relative agents target path resolves against the project
// root, so the "agents" extra target at the same relative path is skipped.
func TestHandleExtrasSync_SkipsAgentOverlapTarget_ProjectMode(t *testing.T) {
	extras := []config.ExtraConfig{{
		Name:    "agents",
		Targets: []config.ExtraTargetConfig{{Path: ".claude/agents", Mode: "copy"}},
	}}
	s, projectRoot := newTestProjectServerWithExtras(t, extras)

	agentsSource := s.projectCfg.EffectiveAgentsSource(projectRoot)
	os.MkdirAll(agentsSource, 0755)
	os.WriteFile(filepath.Join(agentsSource, "helper.md"), []byte("# Helper"), 0644)
	s.projectCfg.Targets = []config.ProjectTargetEntry{{
		Name:   "claude",
		Agents: &config.ResourceTargetConfig{Path: ".claude/agents"},
	}}
	s.mu.Lock()
	err := s.saveAndReloadConfig()
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	srcDir := config.ResolveExtrasSourceDirProject(extras[0], s.projectCfg.EffectiveExtrasSource(projectRoot), projectRoot)
	os.WriteFile(filepath.Join(srcDir, "extra-agent.md"), []byte("# Extra"), 0644)

	targets := postExtrasSync(t, s, `{"name":"agents"}`)

	if got := targets[0]["skipped_by"]; got != "agents" {
		t.Errorf("skipped_by = %v, want %q (full entry %v)", got, "agents", targets[0])
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".claude", "agents", "extra-agent.md")); !os.IsNotExist(err) {
		t.Errorf("extra-agent.md must not be synced into the agents target, stat err = %v", err)
	}
}

// The server skips an extra whose source directory is missing, with a hint in
// the extra's warnings, instead of creating it.
func TestHandleExtrasSync_SkipsMissingSource(t *testing.T) {
	extras := []config.ExtraConfig{{
		Name:    "rules",
		Targets: []config.ExtraTargetConfig{{Path: t.TempDir()}},
	}}
	s, sourceDir := newTestServerWithExtras(t, extras, "")
	srcDir := config.ResolveExtrasSourceDir(extras[0], "", sourceDir)
	if err := os.RemoveAll(srcDir); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/extras/sync", strings.NewReader(`{"name":"rules"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Extras []struct {
			Targets  []map[string]any `json:"targets"`
			Warnings []string         `json:"warnings"`
		} `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Extras) != 1 || len(resp.Extras[0].Targets) != 0 {
		t.Fatalf("expected the extra with no targets, got %s", rr.Body.String())
	}
	if w := resp.Extras[0].Warnings; len(w) != 1 || !strings.Contains(w[0], "Source directory does not exist") {
		t.Errorf("expected a missing-source hint in warnings, got %v", w)
	}
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Errorf("server must not create %s, stat err = %v", srcDir, err)
	}
}

// Characterization: the server reports each target under "target" with the
// path as configured, not the resolved path.
func TestHandleExtrasSync_ReportsRawPathUnderTargetKey(t *testing.T) {
	extras := []config.ExtraConfig{{
		Name:    "rules",
		Targets: []config.ExtraTargetConfig{{Path: ".cursor/rules", Mode: "copy"}},
	}}
	s, _ := newTestProjectServerWithExtras(t, extras)

	targets := postExtrasSync(t, s, `{"name":"rules"}`)

	if got := targets[0]["target"]; got != ".cursor/rules" {
		t.Errorf("target = %v, want %q (full entry %v)", got, ".cursor/rules", targets[0])
	}
}

// latestOpsEntry returns the newest operations log entry.
func latestOpsEntry(t *testing.T, s *Server) oplog.Entry {
	t.Helper()
	entries, err := oplog.Read(s.configPath(), oplog.OpsFile, 10)
	if err != nil || len(entries) == 0 {
		t.Fatalf("read ops log: %v (entries %d)", err, len(entries))
	}
	return entries[0]
}

// When every target fails, the sync still answers 200 but logs status
// "error" with the failure count; extension errors carry an
// "extension <name>: " prefix.
func TestHandleExtrasSync_AllTargetsFailedLogsError(t *testing.T) {
	extras := []config.ExtraConfig{{
		Name:    "rules",
		Targets: []config.ExtraTargetConfig{{Path: t.TempDir(), Extension: "missing-ext"}},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	targets := postExtrasSync(t, s, `{"name":"rules"}`)

	if msg, _ := targets[0]["error"].(string); !strings.HasPrefix(msg, "extension missing-ext: ") {
		t.Errorf("error = %q, want prefix %q", msg, "extension missing-ext: ")
	}
	e := latestOpsEntry(t, s)
	if e.Command != "extras-sync" || e.Status != "error" || e.Args["errors"] != float64(1) {
		t.Errorf("latest oplog entry = %s/%s errors=%v, want extras-sync/error errors=1", e.Command, e.Status, e.Args["errors"])
	}
}

// When only some targets fail, the sync logs status "partial".
func TestHandleExtrasSync_SomeTargetsFailedLogsPartial(t *testing.T) {
	extras := []config.ExtraConfig{{
		Name: "rules",
		Targets: []config.ExtraTargetConfig{
			{Path: t.TempDir(), Extension: "missing-ext"},
			{Path: t.TempDir(), Mode: "copy"},
		},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	postExtrasSync(t, s, `{"name":"rules"}`)

	if e := latestOpsEntry(t, s); e.Status != "partial" || e.Args["errors"] != float64(1) {
		t.Errorf("latest oplog entry = %s errors=%v, want partial errors=1", e.Status, e.Args["errors"])
	}
}

// After a .md→.toml transform sync, /api/extras/diff must compare against the
// transformed target filenames. Regression: the diff path ignored output_ext
// and looked for the source .md name, reporting false drift.
func TestHandleExtrasDiff_TransformOutputExt(t *testing.T) {
	targetDir := t.TempDir()
	extras := []config.ExtraConfig{{
		Name:    "agents",
		Targets: []config.ExtraTargetConfig{{Path: targetDir, Mode: "copy", Extension: "to-toml"}},
	}}
	s, sourceDir := newTestServerWithExtras(t, extras, "")

	extRoot := filepath.Join(filepath.Dir(config.ConfigPath()), "extensions")
	writeExtensionWithDescription(t, extRoot, "to-toml", "identity to toml")

	srcDir := config.ResolveExtrasSourceDir(extras[0], "", sourceDir)
	if err := os.WriteFile(filepath.Join(srcDir, "api-architect.md"), []byte("# Agent\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Sync first so the transformed api-architect.toml lands in the target.
	syncReq := httptest.NewRequest(http.MethodPost, "/api/extras/sync",
		strings.NewReader(`{"name":"agents","force":true}`))
	s.handler.ServeHTTP(httptest.NewRecorder(), syncReq)

	// Diff must now report synced — not false drift against a missing .md target.
	req := httptest.NewRequest(http.MethodGet, "/api/extras/diff?name=agents", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Extras []extrasDiffEntry `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Extras) != 1 {
		t.Fatalf("expected 1 diff entry, got %d: %+v", len(resp.Extras), resp.Extras)
	}
	if !resp.Extras[0].Synced {
		t.Errorf("expected synced=true after .md→.toml sync, got drift: %+v", resp.Extras[0].Items)
	}
}

func TestBuildExtrasDiffItems_CopyContentDrift(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "rule.md"), []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "rule.md"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}

	items := buildExtrasDiffItems([]string{"rule.md"}, sourceDir, targetDir, "copy", false, "")
	if len(items) != 1 {
		t.Fatalf("expected one diff item, got %+v", items)
	}
	if items[0].Action != "update" || items[0].Reason != "content differs" {
		t.Fatalf("expected content update item, got %+v", items[0])
	}
}

// TestHandleExtrasAddTarget adds a target to an existing extra via POST.
func TestHandleExtrasAddTarget(t *testing.T) {
	tgt1 := t.TempDir()
	extras := []config.ExtraConfig{{
		Name:    "rules",
		Targets: []config.ExtraTargetConfig{{Path: tgt1, Mode: "merge"}},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	tgt2 := t.TempDir()
	body := `{"path":"` + tgt2 + `","mode":"copy"}`
	req := httptest.NewRequest(http.MethodPost, "/api/extras/rules/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	s.mu.RLock()
	got := s.cfg.Extras
	s.mu.RUnlock()
	if len(got) != 1 || len(got[0].Targets) != 2 {
		t.Fatalf("expected 2 targets, got %+v", got)
	}
	if got[0].Targets[1].Path != tgt2 || got[0].Targets[1].Mode != "copy" {
		t.Errorf("new target wrong: %+v", got[0].Targets[1])
	}
}

// TestHandleExtrasAddTarget_Duplicate rejects an existing target path.
func TestHandleExtrasAddTarget_Duplicate(t *testing.T) {
	tgt1 := t.TempDir()
	extras := []config.ExtraConfig{{
		Name:    "rules",
		Targets: []config.ExtraTargetConfig{{Path: tgt1, Mode: "merge"}},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	body := `{"path":"` + tgt1 + `","mode":"merge"}`
	req := httptest.NewRequest(http.MethodPost, "/api/extras/rules/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleExtrasAddTarget_TildeDuplicate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tgt1 := filepath.Join(home, ".codex", "agents")
	extras := []config.ExtraConfig{{
		Name:    "agents",
		Targets: []config.ExtraTargetConfig{{Path: tgt1, Mode: "merge"}},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	body := `{"path":"~/.codex/agents","mode":"merge"}`
	req := httptest.NewRequest(http.MethodPost, "/api/extras/agents/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 for equivalent tilde path, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleExtrasRemoveTarget removes one of two targets via DELETE.
func TestHandleExtrasRemoveTarget(t *testing.T) {
	tgt1, tgt2 := t.TempDir(), t.TempDir()
	extras := []config.ExtraConfig{{
		Name: "rules",
		Targets: []config.ExtraTargetConfig{
			{Path: tgt1, Mode: "merge"},
			{Path: tgt2, Mode: "merge"},
		},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	body := `{"path":"` + tgt2 + `"}`
	req := httptest.NewRequest(http.MethodDelete, "/api/extras/rules/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	s.mu.RLock()
	got := s.cfg.Extras
	s.mu.RUnlock()
	if len(got[0].Targets) != 1 || got[0].Targets[0].Path != tgt1 {
		t.Errorf("expected only tgt1 to remain, got %+v", got[0].Targets)
	}
}

// TestHandleExtrasRemoveTarget_Last rejects removing the only target.
func TestHandleExtrasRemoveTarget_Last(t *testing.T) {
	tgt1 := t.TempDir()
	extras := []config.ExtraConfig{{
		Name:    "rules",
		Targets: []config.ExtraTargetConfig{{Path: tgt1, Mode: "merge"}},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	body := `{"path":"` + tgt1 + `"}`
	req := httptest.NewRequest(http.MethodDelete, "/api/extras/rules/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleExtrasCreate_SingleFile(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	targetDir := t.TempDir()
	body := `{"name":"pi-prompt","file":"system.md","targets":[{"path":"` + targetDir + `","as":"APPEND_SYSTEM.md","mode":"copy"}]}`
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/extras", nil))
	var resp struct {
		Extras []extrasListEntry `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Extras) != 1 || resp.Extras[0].File != "system.md" || resp.Extras[0].Targets[0].As != "APPEND_SYSTEM.md" {
		t.Errorf("extras = %+v, want file system.md with as APPEND_SYSTEM.md", resp.Extras)
	}
}

// Two project single-file extras can share one folder: the folder is stored
// as a source relative to the project root and listed resolved.
func TestHandleExtrasCreate_ProjectFolderSharedBySingleFiles(t *testing.T) {
	s, projectRoot := newTestProjectServerWithExtras(t, nil)
	for _, name := range []string{"a", "b"} {
		body := `{"name":"` + name + `","folder":"prompts","file":"` + name + `.md","targets":[{"path":".claude/commands"}]}`
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body)))
		if rr.Code != http.StatusOK {
			t.Fatalf("create %s: expected 200, got %d: %s", name, rr.Code, rr.Body.String())
		}
	}

	s.mu.RLock()
	extras := s.projectCfg.Extras
	s.mu.RUnlock()
	for _, e := range extras {
		if e.Source != ".skillshare/extras/prompts" {
			t.Errorf("extra %s source = %q, want .skillshare/extras/prompts", e.Name, e.Source)
		}
	}

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/extras", nil))
	var resp struct {
		Extras []extrasListEntry `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(projectRoot, ".skillshare", "extras", "prompts")
	if len(resp.Extras) != 2 || resp.Extras[0].SourceDir != want || resp.Extras[1].SourceDir != want {
		t.Errorf("extras = %+v, want both with source_dir %s", resp.Extras, want)
	}
}

func TestHandleExtrasCreate_ProjectRejectsAbsoluteSource(t *testing.T) {
	s, _ := newTestProjectServerWithExtras(t, nil)
	body := `{"name":"a","source":"` + filepath.ToSlash(t.TempDir()) + `","file":"a.md","targets":[{"path":".claude/commands"}]}`
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body)))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleExtrasCreate_SingleFileRejectsInvalidSettings(t *testing.T) {
	targetDir := t.TempDir()
	cases := map[string]string{
		"flatten":      `{"name":"x","file":"a.md","targets":[{"path":"` + targetDir + `","flatten":true}]}`,
		"as_no_file":   `{"name":"x","targets":[{"path":"` + targetDir + `","as":"b.md"}]}`,
		"file_is_path": `{"name":"x","file":"sub/a.md","targets":[{"path":"` + targetDir + `"}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			s, _ := newTestServerWithExtras(t, nil, "")
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body)))
			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestHandleExtrasCreate_SingleFileTargetHeld(t *testing.T) {
	targetDir := t.TempDir()
	s, _ := newTestServerWithExtras(t, []config.ExtraConfig{{Name: "a", File: "a.md", Targets: []config.ExtraTargetConfig{{Path: targetDir, As: "OUT.md"}}}}, "")
	body := `{"name":"b","file":"b.md","targets":[{"path":"` + targetDir + `","as":"OUT.md"}]}`
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body)))
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(s.cfg.Extras) != 1 {
		t.Errorf("extras after rejected create = %+v, want unchanged", s.cfg.Extras)
	}
}
