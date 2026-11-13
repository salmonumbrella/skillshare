package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupExtraFileTest writes AGENTS.md into a source dir and points the backup
// store at a temp state dir.
func setupExtraFileTest(t *testing.T, content string) (srcDir, tgtDir string) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	srcDir, tgtDir = setupExtrasTest(t, map[string]string{"AGENTS.md": content, "other.md": "not synced"})
	return srcDir, tgtDir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestDiscoverExtraSource_SingleFile(t *testing.T) {
	src, _ := setupExtraFileTest(t, "# agents")
	files, err := DiscoverExtraSource(src, "AGENTS.md")
	if err != nil || len(files) != 1 || files[0] != "AGENTS.md" {
		t.Fatalf("files = %v, err = %v", files, err)
	}
	if _, err := DiscoverExtraSource(src, "MISSING.md"); err == nil {
		t.Fatal("expected error for missing source file")
	}
}

func TestSyncExtraFile_SymlinkWithAs(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	f := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "symlink")

	res, err := SyncExtraFile(f, false, "")
	if err != nil || res.Synced != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	dest, err := os.Readlink(filepath.Join(tgt, "CLAUDE.md"))
	if err != nil || dest != filepath.Join(src, "AGENTS.md") {
		t.Fatalf("link = %q, err = %v", dest, err)
	}
	if _, err := os.Lstat(filepath.Join(tgt, "other.md")); !os.IsNotExist(err) {
		t.Error("only the extra's file should be synced")
	}
	if got := ExtraFileStatus(f); got != "synced" {
		t.Errorf("status = %q, want synced", got)
	}
}

func TestSyncExtraFile_Copy(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "copy")

	if _, err := SyncExtraFile(f, false, ""); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(tgt, "AGENTS.md")); got != "# agents" {
		t.Fatalf("copied content = %q", got)
	}
	if got := ExtraFileStatus(f); got != "synced" {
		t.Errorf("status = %q, want synced", got)
	}
}

func TestSyncExtraFile_DryRunChangesNothing(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "import")
	if _, err := SyncExtraFile(f, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(tgt, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("dry run must not create the target")
	}
}

func TestExtraFileStatus_NoSource(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "x")
	f := NewExtraFile(src, "MISSING.md", tgt, "", "merge")
	if got := ExtraFileStatus(f); got != "no source" {
		t.Errorf("status = %q, want no source", got)
	}
}

func TestSyncExtraFile_BacksUpThenReplaces(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	target := filepath.Join(tgt, "CLAUDE.md")
	os.WriteFile(target, []byte("my own rules"), 0644)
	f := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "merge")

	res, err := SyncExtraFile(f, false, "")
	if err != nil || res.Synced != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if info, _ := os.Lstat(target); info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("target should be replaced by a symlink without --force")
	}
	if len(res.Warnings) == 0 {
		t.Error("expected a warning that the file was backed up")
	}

	restored, err := RestoreExtraTarget(f)
	if err != nil || !restored {
		t.Fatalf("restored = %v, err = %v", restored, err)
	}
	info, _ := os.Lstat(target)
	if info.Mode()&os.ModeSymlink != 0 || readFile(t, target) != "my own rules" {
		t.Fatal("restore should bring back the backed-up file")
	}
}

func TestRestoreExtraTarget_NoBackupRemovesOnlyOurLink(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "merge")
	SyncExtraFile(f, false, "")

	if _, err := RestoreExtraTarget(f); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(tgt, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("our symlink should be removed")
	}
	if readFile(t, filepath.Join(src, "AGENTS.md")) != "# agents" {
		t.Fatal("source must be untouched")
	}
}

