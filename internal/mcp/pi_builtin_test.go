package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiBuiltinRenderAndValidation(t *testing.T) {
	s := Server{URL: "https://example.com/mcp", BearerToken: &Value{FromEnv: "TOKEN"}, PiOptions: map[string]any{"exposure": "deferred", "timeout": 120, "custom": map[string]any{"flag": true}}}
	out, err := Render("pi", s)
	if err != nil || out["transport"] != nil || out["exposure"] != "deferred" || out["headers"].(map[string]string)["Authorization"] != "Bearer ${TOKEN}" {
		t.Fatalf("%+v %v", out, err)
	}
	for _, options := range []map[string]any{{"exposure": "search"}, {"timeout": 0}, {"toolExposure": map[string]any{"get_*": "search"}}, {"cwd": true}, {"settings": map[string]any{}}, {"oauth": map[string]any{"clientName": 1}}, {"auth": map[string]any{}}, {"auth": "github"}} {
		s.PiOptions = options
		if err := s.Validate("docs"); err == nil {
			t.Fatalf("invalid options accepted: %+v", options)
		}
	}
	s.PiOptions = nil
	s.Targets = []string{"pi"}
	service := testService(t)
	if got := service.RenderNative("docs.with.dot", s); got[0].Error == "" {
		t.Fatal("Pi built-in rejects dots in names")
	}
	if _, err := Render("pi", Server{Command: "echo", Env: map[string]Value{"MODE": {Literal: "!date"}}}); err == nil {
		t.Fatal("a portable literal must not execute as a Pi secret command")
	}
}

// Pi 0.99.2 sends a provider's login token to the server, reads auth only from its global
// file, and reads names that differ only in - and _ as one server.
func TestPiFollowsPi0992Rules(t *testing.T) {
	auth := map[string]any{"auth": map[string]any{"provider": "github"}}
	for url, ok := range map[string]bool{"https://example.com/mcp": true, "http://localhost:8080/mcp": true, "http://example.com/mcp": false} {
		if err := (Server{URL: url, PiOptions: auth}).Validate("docs"); (err == nil) != ok {
			t.Fatalf("%s: %v", url, err)
		}
	}
	if err := (Server{Command: "docs", PiOptions: auth}).Validate("docs"); err == nil {
		t.Fatal("auth on a stdio server accepted")
	}
	// The adapter's auth was a string; loading and import still drop that one.
	for value, kept := range map[string]bool{`{"provider":"github"}`: true, `"oauth"`: false} {
		c, err := Import("pi", []byte(`{"mcpServers":{"docs":{"url":"https://example.com/mcp","auth":`+value+`}}}`), "")
		if err != nil || (c[0].Server.PiOptions["auth"] != nil) != kept {
			t.Fatalf("import auth %s: %+v %v", value, c, err)
		}
	}

	s := testService(t)
	write := func(servers string) {
		t.Helper()
		if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [pi]\n  servers:\n"+servers), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("    my-docs:\n      command: docs\n    my_docs:\n      command: docs\n")
	if _, err := s.Preview(); err == nil || !strings.Contains(err.Error(), "as one server") {
		t.Fatalf("want a name collision error, got %v", err)
	}
	write("    docs:\n      url: https://example.com/mcp\n      piOptions: {auth: {provider: github}}\n")
	if _, err := s.Preview(); err != nil {
		t.Fatalf("global auth: %v", err)
	}
	if source, err := LoadSource(s.ConfigPath); err != nil || source.Servers["docs"].PiOptions["auth"] == nil || len(source.Notices) > 0 {
		t.Fatalf("loading dropped Pi's auth: %+v %v", source, err)
	}
	s.ProjectRoot = filepath.Join(s.Home, "project")
	if _, err := s.Preview(); err == nil || !strings.Contains(err.Error(), "global mode") {
		t.Fatalf("want project auth refused, got %v", err)
	}
}

func TestPiBuiltinSyncImportAndScopes(t *testing.T) {
	for _, project := range []bool{false, true} {
		s := testService(t)
		if project {
			s.ProjectRoot = filepath.Join(s.Home, "project")
		}
		if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [pi]\n  servers:\n    docs:\n      command: docs\n      piOptions:\n        exposure: deferred\n        timeout: 120\n        custom: {flag: true}\n"), 0600); err != nil {
			t.Fatal(err)
		}
		p, err := s.Preview()
		if err != nil || p.Blocked {
			t.Fatalf("%+v %v", p, err)
		}
		if !strings.HasSuffix(p.Changes[0].Path, string(filepath.Separator)+"mcp.json") {
			t.Fatal(p.Changes)
		}
		if _, err := s.Apply(p.Revision); err != nil {
			t.Fatal(err)
		}
		c, err := s.ImportClient("pi")
		// exposure stays in piOptions.
		if err != nil || len(c) != 1 || !c[0].Server.Tools.IsZero() || c[0].Server.PiOptions["exposure"] != "deferred" || c[0].Server.PiOptions["custom"] == nil {
			t.Fatalf("%+v %v", c, err)
		}
		p, err = s.Preview()
		if err != nil || p.Changes[0].Action != "unchanged" {
			t.Fatalf("%+v %v", p, err)
		}
	}
}

