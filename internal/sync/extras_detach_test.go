package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

func TestSharedOutputSettingsChangeDetachesPriorOutputs(t *testing.T) {
	for _, change := range []string{"extension", "relative-extension", "flatten", "include", "exclude", "local-include"} {
		for _, edited := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/edited=%v", change, edited), func(t *testing.T) {
				extensionChange := strings.Contains(change, "extension")
				if extensionChange && runtime.GOOS == "windows" {
					t.Skip("cat transform fixture requires Unix")
				}
				sb, _, source, target := sharedRunFixture(t, "copy", false)
				sb.WriteFile(filepath.Join(source, "nested", "guide.md"), "guide\n")
				shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
				settings := ""
				if extensionChange {
					settings = ", extension: old-format"
					if change == "relative-extension" {
						settings = ", extension: ./old-format"
					}
				}
				writeDeclaration := func(settings string) *config.Config {
					sb.WriteFile(shared, fmt.Sprintf("version: 1\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: copy%s}]}]\n", filepath.ToSlash(source), filepath.ToSlash(target), settings))
					cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
					if err != nil {
						t.Fatal(err)
					}
					return cfg
				}
				cfg := writeDeclaration(settings)
				opts := ExtraRunOptions{ResolvePath: config.ExpandPath, ResolveExtension: func(name string) (*ExtensionSpec, error) {
					ext := "txt"
					if filepath.Base(name) == "new-format" {
						ext = "toml"
					}
					return &ExtensionSpec{Name: name, Run: []string{"cat"}, Dir: source, OutputExt: ext}, nil
				}}
				session, err := BeginSharedExtras(cfg, false)
				if err != nil {
					t.Fatal(err)
				}
				run := RunExtraTargets(cfg.Extras[0], source, opts)
				if len(run.Targets) != 1 || run.Targets[0].Err != nil || run.Targets[0].Result == nil || len(run.Targets[0].Result.Errors) > 0 {
					t.Fatalf("initial apply=%+v", run)
				}
				session.Record(cfg.Extras[0], source, run, nil)
				if err := session.Save(); err != nil {
					t.Fatal(err)
				}
				session, err = BeginSharedExtras(cfg, false)
				if err != nil {
					t.Fatal(err)
				}
				if detached := session.Detach(config.ExpandPath); len(detached) != 0 {
					t.Fatalf("unchanged output settings churned: %+v", detached)
				}
				oldOutput := filepath.Join(target, "nested", "guide.md")
				switch change {
				case "extension", "relative-extension":
					oldOutput = filepath.Join(target, "nested", "guide.txt")
					settings = ", extension: new-format"
					if change == "relative-extension" {
						settings = ", extension: ./new-format"
					}
				case "flatten":
					settings = ", flatten: true"
				case "include", "local-include":
					settings = ", include: [fixture.md]"
				case "exclude":
					settings = ", exclude: [nested/]"
				}
				if edited {
					sb.WriteFile(oldOutput, "edited by user\n")
				}
				if change == "local-include" {
					sb.WriteConfig(fmt.Sprintf("source: %q\nshared: {path: ./team.yaml}\ntargets: {}\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: copy%s}]}]\n", filepath.ToSlash(sb.SourcePath), filepath.ToSlash(source), filepath.ToSlash(target), settings))
					settings = "" // The local override changes output selection.
				}
				cfg = writeDeclaration(settings)
				session, err = BeginSharedExtras(cfg, false)
				if err != nil {
					t.Fatal(err)
				}
				detached := session.Detach(config.ExpandPath)
				if len(detached) != 1 || len(detached[0].Errors) != 0 {
					t.Fatalf("settings change did not detach: %+v", detached)
				}
				if edited {
					if len(detached[0].Kept) != 1 || detached[0].Kept[0] != oldOutput || !session.Protected()[oldOutput] {
						t.Fatalf("edited prior output not reported/protected: %+v", detached)
					}
				} else if _, err := os.Lstat(oldOutput); !os.IsNotExist(err) {
					t.Fatalf("stale output remains: %s: %v", oldOutput, err)
				}
				opts.Force, opts.Protected = true, session.Protected()
				run = RunExtraTargets(cfg.Extras[0], source, opts)
				if run.Targets[0].Err != nil || run.Targets[0].Result == nil || len(run.Targets[0].Result.Errors) != 0 {
					t.Fatalf("replacement apply=%+v", run)
				}
				session.Record(cfg.Extras[0], source, run, nil)
				if err := session.Save(); err != nil {
					t.Fatal(err)
				}
				if edited && sb.ReadFile(oldOutput) != "edited by user\n" {
					t.Fatal("replacement overwrote edited prior output")
				}
				if extensionChange && sb.ReadFile(filepath.Join(target, "nested", "guide.toml")) != "guide\n" {
					t.Fatal("extension filename was not replaced")
				}
				if change == "flatten" && sb.ReadFile(filepath.Join(target, "guide.md")) != "guide\n" {
					t.Fatal("flattened output missing")
				}
				selected := "fixture.md"
				if extensionChange {
					selected = "fixture.toml"
				}
				if sb.ReadFile(filepath.Join(target, selected)) != "managed content\n" {
					t.Fatal("selected output missing")
				}
			})
		}
	}
}