func TestRestoreExtraTarget_AfterResolvingEditRestoresPreAttachState(t *testing.T) {
	for _, resolve := range []func(ExtraFile, string) error{CollectBackExtraFile, ReapplyExtraFile} {
		src, tgt := setupExtraFileTest(t, "# agents")
		target := filepath.Join(tgt, "AGENTS.md")
		f := NewExtraFile(src, "AGENTS.md", tgt, "", "merge")
		SyncExtraFile(f, false, "")
		os.Remove(target)
		os.WriteFile(target, []byte("# agents\nlocal edit"), 0644)

		if err := resolve(f, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := RestoreExtraTarget(f); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			t.Fatal("the target had no file before it was attached, so restore must leave none")
		}
	}
}

func TestRestoreExtraTarget_KeepsUserFile(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	target := filepath.Join(tgt, "AGENTS.md")
	os.WriteFile(target, []byte("user edit"), 0644)
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "merge")

	restored, err := RestoreExtraTarget(f)
	if err != nil || restored {
		t.Fatalf("restored = %v, err = %v", restored, err)
	}
	if readFile(t, target) != "user edit" {
		t.Fatal("a file skillshare did not create must stay")
	}
}

const importBegin = "<!-- skillshare:instructions:begin -->"
const importEnd = "<!-- skillshare:instructions:end -->"

func TestSyncExtraFile_ImportIdempotent(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	target := filepath.Join(tgt, "CLAUDE.md")
	os.WriteFile(target, []byte("# Mine\nkeep me\n"), 0644)
	f := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "import")

	for i := 0; i < 2; i++ {
		if _, err := SyncExtraFile(f, false, ""); err != nil {
			t.Fatal(err)
		}
	}
	want := importBegin + "\n@" + filepath.Join(src, "AGENTS.md") + "\n" + importEnd + "\n\n# Mine\nkeep me\n"
	if got := readFile(t, target); got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
	if got := ExtraFileStatus(f); got != "synced" {
		t.Errorf("status = %q, want synced", got)
	}
}

func TestSyncExtraFile_ImportTwoExtrasThenRemoveOne(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	os.WriteFile(filepath.Join(src, "TEAM.md"), []byte("# team"), 0644)
	target := filepath.Join(tgt, "CLAUDE.md")
	os.WriteFile(target, []byte("outside\n"), 0644)
	a := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "import")
	b := NewExtraFile(src, "TEAM.md", tgt, "CLAUDE.md", "import")
	SyncExtraFile(a, false, "")
	SyncExtraFile(b, false, "")

	lineA := "@" + filepath.Join(src, "AGENTS.md")
	lineB := "@" + filepath.Join(src, "TEAM.md")
	want := importBegin + "\n" + lineA + "\n" + lineB + "\n" + importEnd + "\n\noutside\n"
	if got := readFile(t, target); got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}

	if _, err := RestoreExtraTarget(a); err != nil {
		t.Fatal(err)
	}
	want = importBegin + "\n" + lineB + "\n" + importEnd + "\n\noutside\n"
	if got := readFile(t, target); got != want {
		t.Fatalf("after removing a =\n%s\nwant\n%s", got, want)
	}

	RestoreExtraTarget(b)
	if got := readFile(t, target); got != "outside\n" {
		t.Fatalf("after removing both = %q, want only outside content", got)
	}
}

func TestSyncExtraFile_ImportCreatedFileRemovedWhenEmpty(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	f := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "import")
	SyncExtraFile(f, false, "")
	target := filepath.Join(tgt, "CLAUDE.md")
	if !strings.HasPrefix(readFile(t, target), importBegin) {
		t.Fatal("import should create the target file with the block")
	}

	if _, err := RestoreExtraTarget(f); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatal("a file skillshare created should be deleted once empty")
	}
}