// Sync always prunes; a config that still opts out with piOptionsPrune: false loads and
// prunes all the same.
func TestPiOptionsPruneOnlyOwnedUnchangedFields(t *testing.T) {
	s := testService(t)
	write := func(options string) {
		t.Helper()
		if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [pi]\n  servers:\n    docs:\n      command: docs\n      piOptionsPrune: false\n"+options), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("      piOptions: {exposure: deferred, timeout: 120}\n")
	p, err := s.Preview()
	if err != nil || len(p.Notices) != 1 || !strings.Contains(p.Notices[0], "piOptionsPrune") {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := s.Apply(p.Revision); err != nil {
		t.Fatal(err)
	}
	path := p.Changes[0].Path
	data, _ := os.ReadFile(path)
	var native map[string]any
	if err := json.Unmarshal(data, &native); err != nil {
		t.Fatal(err)
	}
	entry := native["mcpServers"].(map[string]any)["docs"].(map[string]any)
	entry["custom"] = "manual"
	if err := writeJSONFile(path, native); err != nil {
		t.Fatal(err)
	}
	write("      piOptions: {timeout: 120}\n")
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	server := source.Servers["docs"]
	server.Targets = []string{"pi"}
	view := s.RenderNative("docs", server)
	if len(view) != 1 || view[0].Error != "" || strings.Contains(view[0].Content, "exposure") || !strings.Contains(view[0].Content, "manual") {
		t.Fatalf("prune preview: %+v", view)
	}
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "exposure") {
		t.Fatal("preview modified native file")
	}
	p, err = s.Preview()
	if err != nil || p.Blocked || p.Changes[0].Action != "update" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := s.Apply(p.Revision); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "exposure") || !strings.Contains(string(data), "manual") {
		t.Fatal(string(data))
	}
	// A field edited directly in Pi must conflict before pruning it.
	if err := json.Unmarshal(data, &native); err != nil {
		t.Fatal(err)
	}
	native["mcpServers"].(map[string]any)["docs"].(map[string]any)["timeout"] = 99
	if err := writeJSONFile(path, native); err != nil {
		t.Fatal(err)
	}
	write("")
	server.PiOptions = nil
	view = s.RenderNative("docs", server)
	if len(view) != 1 || !strings.Contains(view[0].Error, "changed since sync: timeout") || view[0].Content != "" {
		t.Fatalf("conflict preview: %+v", view)
	}
	p, err = s.Preview()
	if err != nil || !p.Blocked {
		t.Fatalf("manual change was silently removed: %+v %v", p, err)
	}
}