func TestSharedOutputSettingsSelectedTargetKeepsOtherDeclaration(t *testing.T) {
	sb, _, source, target := sharedRunFixture(t, "copy", false)
	other := filepath.Join(sb.Root, "other")
	sb.WriteFile(filepath.Join(source, "nested", "guide.md"), "guide\n")
	shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
	writeDeclaration := func(flatten bool) *config.Config {
		sb.WriteFile(shared, fmt.Sprintf("version: 1\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: copy, flatten: %v},{path: %q, mode: copy, flatten: %v}]}]\n", filepath.ToSlash(source), filepath.ToSlash(target), flatten, filepath.ToSlash(other), flatten))
		cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	cfg := writeDeclaration(false)
	applySharedFixture(t, cfg, source)
	cfg = writeDeclaration(true)
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if detached := session.DetachTarget(config.ExpandPath, "rules", target); len(detached) != 1 {
		t.Fatalf("selected detach=%+v", detached)
	}
	extra := cfg.Extras[0]
	extra.Targets = extra.Targets[:1]
	run := RunExtraTargets(extra, source, ExtraRunOptions{ResolvePath: config.ExpandPath, Protected: session.Protected()})
	session.Record(extra, source, run, nil)
	if err := session.Save(); err != nil {
		t.Fatal(err)
	}
	if sb.ReadFile(filepath.Join(other, "nested", "guide.md")) != "guide\n" {
		t.Fatal("selected sync touched unselected target")
	}
	session, err = BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if detached := session.DetachTarget(config.ExpandPath, "rules", other); len(detached) != 1 || detached[0].Removed != 2 {
		t.Fatalf("unselected target lost its prior declaration: %+v", detached)
	}
}

func TestSharedSameOutputSettingsLocalHandoffAndNoPointer(t *testing.T) {
	sb, _, source, target := sharedRunFixture(t, "copy", false)
	shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
	sb.WriteFile(shared, fmt.Sprintf("version: 1\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: copy, flatten: true, include: ['*.md'], exclude: [draft.md]}]}]\n", filepath.ToSlash(source), filepath.ToSlash(target)))
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	applySharedFixture(t, cfg, source)
	sb.WriteConfig(fmt.Sprintf("source: %q\nshared: {path: ./team.yaml}\ntargets: {}\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: copy, flatten: true, include: ['*.md'], exclude: [draft.md]}]}]\n", filepath.ToSlash(sb.SourcePath), filepath.ToSlash(source), filepath.ToSlash(target)))
	cfg, err = config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if detached := session.Detach(config.ExpandPath); len(detached) != 0 || len(session.state.State.Extras) != 0 {
		t.Fatalf("same settings should hand off without cleanup: %+v", detached)
	}
	if sb.ReadFile(filepath.Join(target, "fixture.md")) != "managed content\n" {
		t.Fatal("same-identity handoff removed local output")
	}
	if err := session.Save(); err != nil {
		t.Fatal(err)
	}
	statePath, err := config.SharedStatePath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	before := sb.ReadFile(statePath)
	// A directory in place of the pointer would fail if no-pointer consulted it.
	if err := os.Remove(shared); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(shared, 0755); err != nil {
		t.Fatal(err)
	}
	cfg.Shared = nil
	session, err = BeginSharedExtras(cfg, false)
	if session != nil || err != nil || len(session.Detach(config.ExpandPath)) != 0 || session.Save() != nil || before != sb.ReadFile(statePath) {
		t.Fatal("no-pointer touched shared state")
	}
}

func sharedRunFixture(t *testing.T, mode string, single bool) (*testutil.Sandbox, *config.Config, string, string) {
	t.Helper()
	sb := testutil.NewSandbox(t)
	t.Cleanup(sb.Cleanup)
	source, target := filepath.Join(sb.Root, "source-extra"), filepath.Join(sb.Root, "target-extra")
	sb.WriteFile(filepath.Join(source, "fixture.md"), "managed content\n")
	file := ""
	if single {
		file = "file: fixture.md,"
	}
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), fmt.Sprintf("version: 1\nextras: [{name: rules, source: %q, %s targets: [{path: %q, mode: %s}]}]\n", filepath.ToSlash(source), file, filepath.ToSlash(target), mode))
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {}\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	return sb, cfg, source, target
}