func TestExtraFileStatus_ModifiedThenResolve(t *testing.T) {
	for _, resolution := range []string{"collect", "reapply"} {
		t.Run(resolution, func(t *testing.T) {
			src, tgt := setupExtraFileTest(t, "# agents")
			f := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "symlink")
			SyncExtraFile(f, false, "")
			target := filepath.Join(tgt, "CLAUDE.md")
			os.Remove(target)
			os.WriteFile(target, []byte("edited in tool"), 0644)

			if got := ExtraFileStatus(f); got != "modified" {
				t.Fatalf("status = %q, want modified", got)
			}

			var err error
			wantSource := "# agents"
			if resolution == "collect" {
				err = CollectBackExtraFile(f, "")
				wantSource = "edited in tool"
			} else {
				err = ReapplyExtraFile(f, "")
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := ExtraFileStatus(f); got != "synced" {
				t.Errorf("status after %s = %q, want synced", resolution, got)
			}
			if got := readFile(t, filepath.Join(src, "AGENTS.md")); got != wantSource {
				t.Errorf("source = %q, want %q", got, wantSource)
			}
			if resolution == "reapply" {
				drift, _ := os.ReadDir(filepath.Join(extraBackupDir(target), "drift"))
				if len(drift) != 1 {
					t.Error("reapply should keep the edited file as a drift backup")
				}
			}
		})
	}
}

// driftBackups returns the contents of the drift backups kept for target.
func driftBackups(t *testing.T, target string) []string {
	t.Helper()
	dir := filepath.Join(extraBackupDir(target), "drift")
	var out []string
	for _, name := range extraBackupNames(dir) {
		out = append(out, readFile(t, filepath.Join(dir, name)))
	}
	return out
}

func TestSyncExtraFile_ModifiedTargetKeptAsDriftNotRestorePoint(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	target := filepath.Join(tgt, "AGENTS.md")
	os.WriteFile(target, []byte("my own rules"), 0644)
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "merge")
	SyncExtraFile(f, false, "")
	os.Remove(target)
	os.WriteFile(target, []byte("edited in tool"), 0644)

	if _, err := SyncExtraFile(f, false, ""); err != nil {
		t.Fatal(err)
	}
	if got := driftBackups(t, target); len(got) != 1 || got[0] != "edited in tool" {
		t.Fatalf("drift backups = %q, want the edit", got)
	}
	if _, err := RestoreExtraTarget(f); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "my own rules" {
		t.Fatalf("restored %q, want the pre-attach file", got)
	}
}

func TestRestoreExtraTarget_ModifiedTargetKeepsEditAsDrift(t *testing.T) {
	for _, before := range []string{"", "my own rules"} {
		t.Run("before="+before, func(t *testing.T) {
			src, tgt := setupExtraFileTest(t, "# agents")
			target := filepath.Join(tgt, "AGENTS.md")
			if before != "" {
				os.WriteFile(target, []byte(before), 0644)
			}
			f := NewExtraFile(src, "AGENTS.md", tgt, "", "symlink")
			SyncExtraFile(f, false, "")
			os.Remove(target)
			os.WriteFile(target, []byte("edited in tool"), 0644)

			restored, err := RestoreExtraTarget(f)
			if err != nil || !restored {
				t.Fatalf("restored = %v, err = %v", restored, err)
			}
			if before == "" {
				if _, err := os.Lstat(target); !os.IsNotExist(err) {
					t.Fatal("no file existed before attach, so restore must remove it")
				}
			} else if got := readFile(t, target); got != before {
				t.Fatalf("restored %q, want %q", got, before)
			}
			if got := driftBackups(t, target); len(got) != 1 || got[0] != "edited in tool" {
				t.Fatalf("drift backups = %q, want the edit", got)
			}
		})
	}
}

func TestRestoreExtraTarget_NoFileAtAttachIgnoresOlderBackup(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	target := filepath.Join(tgt, "AGENTS.md")
	os.WriteFile(target, []byte("old content"), 0644)
	if err := BackupFile(target, BackupReasonEdit); err != nil {
		t.Fatal(err)
	}
	os.Remove(target)
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "merge")
	SyncExtraFile(f, false, "")

	if _, err := RestoreExtraTarget(f); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("restore put back %q; no file existed at attach time", readFile(t, target))
	}
}

