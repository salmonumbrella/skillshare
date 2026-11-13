package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExtraFileImportExternalLinkRestore(t *testing.T) {
	for _, via := range []string{"", "symlink", "copy"} {
		t.Run(via, func(t *testing.T) {
			src, tgt := setupExtraFileTest(t, "shared\n")
			dot := filepath.Join(t.TempDir(), "dotfile")
			original := "my rules\r\n"
			os.WriteFile(dot, []byte(original), 0600)
			f := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "import")
			os.Symlink(dot, f.Target)
			if _, err := SyncExtraFile(f, false, ""); err != nil {
				t.Fatal(err)
			}
			if via != "" {
				other := f
				other.Mode = via
				if _, err := SyncExtraFile(other, false, ""); err != nil {
					t.Fatal(err)
				}
				if _, err := SyncExtraFile(f, false, ""); err != nil {
					t.Fatal(err)
				}
			}
			if !strings.Contains(readFile(t, f.Target), original) {
				t.Fatal("lost user lines")
			}
			if _, err := RestoreExtraTarget(f); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, dot); got != original {
				t.Fatalf("dotfile = %q", got)
			}
			if dest, err := os.Readlink(f.Target); err != nil || dest != dot {
				t.Fatalf("restore link = %q, %v", dest, err)
			}
		})
	}
}

func TestExtraFileDamagedImportBlock(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "shared")
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "import")
	damaged := importBlockBegin + "\n" + f.ImportLine() + "\nmy rules\n"
	os.WriteFile(f.Target, []byte(damaged), 0644)
	if _, err := SyncExtraFile(f, false, ""); err == nil || !strings.Contains(err.Error(), f.Target) {
		t.Fatalf("expected named error, got %v", err)
	}
	if got := readFile(t, f.Target); got != damaged {
		t.Fatalf("damaged block rewritten: %q", got)
	}
	// Include a complete duplicate from an older version as well.
	os.WriteFile(f.Target, []byte(importBlockBegin+"\n"+f.ImportLine()+"\n"+importBlockEnd+"\n\n"+damaged), 0644)
	if _, err := RestoreExtraTarget(f); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, f.Target); strings.Contains(got, f.ImportLine()) || !strings.Contains(got, "my rules") {
		t.Fatalf("restore = %q", got)
	}
}

func TestExtraFileRepointedLinkDrift(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "shared")
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "symlink")
	if _, err := SyncExtraFile(f, false, ""); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "missing-dotfile")
	os.Remove(f.Target)
	os.Symlink(dest, f.Target)
	for _, dry := range []bool{true, false} {
		res, err := SyncExtraFile(f, dry, "")
		if err != nil || len(res.Warnings) == 0 {
			t.Fatalf("dry=%v result=%+v err=%v", dry, res, err)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(extraBackupDir(f.Target), "drift"))
	found := false
	for _, e := range entries {
		if strings.Contains(readFile(t, filepath.Join(extraBackupDir(f.Target), "drift", e.Name())), dest) {
			found = true
		}
	}
	if !found {
		t.Fatal("old link destination not backed up")
	}
}

func TestExtraFileDryRunWarnsReplacement(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "shared")
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "copy")
	SyncExtraFile(f, false, "")
	os.WriteFile(f.Target, []byte("edited"), 0644)
	res, err := SyncExtraFile(f, true, "")
	if err != nil || !strings.Contains(strings.Join(res.Warnings, " "), "would back up") {
		t.Fatalf("result=%+v err=%v", res, err)
	}
	if readFile(t, f.Target) != "edited" {
		t.Fatal("dry run changed target")
	}
}

func TestExtraFileInvalidModeStatus(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "shared")
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "symlink")
	SyncExtraFile(f, false, "")
	f.Mode = "hardlink"
	if got := ExtraFileStatus(f); got != "invalid mode" {
		t.Fatalf("status=%q", got)
	}
}

func TestExtraJunctionRestoreRecord(t *testing.T) {
	_, tgt := setupExtraFileTest(t, "shared")
	path := filepath.Join(tgt, "AGENTS.md")
	if err := recordExtraAttach(path, "restore-junction", []byte("original destination"), 0644); err != nil {
		t.Fatal(err)
	}
	if !extraAttached(path) {
		t.Fatal("junction restore record must count as attached")
	}
	kind, _ := extraAttachRecord(path)
	if kind != "restore-junction" {
		t.Fatalf("kind=%q", kind)
	}
	preview := PreviewRestoreExtraTarget(ExtraFile{Target: path, Mode: "copy"})
	if preview.Kind != RestoreKindLink || preview.LinkTo != "original destination" {
		t.Fatalf("preview=%+v", preview)
	}
	clearExtraAttach(path)
	if _, err := os.Stat(filepath.Join(extraBackupDir(path), "restore-junction")); !os.IsNotExist(err) {
		t.Fatal("junction record not cleared")
	}
}