func applySharedFixture(t *testing.T, cfg *config.Config, source string) *SharedExtrasRun {
	t.Helper()
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	session.Detach(config.ExpandPath)
	for _, extra := range cfg.Extras {
		run := RunExtraTargets(extra, source, ExtraRunOptions{ResolvePath: config.ExpandPath, Protected: session.Protected()})
		for _, target := range run.Targets {
			if target.Err != nil || target.Result == nil || len(target.Result.Errors) > 0 {
				t.Fatalf("apply=%+v", target)
			}
		}
		session.Record(extra, source, run, nil)
	}
	if err := session.Save(); err != nil {
		t.Fatal(err)
	}
	return session
}

func TestSharedExtrasDetachMatrix(t *testing.T) {
	for _, single := range []bool{false, true} {
		for _, mode := range []string{"merge", "copy", "symlink", "import", "prepend", "append"} {
			if !single && (mode == "import" || mode == "prepend" || mode == "append") {
				continue
			}
			t.Run(fmt.Sprintf("single=%v/%s", single, mode), func(t *testing.T) {
				sb, cfg, source, target := sharedRunFixture(t, mode, single)
				if (mode == "merge" || mode == "symlink") && !CanCreateFileLink() {
					t.Skip("file link runtime requires Windows token coverage")
				}
				applySharedFixture(t, cfg, source)
				// Receipts, not current discovery, own removal after the source disappears.
				if err := os.RemoveAll(source); err != nil {
					t.Fatal(err)
				}
				sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\nextras: []\n")
				cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				session, err := BeginSharedExtras(cfg, false)
				if err != nil {
					t.Fatal(err)
				}
				detached := session.Detach(config.ExpandPath)
				if len(detached) != 1 || len(detached[0].Errors) > 0 || len(detached[0].Kept) > 0 {
					t.Fatalf("detach=%+v", detached)
				}
				output := filepath.Join(target, "fixture.md")
				if !single && mode == "symlink" {
					output = target
				}
				if _, err := os.Lstat(output); !os.IsNotExist(err) {
					t.Fatalf("output remains: %v", err)
				}
				if err := session.Save(); err != nil {
					t.Fatal(err)
				}
				state, err := config.LoadSharedState(sb.ConfigPath, cfg.EffectiveGitRoot(), filepath.Dir(cfg.SharedStatus().Path))
				if err != nil {
					t.Fatal(err)
				}
				if len(state.State.Extras) != 0 {
					t.Fatal("detached receipt remains")
				}
			})
		}
	}
}

func TestSharedCopyReceiptsExcludeIdenticalPreexisting(t *testing.T) {
	for _, preexisting := range []bool{false, true} {
		t.Run(fmt.Sprint(preexisting), func(t *testing.T) {
			sb, cfg, source, target := sharedRunFixture(t, "copy", false)
			if preexisting {
				sb.WriteFile(filepath.Join(target, "fixture.md"), "managed content\n")
			}
			applySharedFixture(t, cfg, source)
			applySharedFixture(t, cfg, source) // idempotent apply keeps previous ownership only
			sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\n")
			cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			session, err := BeginSharedExtras(cfg, false)
			if err != nil {
				t.Fatal(err)
			}
			session.Detach(config.ExpandPath)
			_, err = os.Stat(filepath.Join(target, "fixture.md"))
			if preexisting && err != nil {
				t.Fatal("user's identical file was detached")
			}
			if !preexisting && !os.IsNotExist(err) {
				t.Fatal("owned unchanged copy was forgotten")
			}
		})
	}
}