func TestRestoreExtraTarget_PutsBackForeignSymlink(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	dotfile := filepath.Join(t.TempDir(), "dotfiles-AGENTS.md")
	os.WriteFile(dotfile, []byte("dotfiles rules"), 0644)
	target := filepath.Join(tgt, "AGENTS.md")
	os.Symlink(dotfile, target)
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "merge")
	SyncExtraFile(f, false, "")

	if _, err := RestoreExtraTarget(f); err != nil {
		t.Fatal(err)
	}
	if dest, err := os.Readlink(target); err != nil || dest != dotfile {
		t.Fatalf("link = %q, err = %v; want the user's symlink back", dest, err)
	}
}

func TestSyncExtraFile_ReattachAfterConfigOnlyDetachTakesFreshRestorePoint(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	target := filepath.Join(tgt, "AGENTS.md")
	os.WriteFile(target, []byte("v1"), 0644)
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "merge")
	SyncExtraFile(f, false, "")
	ForgetExtraTarget(f) // detached from config without restoring
	os.Remove(target)
	os.WriteFile(target, []byte("v2"), 0644)

	SyncExtraFile(f, false, "")
	if _, err := RestoreExtraTarget(f); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "v2" {
		t.Fatalf("restored %q, want the file present at the second attach", got)
	}
}

func TestRestoreExtraTarget_SymlinkToImportKeepsRestorePoint(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	target := filepath.Join(tgt, "CLAUDE.md")
	os.WriteFile(target, []byte("mine"), 0644)
	SyncExtraFile(NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "symlink"), false, "")
	f := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "import")
	if _, err := SyncExtraFile(f, false, ""); err != nil {
		t.Fatal(err)
	}

	if _, err := RestoreExtraTarget(f); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "mine" {
		t.Fatalf("restored %q, want the pre-attach file", got)
	}
}

func TestRestoreExtraTarget_ModeSwitchKeepsUserFile(t *testing.T) {
	for _, modes := range [][2]string{{"import", "symlink"}, {"copy", "symlink"}, {"symlink", "copy"}, {"copy", "import"}, {"import", "copy"}} {
		t.Run(modes[0]+"->"+modes[1], func(t *testing.T) {
			src, tgt := setupExtraFileTest(t, "# agents")
			target := filepath.Join(tgt, "CLAUDE.md")
			os.WriteFile(target, []byte("mine"), 0644)
			SyncExtraFile(NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", modes[0]), false, "")
			f := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", modes[1])
			if _, err := SyncExtraFile(f, false, ""); err != nil {
				t.Fatal(err)
			}

			if _, err := RestoreExtraTarget(f); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, target); got != "mine" {
				t.Fatalf("restored %q, want the pre-attach file", got)
			}
		})
	}
}

func TestSyncExtraFile_SwitchBackToImportRestoresUserLines(t *testing.T) {
	for _, via := range []string{"copy", "symlink"} {
		t.Run("import->"+via+"->import", func(t *testing.T) {
			src, tgt := setupExtraFileTest(t, "# agents")
			target := filepath.Join(tgt, "CLAUDE.md")
			os.WriteFile(target, []byte("mine\n"), 0644)
			imp := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "import")
			SyncExtraFile(imp, false, "")
			attached := readFile(t, target)
			SyncExtraFile(NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", via), false, "")

			if _, err := SyncExtraFile(imp, false, ""); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, target); got != attached {
				t.Fatalf("after switching back = %q, want %q", got, attached)
			}
			if _, err := RestoreExtraTarget(imp); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, target); got != "mine\n" {
				t.Errorf("restored %q, want the original file", got)
			}
		})
	}
}

