package sync

import (
	"os"
	"path/filepath"
	"testing"
)

// The preview must match what RestoreExtraTarget then leaves behind.
func TestPreviewRestoreExtraTarget_MatchesRestore(t *testing.T) {
	for _, mode := range []string{"import", "symlink", "copy"} {
		t.Run(mode, func(t *testing.T) {
			src, tgt := setupExtraFileTest(t, "# agents\n")
			target := filepath.Join(tgt, "CLAUDE.md")
			os.WriteFile(target, []byte("mine\n"), 0644)
			f := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", mode)
			if _, err := SyncExtraFile(f, false, ""); err != nil {
				t.Fatal(err)
			}

			p := PreviewRestoreExtraTarget(f)
			if _, err := RestoreExtraTarget(f); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, target); p.Kind != RestoreKindContent || p.Content != got || p.Drift {
				t.Errorf("preview = %+v, restored %q", p, got)
			}
		})
	}
}

func TestPreviewRestoreExtraTarget_NoFileBefore(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents\n")
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "symlink")
	if _, err := SyncExtraFile(f, false, ""); err != nil {
		t.Fatal(err)
	}

	if p := PreviewRestoreExtraTarget(f); p.Kind != RestoreKindDelete || p.RecordedAt.IsZero() {
		t.Errorf("preview = %+v, want delete with the attach time", p)
	}
}

func TestPreviewRestoreExtraTarget_PreservesUnmanagedFile(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "shared")
	f := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "copy")
	os.WriteFile(f.Target, []byte("mine"), 0644)
	p := PreviewRestoreExtraTarget(f)
	changed, err := RestoreExtraTarget(f)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != RestoreKindContent || p.Content != "mine" || p.Drift || changed {
		t.Fatalf("preview=%+v, restore changed=%v, actual=%q", p, changed, readFile(t, f.Target))
	}
}