func TestSharedDetachProtectsEditedReplacementAndLocalOverride(t *testing.T) {
	for _, single := range []bool{false, true} {
		for _, local := range []bool{false, true} {
			t.Run(fmt.Sprintf("single=%v/local=%v", single, local), func(t *testing.T) {
				sb, cfg, source, target := sharedRunFixture(t, "copy", single)
				applySharedFixture(t, cfg, source)
				output := filepath.Join(target, "fixture.md")
				sb.WriteFile(output, "edited by user\n")
				replacement := filepath.Join(sb.Root, "replacement")
				sb.WriteFile(filepath.Join(replacement, "fixture.md"), "replacement\n")
				if local {
					sb.WriteConfig(fmt.Sprintf("source: %s\nshared: {path: ./team.yaml}\ntargets: {}\nextras: [{name: rules, source: %q, targets: [{path: %q,mode: copy}]}]\n", filepath.ToSlash(sb.SourcePath), filepath.ToSlash(replacement), filepath.ToSlash(target)))
				} else {
					raw := sb.ReadFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"))
					sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), strings.ReplaceAll(raw, filepath.ToSlash(source), filepath.ToSlash(replacement)))
				}
				cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				session, err := BeginSharedExtras(cfg, false)
				if err != nil {
					t.Fatal(err)
				}
				results := session.Detach(config.ExpandPath)
				if len(results) != 1 || len(results[0].Kept) != 1 {
					t.Fatalf("detach=%+v", results)
				}
				run := RunExtraTargets(cfg.Extras[0], replacement, ExtraRunOptions{ResolvePath: config.ExpandPath, Force: true, Protected: session.Protected()})
				session.Record(cfg.Extras[0], replacement, run, nil)
				if sb.ReadFile(output) != "edited by user\n" {
					t.Fatal("force overwrote protected output")
				}
				if err := session.Save(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestSharedDetachDryRunAndNoPointer(t *testing.T) {
	sb, cfg, source, target := sharedRunFixture(t, "copy", false)
	applySharedFixture(t, cfg, source)
	statePath, err := config.SharedStatePath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	before := sb.ReadFile(statePath)
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\n")
	cfg, err = config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	session, err := BeginSharedExtras(cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	session.Detach(config.ExpandPath)
	if err := session.Save(); err != nil {
		t.Fatal(err)
	}
	if sb.ReadFile(statePath) != before || sb.ReadFile(filepath.Join(target, "fixture.md")) != "managed content\n" {
		t.Fatal("dry-run changed files")
	}
	cfg.Shared = nil
	if session, err := BeginSharedExtras(cfg, false); session != nil || err != nil {
		t.Fatalf("no-pointer=%v %v", session, err)
	}
}

func TestDetachExtraFileKeepsEditedBlocks(t *testing.T) {
	for _, mode := range []string{"prepend", "append"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
			if err := os.MkdirAll(source, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "fixture.md"), []byte("managed\n"), 0644); err != nil {
				t.Fatal(err)
			}
			f := NewExtraFile(source, "fixture.md", target, "", mode)
			if _, err := SyncExtraFile(f, false, ""); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(f.Target)
			if err != nil {
				t.Fatal(err)
			}
			edited := strings.Replace(string(raw), "managed\n", "user edit\n", 1)
			if err := os.WriteFile(f.Target, []byte(edited), 0644); err != nil {
				t.Fatal(err)
			}
			changed, kept, err := DetachExtraFileTarget(f)
			if err != nil || changed || !kept {
				t.Fatalf("detach=%v %v %v", changed, kept, err)
			}
			raw, err = os.ReadFile(f.Target)
			if err != nil || string(raw) != edited {
				t.Fatal("edited block removed")
			}
		})
	}
}

func TestSharedMergeReplacedLinkVanishedSourceProtectsForce(t *testing.T) {
	if !CanCreateFileLink() {
		t.Skip("requires file links")
	}
	sb, cfg, source, target := sharedRunFixture(t, "merge", false)
	applySharedFixture(t, cfg, source)
	output := filepath.Join(target, "fixture.md")
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(output, "local replacement\n")
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(sb.Root, "replacement")
	sb.WriteFile(filepath.Join(replacement, "fixture.md"), "new source\n")
	path := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
	sb.WriteFile(path, strings.ReplaceAll(sb.ReadFile(path), filepath.ToSlash(source), filepath.ToSlash(replacement)))
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	detached := session.Detach(config.ExpandPath)
	if len(detached) != 1 || len(detached[0].Kept) != 1 {
		t.Fatalf("detach=%+v", detached)
	}
	RunExtraTargets(cfg.Extras[0], replacement, ExtraRunOptions{ResolvePath: config.ExpandPath, Force: true, Protected: session.Protected()})
	if sb.ReadFile(output) != "local replacement\n" {
		t.Fatal("force erased replaced link")
	}
}

func TestSharedFailedDetachRetainedAndRetried(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission regression; Windows token verification pending")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses fixture permissions")
	}
	sb, cfg, source, target := sharedRunFixture(t, "copy", false)
	applySharedFixture(t, cfg, source)
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(target, 0755) })
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	results := session.Detach(config.ExpandPath)
	if len(results) != 1 || len(results[0].Errors) == 0 || len(session.state.State.Extras) != 1 || !protectedOutput(filepath.Join(target, "fixture.md"), session.Protected()) {
		t.Fatalf("failed cleanup=%+v", results)
	}
	if err := session.Save(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0755); err != nil {
		t.Fatal(err)
	}
	retry, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	results = retry.Detach(config.ExpandPath)
	if len(results) != 1 || len(results[0].Errors) > 0 || len(retry.state.State.Extras) != 0 {
		t.Fatalf("retry=%+v", results)
	}
}