func TestPiToolExposureOrderAndCredentialImport(t *testing.T) {
	input := []byte(`{"mcpServers":{"docs":{"command":"docs","exposure":"deferred","toolExposure":{"z*":"hidden","*":"direct"},"oauth":{"clientSecret":"!printf example-secret"},"custom":{"token":"example-token"}}}}`)
	candidates, err := Import("pi", input, "")
	if err != nil || len(candidates) != 1 || len(candidates[0].Problems) > 0 {
		t.Fatalf("%+v %v", candidates, err)
	}
	data, _ := json.Marshal(candidates[0].Server)
	if strings.Contains(string(data), "example-secret") || strings.Contains(string(data), "example-token") {
		t.Fatal("import retained literal credentials")
	}
	if strings.Index(string(data), `"z*"`) > strings.Index(string(data), `"*"`) {
		t.Fatal("import reordered patterns")
	}
	s := testService(t)
	server := candidates[0].Server
	server.Targets = []string{"pi"}
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, "", false); err != nil {
		t.Fatal(err)
	}
	p, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(p.Revision); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(p.Changes[0].Path)
	if strings.Index(string(data), `"z*"`) > strings.Index(string(data), `"*"`) {
		t.Fatal("source round trip reordered patterns")
	}
	// An unrelated native rewrite must keep the wildcard order too.
	server.URL, server.Command = "https://example.com/mcp", ""
	p, err = s.PreviewMutations([]Mutation{{Name: "docs", Server: &server, Replace: true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, p.Revision, true); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(p.Changes[0].Path)
	if strings.Index(string(data), `"z*"`) > strings.Index(string(data), `"*"`) {
		t.Fatal("rewrite reordered patterns")
	}
}

func TestPiBuiltinEnabledAndNativePreview(t *testing.T) {
	s := testService(t)
	server := Server{Command: "docs", Targets: []string{"pi"}, PiOptions: PiOptions{"enabled": false, "exposure": "direct"}}
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, "", true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Home, ".pi", "agent", "mcp.json")
	native, _ := os.ReadFile(path)
	var doc map[string]any
	_ = json.Unmarshal(native, &doc)
	entry := doc["mcpServers"].(map[string]any)["docs"].(map[string]any)
	entry["type"] = "stdio"
	entry["oauth"] = map[string]any{"clientSecret": "native-secret"}
	entry["toolExposure"] = map[string]any{"get_*": "direct"}
	if err := writeJSONFile(path, doc); err != nil {
		t.Fatal(err)
	}
	server.PiOptions = nil
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, "", true); err != nil {
		t.Fatal(err)
	}
	p, err := s.Preview()
	if err != nil || p.Blocked || p.Changes[0].Action != "unchanged" {
		t.Fatalf("cleared enabled drifted: %+v %v", p, err)
	}
	server.Command, server.URL = "", "https://example.com/mcp"
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, "", true); err != nil {
		t.Fatal(err)
	}
	native, _ = os.ReadFile(path)
	if strings.Contains(string(native), `"type"`) {
		t.Fatal("stdio type remained on an HTTP server")
	}
	view := s.RenderNative("docs", server)
	if len(view) != 1 || view[0].Error != "" || strings.Contains(view[0].Content, "native-secret") || !strings.Contains(view[0].Content, "<kept in Pi>") || !strings.Contains(view[0].Content, "toolExposure") {
		t.Fatalf("unsafe/incomplete preview: %+v", view)
	}
}

func TestPiBuiltinNativeDisableDoesNotConflict(t *testing.T) {
	s := testService(t)
	server := Server{Command: "docs", Targets: []string{"pi"}}
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, "", true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Home, ".pi", "agent", "mcp.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]any
	if err := json.Unmarshal(data, &native); err != nil {
		t.Fatal(err)
	}
	native["mcpServers"].(map[string]any)["docs"].(map[string]any)["enabled"] = false
	if err := writeJSONFile(path, native); err != nil {
		t.Fatal(err)
	}
	p, err := s.Preview()
	if err != nil || p.Blocked || p.Changes[0].Action != "unchanged" {
		t.Fatalf("%+v %v", p, err)
	}
	server.PiOptions = PiOptions{"enabled": true}
	p, err = s.PreviewMutation(Mutation{Name: "docs", Server: &server, Replace: true})
	if err != nil || p.Blocked || p.Changes[0].Action != "update" {
		t.Fatalf("%+v %v", p, err)
	}
}
