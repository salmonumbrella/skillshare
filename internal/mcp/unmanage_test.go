package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func changeFor(p *Plan, path, name string) *Change {
	for i, c := range p.Changes {
		if c.Path == path && c.Name == name {
			return &p.Changes[i]
		}
	}
	return nil
}

func TestUnmanageKeepsAgentFilesAndSyncLeavesThemAlone(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	applyProjects(t, s)
	cursor := filepath.Join(tmp, ".cursor", "mcp.json")
	before, _ := os.ReadFile(cursor)
	if _, err := s.Mutate(Mutation{Name: "shared", Remove: true, Unmanage: true}, "", false); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(cursor)
	if !bytes.Equal(before, after) {
		t.Fatalf("Agent file changed:\n%s", after)
	}
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if c := changeFor(plan, cursor, "shared"); c != nil {
		t.Fatalf("sync still plans %s for the unmanaged entry", c.Action)
	}
	// projB's own switch of the same name is still managed.
	if c := changeFor(plan, filepath.Join(tmp, "projB", "opencode.json"), "shared"); c == nil || c.Action != "unchanged" {
		t.Fatalf("projB switch: %+v", c)
	}
}

func TestUnmanageInAProjectLeavesTheGlobalServerManaged(t *testing.T) {
	s, tmp := projectsService(t, `mcp:
  servers:
    docs:
      command: docs-server
      targets: [cursor]
  projects:
    $TMP/projA:
      servers:
        docs:
          command: docs-server
          targets: [cursor]
`)
	applyProjects(t, s)
	root := filepath.Join(tmp, "projA")
	if _, err := s.Mutate(Mutation{Project: root, Name: "docs", Remove: true, Unmanage: true}, "", false); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if c := changeFor(plan, filepath.Join(root, ".cursor", "mcp.json"), "docs"); c != nil {
		t.Fatalf("project entry still planned: %s", c.Action)
	}
	if c := changeFor(plan, filepath.Join(tmp, ".cursor", "mcp.json"), "docs"); c == nil || c.Action != "unchanged" {
		t.Fatalf("global entry: %+v", c)
	}
}

func TestUnmanageRejected(t *testing.T) {
	s, _ := projectsService(t, projectsConfig)
	if _, err := s.Mutate(Mutation{Name: "shared", Remove: true, Unmanage: true}, "", true); err == nil || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("with sync: %v", err)
	}
	if _, err := s.Mutate(Mutation{Name: "shared", Unmanage: true}, "", false); err == nil {
		t.Fatal("unmanage without remove was saved")
	}
}

func TestUnmanagedEntryConflictsWhenTheServerIsAddedBack(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	applyProjects(t, s)
	if _, err := s.Mutate(Mutation{Name: "shared", Remove: true, Unmanage: true}, "", false); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PreviewMutation(Mutation{Name: "shared", Server: &Server{Command: "other", Targets: []string{"cursor"}}})
	if err != nil {
		t.Fatal(err)
	}
	if c := changeFor(plan, filepath.Join(tmp, ".cursor", "mcp.json"), "shared"); c == nil || !strings.HasPrefix(c.Message, "existing entry is not managed") {
		t.Fatalf("want the not managed conflict, got %+v", c)
	}
}

func addNativeEntry(t *testing.T, path, name string) {
	t.Helper()
	var document map[string]map[string]any
	data, _ := os.ReadFile(path)
	if json.Unmarshal(data, &document) != nil {
		document = map[string]map[string]any{"mcpServers": {}}
	}
	document["mcpServers"][name] = map[string]any{"command": "mine"}
	data, _ = json.Marshal(document)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestFindUnmanagedListsOnlyEntriesNoConfigManages(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	applyProjects(t, s)
	global := filepath.Join(tmp, ".cursor", "mcp.json")
	project := filepath.Join(tmp, "projA", ".cursor", "mcp.json")
	addNativeEntry(t, global, "mine")
	addNativeEntry(t, project, "local")
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []Unmanaged{
		{Target: "cursor", Path: global, Names: []string{"mine"}},
		{Target: "cursor", Project: filepath.Join(tmp, "projA"), Path: project, Names: []string{"local"}},
	}
	if got := s.FindUnmanaged(source); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestImportProjectClientReadsThatRootsFile(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	root := filepath.Join(tmp, "projA")
	addNativeEntry(t, filepath.Join(root, ".cursor", "mcp.json"), "local")
	candidates, err := s.ImportProjectClient(root, "cursor")
	if err != nil || len(candidates) != 1 || candidates[0].Name != "local" {
		t.Fatalf("candidates %+v, err %v", candidates, err)
	}
	if _, err := s.ImportProjectClient(filepath.Join(tmp, "nope"), "cursor"); !errors.Is(err, ErrUnknownProject) {
		t.Fatalf("unknown root: %v", err)
	}
}