func TestSharedSamePathModeAndFileIdentityDetach(t *testing.T) {
	for _, change := range []string{"mode", "file", "skip", "local-same"} {
		t.Run(change, func(t *testing.T) {
			sb, cfg, source, target := sharedRunFixture(t, "copy", true)
			applySharedFixture(t, cfg, source)
			shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
			raw := sb.ReadFile(shared)
			switch change {
			case "mode":
				sb.WriteFile(shared, strings.ReplaceAll(raw, "mode: copy", "mode: import"))
			case "file":
				sb.WriteFile(filepath.Join(source, "other.md"), "other\n")
				sb.WriteFile(shared, strings.ReplaceAll(raw, "file: fixture.md", "file: other.md"))
			case "skip":
				sb.WriteConfig(strings.ReplaceAll(sb.ReadFile(sb.ConfigPath), "{path: ./team.yaml}", "{path: ./team.yaml, skip: {extras: ['*']}}"))
			case "local-same":
				sb.WriteConfig(sb.ReadFile(sb.ConfigPath) + fmt.Sprintf("extras: [{name: rules,source: %q,file: fixture.md,targets: [{path: %q,mode: copy}]}]\n", filepath.ToSlash(source), filepath.ToSlash(target)))
			}
			cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			session, err := BeginSharedExtras(cfg, false)
			if err != nil {
				t.Fatal(err)
			}
			detached := session.Detach(config.ExpandPath)
			if change == "local-same" {
				if len(detached) != 0 || len(session.state.State.Extras) != 0 {
					t.Fatalf("handoff=%+v", detached)
				}
				return
			}
			if len(detached) != 1 || len(detached[0].Errors) > 0 {
				t.Fatalf("detach=%+v", detached)
			}
			if _, err := os.Lstat(filepath.Join(target, "fixture.md")); !os.IsNotExist(err) {
				t.Fatal("old identity left behind")
			}
		})
	}
}

func TestProtectedExtensionOutputSkipsBeforeExecution(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "fixture.md"), []byte("input"), 0644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(target, "fixture.txt")
	if err := os.WriteFile(output, []byte("user edit"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := SyncExtraTarget(config.ExtraConfig{}, config.ExtraTargetConfig{}, source, target, "copy", &ExtensionSpec{OutputExt: "txt"}, ExtraSyncOptions{Force: true, Protected: map[string]bool{output: true}})
	if err != nil || result.Skipped != 1 || len(result.Errors) != 0 {
		t.Fatalf("protected transform=%+v %v", result, err)
	}
	raw, err := os.ReadFile(output)
	if err != nil || string(raw) != "user edit" {
		t.Fatal("protected output changed")
	}
}

func TestSharedMergeReplacedForeignLinkIsProtected(t *testing.T) {
	if !CanCreateFileLink() {
		t.Skip("requires file links")
	}
	sb, cfg, source, target := sharedRunFixture(t, "merge", false)
	applySharedFixture(t, cfg, source)
	output := filepath.Join(target, "fixture.md")
	foreign := filepath.Join(sb.Root, "native.md")
	sb.WriteFile(foreign, "native content\n")
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, output); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(sb.Root, "replacement")
	sb.WriteFile(filepath.Join(replacement, "fixture.md"), "replacement\n")
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), fmt.Sprintf("version: 1\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: merge}]}]\n", filepath.ToSlash(replacement), filepath.ToSlash(target)))
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	results := session.Detach(config.ExpandPath)
	if len(results) != 1 || len(results[0].Kept) != 1 {
		t.Fatalf("detach=%+v", results)
	}
	RunExtraTargets(cfg.Extras[0], replacement, ExtraRunOptions{ResolvePath: config.ExpandPath, Force: true, Protected: session.Protected()})
	dest, err := os.Readlink(output)
	if err != nil || dest != foreign {
		t.Fatalf("link replaced: %q %v", dest, err)
	}
}

