package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadManifest_NotExist(t *testing.T) {
	m, err := ReadManifest("/nonexistent/path")
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if len(m.Managed) != 0 {
		t.Errorf("expected empty Managed map, got %d entries", len(m.Managed))
	}
}

func TestReadManifest_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest for corrupt JSON")
	}
	if len(m.Managed) != 0 {
		t.Errorf("expected empty Managed map for corrupt JSON, got %d entries", len(m.Managed))
	}
}

func TestWriteReadManifest_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	original := &Manifest{
		Managed: map[string]string{
			"skill-a": "abc123",
			"skill-b": "def456",
		},
	}

	if err := WriteManifest(dir, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Managed) != 2 {
		t.Fatalf("expected 2 managed entries, got %d", len(loaded.Managed))
	}
	if loaded.Managed["skill-a"] != "abc123" {
		t.Errorf("expected checksum 'abc123', got %q", loaded.Managed["skill-a"])
	}
	if loaded.Managed["skill-b"] != "def456" {
		t.Errorf("expected checksum 'def456', got %q", loaded.Managed["skill-b"])
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}

	// Verify file is valid JSON
	data, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var check map[string]any
	if err := json.Unmarshal(data, &check); err != nil {
		t.Errorf("manifest file is not valid JSON: %v", err)
	}
}

func TestWriteReadManifest_MtimesRoundtrip(t *testing.T) {
	dir := t.TempDir()
	original := &Manifest{
		Managed: map[string]string{
			"skill-a": "abc123",
		},
		Mtimes: map[string]int64{
			"skill-a": 1718449200000000000, // some UnixNano
		},
	}

	if err := WriteManifest(dir, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Mtimes["skill-a"] != 1718449200000000000 {
		t.Errorf("expected mtime 1718449200000000000, got %d", loaded.Mtimes["skill-a"])
	}
}

func TestReadManifest_OldFormat_NoMtimes(t *testing.T) {
	dir := t.TempDir()
	// Simulate old manifest without mtimes field
	data := []byte(`{"managed":{"skill-a":"abc123"},"updated_at":"2025-01-01T00:00:00Z"}`)
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Mtimes == nil {
		t.Fatal("Mtimes should be initialized even for old format")
	}
	if len(loaded.Mtimes) != 0 {
		t.Errorf("expected empty Mtimes for old format, got %d entries", len(loaded.Mtimes))
	}
	if loaded.Managed["skill-a"] != "abc123" {
		t.Error("Managed should still work with old format")
	}
}

func TestRemoveManifest(t *testing.T) {
	dir := t.TempDir()

	// Write then remove
	original := &Manifest{Managed: map[string]string{"x": "y"}}
	if err := WriteManifest(dir, original); err != nil {
		t.Fatal(err)
	}
	if err := RemoveManifest(dir); err != nil {
		t.Fatal(err)
	}

	// Verify file is gone
	if _, err := os.Stat(filepath.Join(dir, ManifestFile)); !os.IsNotExist(err) {
		t.Error("expected manifest file to be removed")
	}

	// Removing again should be a no-op
	if err := RemoveManifest(dir); err != nil {
		t.Errorf("removing non-existent manifest should not error: %v", err)
	}
}

func TestReadManifestStrictRetainsOrdinaryRecoveryPolicy(t *testing.T) {
	for _, unreadable := range []bool{false, true} {
		t.Run(map[bool]string{false: "malformed", true: "unreadable"}[unreadable], func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ManifestFile)
			if unreadable {
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("{malformed"), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := readManifest(dir, true); err == nil {
				t.Fatal("strict observation accepted ambiguous metadata")
			}
			ordinary, err := ReadManifest(dir)
			if unreadable && err == nil || !unreadable && (err != nil || len(ordinary.Managed) != 0) {
				t.Fatalf("public policy changed: %+v %v", ordinary, err)
			}
			if tracker := loadCopyTracker(dir); tracker == nil || len(tracker.m.Managed) != 0 {
				t.Fatal("ordinary tracker no longer suppresses errors")
			}
			if _, err := loadSharedCopyTracker(dir); err == nil {
				t.Fatal("shared tracker suppressed metadata error")
			}
		})
	}
	strict, err := readManifest(t.TempDir(), true)
	if err != nil || strict.Managed == nil || strict.Mtimes == nil || strict.Naming == nil {
		t.Fatalf("missing manifest: %+v %v", strict, err)
	}
}