func TestSyncExtraFile_SwitchBackToImportBacksUpEditedCopy(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	target := filepath.Join(tgt, "CLAUDE.md")
	os.WriteFile(target, []byte("mine\n"), 0644)
	imp := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "import")
	SyncExtraFile(imp, false, "")
	SyncExtraFile(NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "copy"), false, "")
	os.WriteFile(target, []byte("edited in copy mode"), 0644)

	if _, err := SyncExtraFile(imp, false, ""); err != nil {
		t.Fatal(err)
	}

	if got := driftBackups(t, target); len(got) != 1 || got[0] != "edited in copy mode" {
		t.Errorf("drift = %v, want the edit", got)
	}
	if got := readFile(t, target); !strings.HasSuffix(got, "\nmine\n") {
		t.Errorf("target = %q, want the user lines back", got)
	}
}

func TestSyncExtraFile_SwitchBackToImportKeepsOtherImports(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "# agents")
	os.WriteFile(filepath.Join(src, "TEAM.md"), []byte("# team"), 0644)
	target := filepath.Join(tgt, "CLAUDE.md")
	os.WriteFile(target, []byte("mine\n"), 0644)
	a := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "import")
	b := NewExtraFile(src, "TEAM.md", tgt, "CLAUDE.md", "import")
	SyncExtraFile(a, false, "")
	SyncExtraFile(b, false, "")
	SyncExtraFile(NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "copy"), false, "")
	SyncExtraFile(b, false, "")

	if _, err := SyncExtraFile(a, false, ""); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, target)
	if len(ManagedImportLines(got)) != 4 || !strings.Contains(got, a.ImportLine()) || !strings.Contains(got, b.ImportLine()) || !strings.HasSuffix(got, "\nmine\n") {
		t.Errorf("target = %q, want both imports in one block and the user lines", got)
	}
}

func TestSyncExtraFile_SwitchBackToImportKeepsEditsMadeInImportMode(t *testing.T) {
	for _, via := range []string{"copy", "symlink"} {
		t.Run("import->"+via+"->import", func(t *testing.T) {
			src, tgt := setupExtraFileTest(t, "# agents")
			target := filepath.Join(tgt, "CLAUDE.md")
			os.WriteFile(target, []byte("A\n"), 0644)
			imp := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "import")
			other := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", via)
			// Leave import once so the attach-time restore point holds only A.
			SyncExtraFile(imp, false, "")
			SyncExtraFile(other, false, "")
			SyncExtraFile(imp, false, "")
			os.WriteFile(target, []byte(readFile(t, target)+"B\n"), 0644)
			before := readFile(t, target)

			SyncExtraFile(other, false, "")
			if _, err := SyncExtraFile(imp, false, ""); err != nil {
				t.Fatal(err)
			}

			if got := readFile(t, target); got != before {
				t.Fatalf("after switching back = %q, want %q", got, before)
			}
			if _, err := RestoreExtraTarget(imp); err != nil {
				t.Fatal(err)
			}
			// Import-mode restore drops only the import line, as it would have
			// before the switch: A plus the edit made in import mode.
			if got := readFile(t, target); got != "A\nB\n" {
				t.Errorf("restored %q, want the file as left in import mode", got)
			}
		})
	}
}

func TestSyncExtraFile_ModeSwitchPreservesEmptyImportBase(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "shared")
	f := NewExtraFile(src, "AGENTS.md", tgt, "CLAUDE.md", "copy")
	os.WriteFile(f.Target, []byte("old local content"), 0644)
	run := func(mode string) {
		t.Helper()
		f.Mode = mode
		if _, err := SyncExtraFile(f, false, ""); err != nil {
			t.Fatal(err)
		}
	}
	run("copy")
	run("import")
	only, _ := addImportLine("", f.importLine())
	os.WriteFile(f.Target, []byte(only), 0644)
	run("copy")
	run("import")
	if got := readFile(t, f.Target); strings.Contains(got, "old local content") {
		t.Fatalf("deleted import content resurrected: %q", got)
	}
}