func TestSharedMergeDoesNotAcquirePreexistingLinks(t *testing.T) {
	if !CanCreateFileLink() {
		t.Skip("requires file links")
	}
	sb, cfg, source, target := sharedRunFixture(t, "merge", false)
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(target, "fixture.md")
	if err := os.Symlink(filepath.Join(source, "fixture.md"), output); err != nil {
		t.Fatal(err)
	}
	applySharedFixture(t, cfg, source)
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	session.Detach(config.ExpandPath)
	if _, err := os.Readlink(output); err != nil {
		t.Fatal("preexisting link detached", err)
	}
}

func TestProtectedAncestorPreventsMergeOrphanPruning(t *testing.T) {
	if !CanCreateFileLink() {
		t.Skip("requires file links")
	}
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target", "nested")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(target, "orphan.md")
	if err := os.Symlink(filepath.Join(source, "gone.md"), output); err != nil {
		t.Fatal(err)
	}
	_, err := SyncExtraTarget(config.ExtraConfig{}, config.ExtraTargetConfig{}, source, target, "merge", nil, ExtraSyncOptions{Force: true, Protected: map[string]bool{filepath.Dir(target): true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Readlink(output); err != nil {
		t.Fatal("protected ancestor pruned", err)
	}
}

func TestSharedDetachDoesNotFollowReplacedParent(t *testing.T) {
	if !CanCreateFileLink() {
		t.Skip("requires directory links")
	}
	sb, cfg, source, target := sharedRunFixture(t, "copy", false)
	sb.WriteFile(filepath.Join(source, "nested", "own.md"), "same content\n")
	applySharedFixture(t, cfg, source)
	nested := filepath.Join(target, "nested")
	if err := os.RemoveAll(nested); err != nil {
		t.Fatal(err)
	}
	native := filepath.Join(sb.Root, "native")
	sb.WriteFile(filepath.Join(native, "own.md"), "same content\n")
	if err := os.Symlink(native, nested); err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	results := session.Detach(config.ExpandPath)
	if len(results) != 1 || len(results[0].Kept) == 0 || !session.Protected()[nested] {
		t.Fatalf("detach=%+v protected=%v", results, session.Protected())
	}
	if sb.ReadFile(filepath.Join(native, "own.md")) != "same content\n" {
		t.Fatal("native file removed")
	}
	RunExtraTargets(config.ExtraConfig{Targets: []config.ExtraTargetConfig{{Path: target, Mode: "copy"}}}, source, ExtraRunOptions{ResolvePath: config.ExpandPath, Force: true, Protected: session.Protected()})
	if sb.ReadFile(filepath.Join(native, "own.md")) != "same content\n" {
		t.Fatal("native file overwritten")
	}
}

func TestSharedDetachSelectionPreservesOtherReceipts(t *testing.T) {
	sb, cfg, source, target := sharedRunFixture(t, "copy", false)
	other := filepath.Join(sb.Root, "other")
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), fmt.Sprintf("version: 1\nextras: [{name: rules, source: %q, targets: [{path: %q,mode: copy},{path: %q,mode: copy}]}]\n", filepath.ToSlash(source), filepath.ToSlash(target), filepath.ToSlash(other)))
	cfg, loadErr := config.LoadReadOnlyPath(sb.ConfigPath)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	applySharedFixture(t, cfg, source)
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if results := session.Detach(config.ExpandPath, "unselected"); len(results) != 0 || len(session.state.State.Extras[0].Receipt.Targets) != 2 {
		t.Fatalf("name filter=%+v", results)
	}
	results := session.DetachTarget(config.ExpandPath, "rules", target)
	if len(results) != 1 || len(session.state.State.Extras[0].Receipt.Targets) != 1 || session.state.State.Extras[0].Receipt.Targets[0].Path != other {
		t.Fatalf("target filter=%+v state=%+v", results, session.state.State.Extras)
	}
	if _, err := os.Stat(filepath.Join(other, "fixture.md")); err != nil {
		t.Fatal("unselected target touched", err)
	}
}