func TestExtraFileCopyEditIsModified(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "shared")
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "copy")
	SyncExtraFile(f, false, "")
	os.WriteFile(f.Target, []byte("edited"), 0644)
	if got := ExtraFileStatus(f); got != "modified" {
		t.Fatalf("status=%q", got)
	}
}

func TestExtraFileImportCRLFAndCleanup(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "shared")
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "import")
	original := "mine\r\nsecond\r\n"
	os.WriteFile(f.Target, []byte(original), 0644)
	SyncExtraFile(f, false, "")
	got := readFile(t, f.Target)
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Errorf("mixed line endings: %q", got)
	}
	other := f
	other.Mode = "copy"
	SyncExtraFile(other, false, "")
	SyncExtraFile(f, false, "")
	backupExtraDrift(f.Target, "")
	if _, err := RestoreExtraTarget(f); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, f.Target); got != original {
		t.Fatalf("restore=%q", got)
	}
	if extraAttached(f.Target) || hasExtraWritten(f.Target) || fileExists(filepath.Join(extraBackupDir(f.Target), extraImportBase)) {
		t.Fatal("stale attach record")
	}
	if len(driftBackups(t, f.Target)) == 0 {
		t.Fatal("drift backup deleted")
	}
}

func TestExtraFileRestoreKeepsRecordForRemainingImport(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "shared")
	a := NewExtraFile(src, "AGENTS.md", tgt, "", "import")
	b := NewExtraFile(src, "other.md", tgt, "AGENTS.md", "import")
	SyncExtraFile(a, false, "")
	SyncExtraFile(b, false, "")
	if _, err := RestoreExtraTarget(a); err != nil {
		t.Fatal(err)
	}
	if !extraAttached(a.Target) {
		t.Fatal("removed attach record while another shared import remains")
	}
	if _, err := RestoreExtraTarget(b); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.Target); !os.IsNotExist(err) {
		t.Fatal("last restore did not remove created target")
	}
}

func TestExtraFileRestorePreservesUnmanagedImportLine(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "shared")
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "import")
	original := f.ImportLine() + "\nmy rules\n"
	os.WriteFile(f.Target, []byte(original), 0644)
	SyncExtraFile(f, false, "")
	if _, err := RestoreExtraTarget(f); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, f.Target); got != original {
		t.Fatalf("unmanaged import changed: %q", got)
	}
}

func TestSingleFileJunctionWarning(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows directory junction")
	}
	src, tgt := setupExtraFileTest(t, "shared")
	source, target, destination := filepath.Join(src, "AGENTS.md"), filepath.Join(tgt, "AGENTS.md"), t.TempDir()
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", target, destination).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v: %s", err, out)
	}
	for _, dryRun := range []bool{true, false} {
		res, err := SyncExtraFile(ExtraFile{Source: source, Target: target, Mode: "copy"}, dryRun, "")
		if err != nil {
			t.Fatal(err)
		}
		code := "junction_replaced"
		if dryRun {
			code = "would_replace_junction"
		}
		if len(res.FileWarnings) != 1 || res.FileWarnings[0].Code != code || res.FileWarnings[0].Params["destination"] != destination || !strings.Contains(res.Warnings[0], "junction "+target+" pointing to "+destination) {
			t.Fatalf("%+v", res)
		}
	}
}

func TestSingleFileCodedWarnings(t *testing.T) {
	for _, code := range []string{"backed_up", "would_back_up", "target_directory", "file_link_fallback"} {
		t.Run(code, func(t *testing.T) {
			src, tgt := setupExtraFileTest(t, "shared")
			mode := "copy"
			if code == "file_link_fallback" {
				withoutFileLinks(t)
				mode = "merge"
			}
			f := NewExtraFile(src, "AGENTS.md", tgt, "AGENTS.md", mode)
			if code == "target_directory" {
				if err := os.Mkdir(f.Target, 0755); err != nil {
					t.Fatal(err)
				}
			} else if code != "file_link_fallback" {
				if err := os.WriteFile(f.Target, []byte("local"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			res, err := SyncExtraFile(f, code == "would_back_up", "")
			if err != nil {
				t.Fatal(err)
			}
			if len(res.FileWarnings) != 1 || res.FileWarnings[0].Code != code || res.FileWarnings[0].Message != res.Warnings[0] {
				t.Fatalf("%+v", res)
			}
			if code != "file_link_fallback" && res.FileWarnings[0].Params["path"] != f.Target {
				t.Fatalf("%+v", res.FileWarnings[0])
			}
		})
	}
}
