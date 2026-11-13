package sharedfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenBoundsAndReasons(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(filepath.Join(dir, "missing")); err == nil || err.Reason != Missing {
		t.Fatalf("missing: %v", err)
	}
	if _, err := Open(dir); err == nil || err.Reason != Unreadable {
		t.Fatalf("directory: %v", err)
	}
	path := filepath.Join(dir, "team.yaml")
	if err := os.WriteFile(path, make([]byte, (1<<20)+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil || err.Reason != Unreadable {
		t.Fatalf("oversize: %v", err)
	}
	raw := []byte("version: 1\nextras: []\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil || string(doc.Raw) != string(raw) || doc.Dir != dir {
		t.Fatalf("open: %v", err)
	}
}
