package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

func TestSharedCopyReceiptSaveFailureRecoversBeforeDetach(t *testing.T) {
	sb, cfg, source, target := sharedRunFixture(t, "copy", false)
	sb.WriteFile(filepath.Join(source, "control.md"), "already here\n")
	sb.WriteFile(filepath.Join(target, "control.md"), "already here\n")
	session, err := BeginSharedExtras(cfg, false)
	t.Cleanup(session.Close)
	if err != nil {
		t.Fatal(err)
	}
	fault := errors.New("receipt replacement fault")
	session.state.SetWriteFaultForTest(func() error {
		if session.state.State.Pending == nil {
			return fault
		}
		return nil
	})
	run := RunExtraTargets(cfg.Extras[0], source, ExtraRunOptions{ResolvePath: config.ExpandPath, Shared: session})
	if len(run.Targets) != 1 || run.Targets[0].Result == nil || len(run.Targets[0].Result.Errors) == 0 {
		t.Fatalf("receipt fault was not surfaced: %+v", run)
	}
	if got := sb.ReadFile(filepath.Join(target, "fixture.md")); got != "managed content\n" {
		t.Fatalf("native copy did not complete: %q", got)
	}
	if !errors.Is(session.Save(), fault) {
		t.Fatal("failed run did not retain its receipt-save error")
	}
	session.Close()
	// A fresh run must recover ownership without treating the identical copy as
	// a pre-existing user file. The sibling present before the intent stays local.
	retry, err := BeginSharedExtras(cfg, false)
	t.Cleanup(retry.Close)
	if err != nil {
		t.Fatal(err)
	}
	RunExtraTargets(retry.Config().Extras[0], source, ExtraRunOptions{ResolvePath: config.ExpandPath, Shared: retry})
	if err := retry.Save(); err != nil {
		t.Fatal(err)
	}
	retry.Close()
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\nextras: []\n")
	cfg, err = config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	detach, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer detach.Close()
	results := detach.Detach(config.ExpandPath)
	if len(results) != 1 || len(results[0].Errors) != 0 {
		t.Fatalf("detach=%+v", results)
	}
	if _, err := os.Lstat(filepath.Join(target, "fixture.md")); !os.IsNotExist(err) {
		t.Fatalf("recovered copy remained after removal: %v", err)
	}
	if got := sb.ReadFile(filepath.Join(target, "control.md")); got != "already here\n" {
		t.Fatalf("identical pre-existing control changed: %q", got)
	}
}

func TestSharedRelinkStopsNativeCleanupOnReceiptFailure(t *testing.T) {
	for _, failure := range []string{"checkpoint", "native-proof"} {
		t.Run(failure, func(t *testing.T) {
			sb, cfg, source, target := sharedRunFixture(t, "copy", true)
			sb.WriteFile(filepath.Join(target, "fixture.md"), "original\n")
			applySharedFixture(t, cfg, source)
			old := cfg.Extras[0]
			native := filepath.Join(sb.Root, "native-only")
			sb.WriteFile(filepath.Join(native, "fixture.md"), "native original\n")
			f := NewExtraFile(source, old.File, native, "", "copy")
			if _, err := SyncExtraFile(f, false, ""); err != nil {
				t.Fatal(err)
			}
			old.Targets = append(old.Targets, config.ExtraTargetConfig{Path: native, Mode: "copy"})
			replacement := filepath.Join(sb.Root, "replacement")
			sb.WriteFile(filepath.Join(replacement, "fixture.md"), "replacement\n")
			sb.WriteConfig(fmt.Sprintf("source: %q\nshared: {path: ./team.yaml}\ntargets: {}\nextras: [{name: rules, file: fixture.md, source: %q, targets: [{path: %q, mode: copy}]}]\n", filepath.ToSlash(sb.SourcePath), filepath.ToSlash(replacement), filepath.ToSlash(target)))
			session, err := BeginSharedExtras(cfg, false)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			if failure == "checkpoint" {
				session.state.SetWriteFaultForTest(func() error { return errors.New("relink checkpoint fault") })
			} else {
				ForgetExtraTarget(NewExtraFile(source, old.File, target, "", "copy"))
			}
			results := session.DetachRelink(old, source, config.ExpandPath)
			found := false
			for _, result := range results {
				found = found || len(result.Errors) > 0
			}
			if !found || sb.ReadFile(f.Target) != "managed content\n" {
				t.Fatalf("cleanup advanced after receipt failure: %+v", results)
			}
			state, err := config.LoadSharedState(sb.ConfigPath, cfg.EffectiveGitRoot(), filepath.Dir(cfg.SharedStatus().Path))
			if err != nil || len(state.State.Extras) != 1 {
				t.Fatalf("failed cleanup lost retry receipt: %v %+v", err, state)
			}
		})
	}
}

func TestSharedRelinkProtectionSurvivesForceApply(t *testing.T) {
	sb, cfg, source, target := sharedRunFixture(t, "copy", true)
	applySharedFixture(t, cfg, source)
	sb.WriteFile(filepath.Join(target, "fixture.md"), "edited\n")
	old := cfg.Extras[0]
	replacement := filepath.Join(sb.Root, "replacement")
	sb.WriteFile(filepath.Join(replacement, "fixture.md"), "replacement\n")
	sb.WriteConfig(fmt.Sprintf("source: %q\nshared: {path: ./team.yaml}\ntargets: {}\nextras: [{name: rules, file: fixture.md, source: %q, targets: [{path: %q, mode: copy}]}]\n", filepath.ToSlash(sb.SourcePath), filepath.ToSlash(replacement), filepath.ToSlash(target)))
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	results := session.DetachRelink(old, source, config.ExpandPath)
	if len(results) != 1 || len(results[0].Kept) != 1 || len(results[0].Errors) > 0 {
		t.Fatalf("detach=%+v", results)
	}
	RunExtraTargets(session.Config().Extras[0], replacement, ExtraRunOptions{ResolvePath: config.ExpandPath, Protected: session.Protected(), Shared: session, Force: true})
	if sb.ReadFile(filepath.Join(target, "fixture.md")) != "edited\n" {
		t.Fatal("force apply overwrote kept receipt output")
	}
}

func TestSharedDirectoryRelinkCleansOnlyNativeUnreceiptedLinks(t *testing.T) {
	if !CanCreateFileLink() {
		t.Skip("requires file links")
	}
	sb, cfg, source, target := sharedRunFixture(t, "merge", false)
	applySharedFixture(t, cfg, source)
	old := cfg.Extras[0]
	native := filepath.Join(target, "native.md")
	sb.WriteFile(filepath.Join(source, "native.md"), "native\n")
	if err := os.Symlink(filepath.Join(source, "native.md"), native); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(target, "fixture.md")); err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(filepath.Join(target, "fixture.md"), "edited\n")
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\n")
	for _, dry := range []bool{true, false} {
		session, err := BeginSharedExtras(cfg, dry)
		if err != nil {
			t.Fatal(err)
		}
		// Dry-run keeps its supplied config snapshot, as the existing owner does.
		session.cfg.Extras = nil
		results := session.DetachRelink(old, source, config.ExpandPath)
		session.Close()
		for _, result := range results {
			if len(result.Errors) > 0 {
				t.Fatalf("cleanup=%+v", results)
			}
		}
		if _, err := os.Lstat(native); dry && err != nil || !dry && !os.IsNotExist(err) {
			t.Fatalf("native cleanup dry=%v: %v", dry, err)
		}
		if sb.ReadFile(filepath.Join(target, "fixture.md")) != "edited\n" {
			t.Fatal("native cleanup erased receipt-protected content")
		}
	}
}

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
				t.Cleanup(session.Close)
				if err != nil {
					t.Fatal(err)
				}
				opts.Shared = session
				run := RunExtraTargets(cfg.Extras[0], source, opts)
				if len(run.Targets) != 1 || run.Targets[0].Err != nil || run.Targets[0].Result == nil || len(run.Targets[0].Result.Errors) > 0 {
					t.Fatalf("initial apply=%+v", run)
				}

				if err := session.Save(); err != nil {
					t.Fatal(err)
				}
				session.Close()
				session, err = BeginSharedExtras(cfg, false)
				t.Cleanup(session.Close)
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
				session.Close()
				session, err = BeginSharedExtras(cfg, false)
				t.Cleanup(session.Close)
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
				opts.Force, opts.Protected, opts.Shared = true, session.Protected(), session
				run = RunExtraTargets(cfg.Extras[0], source, opts)
				if run.Targets[0].Err != nil || run.Targets[0].Result == nil || len(run.Targets[0].Result.Errors) != 0 {
					t.Fatalf("replacement apply=%+v", run)
				}

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
	t.Cleanup(session.Close)
	if err != nil {
		t.Fatal(err)
	}
	if detached := session.DetachTarget(config.ExpandPath, "rules", target); len(detached) != 1 {
		t.Fatalf("selected detach=%+v", detached)
	}
	extra := cfg.Extras[0]
	extra.Targets = extra.Targets[:1]
	RunExtraTargets(extra, source, ExtraRunOptions{ResolvePath: config.ExpandPath, Protected: session.Protected(), Shared: session})

	if err := session.Save(); err != nil {
		t.Fatal(err)
	}
	if sb.ReadFile(filepath.Join(other, "nested", "guide.md")) != "guide\n" {
		t.Fatal("selected sync touched unselected target")
	}
	session.Close()
	session, err = BeginSharedExtras(cfg, false)
	t.Cleanup(session.Close)
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
	t.Cleanup(session.Close)
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
	session.Close()
	session, err = BeginSharedExtras(cfg, false)
	t.Cleanup(session.Close)
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
	t.Cleanup(session.Close)
	if err != nil {
		t.Fatal(err)
	}
	session.Detach(config.ExpandPath)
	for _, extra := range cfg.Extras {
		run := RunExtraTargets(extra, source, ExtraRunOptions{ResolvePath: config.ExpandPath, Protected: session.Protected(), Shared: session})
		for _, target := range run.Targets {
			if target.Err != nil || target.Result == nil || len(target.Result.Errors) > 0 {
				t.Fatalf("apply=%+v", target)
			}
		}

	}
	if err := session.Save(); err != nil {
		t.Fatal(err)
	}
	session.Close()
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
				t.Cleanup(session.Close)
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
			t.Cleanup(session.Close)
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
				t.Cleanup(session.Close)
				if err != nil {
					t.Fatal(err)
				}
				results := session.Detach(config.ExpandPath)
				if len(results) != 1 || len(results[0].Kept) != 1 {
					t.Fatalf("detach=%+v", results)
				}
				RunExtraTargets(cfg.Extras[0], replacement, ExtraRunOptions{ResolvePath: config.ExpandPath, Force: true, Protected: session.Protected(), Shared: session})

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
	t.Cleanup(session.Close)
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
	t.Cleanup(session.Close)
	if err != nil {
		t.Fatal(err)
	}
	detached := session.Detach(config.ExpandPath)
	if len(detached) != 1 || len(detached[0].Kept) != 1 {
		t.Fatalf("detach=%+v", detached)
	}
	RunExtraTargets(cfg.Extras[0], replacement, ExtraRunOptions{ResolvePath: config.ExpandPath, Force: true, Protected: session.Protected(), Shared: session})
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
	t.Cleanup(session.Close)
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
	session.Close()
	retry, err := BeginSharedExtras(cfg, false)
	t.Cleanup(retry.Close)
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
			t.Cleanup(session.Close)
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
	t.Cleanup(session.Close)
	if err != nil {
		t.Fatal(err)
	}
	results := session.Detach(config.ExpandPath)
	if len(results) != 1 || len(results[0].Kept) != 1 {
		t.Fatalf("detach=%+v", results)
	}
	RunExtraTargets(cfg.Extras[0], replacement, ExtraRunOptions{ResolvePath: config.ExpandPath, Force: true, Protected: session.Protected(), Shared: session})
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
	t.Cleanup(session.Close)
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
	t.Cleanup(session.Close)
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
	RunExtraTargets(config.ExtraConfig{Targets: []config.ExtraTargetConfig{{Path: target, Mode: "copy"}}}, source, ExtraRunOptions{ResolvePath: config.ExpandPath, Force: true, Protected: session.Protected(), Shared: session})
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
	t.Cleanup(session.Close)
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

func TestSharedFallbackRecoveryRefusesMalformedNativeMetadata(t *testing.T) {
	restore := SetFileLinksForTest(false)
	defer restore()
	sb, cfg, source, target := sharedRunFixture(t, "merge", false)
	session, err := BeginSharedExtras(cfg, false)
	t.Cleanup(session.Close)
	if err != nil {
		t.Fatal(err)
	}
	session.state.SetWriteFaultForTest(func() error {
		if session.state.State.Pending == nil {
			return errors.New("promotion failed")
		}
		return nil
	})
	RunExtraTargets(cfg.Extras[0], source, ExtraRunOptions{ResolvePath: config.ExpandPath, Shared: session})
	session.Close()
	statePath, _ := config.SharedStatePath(sb.ConfigPath)
	before := sb.ReadFile(statePath)
	output := sb.ReadFile(filepath.Join(target, "fixture.md"))
	sb.WriteFile(filepath.Join(target, ManifestFile), "{malformed")
	if retry, err := BeginSharedExtras(cfg, false); err == nil {
		retry.Close()
		t.Fatal("ambiguous tracker was rebuilt during recovery")
	}
	if sb.ReadFile(statePath) != before || sb.ReadFile(filepath.Join(target, "fixture.md")) != output || sb.ReadFile(filepath.Join(target, ManifestFile)) != "{malformed" {
		t.Fatal("refusal changed evidence")
	}
}

func TestSharedReceiptRecoveryNativeModes(t *testing.T) {
	for _, single := range []bool{false, true} {
		for _, mode := range []string{"copy", "merge", "symlink", "import", "prepend", "append"} {
			if !single && (mode == "import" || isContentBlockMode(mode)) {
				continue
			}
			t.Run(fmt.Sprintf("single=%v/%s", single, mode), func(t *testing.T) {
				if (mode == "merge" || mode == "symlink") && !CanCreateFileLink() {
					t.Skip("link token coverage is root-hosted")
				}
				sb, cfg, source, target := sharedRunFixture(t, mode, single)
				output := filepath.Join(target, "fixture.md")
				if single {
					sb.WriteFile(output, "original user lines\n")
				}
				session, err := BeginSharedExtras(cfg, false)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(session.Close)
				session.state.SetWriteFaultForTest(func() error {
					if session.state.State.Pending == nil {
						return errors.New("promotion fault")
					}
					return nil
				})
				RunExtraTargets(cfg.Extras[0], source, ExtraRunOptions{ResolvePath: config.ExpandPath, Shared: session})
				if session.failed == nil || session.state.State.Pending == nil {
					t.Fatal("native completion did not leave durable recovery evidence")
				}
				session.Close()
				// Recovery uses saved output/metadata facts after both inputs disappear.
				if err := os.RemoveAll(source); err != nil {
					t.Fatal(err)
				}
				sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\nextras: []\n")
				cfg, err = config.LoadReadOnlyPath(sb.ConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				retry, err := BeginSharedExtras(cfg, false)
				if err != nil {
					t.Fatal(err)
				}
				defer retry.Close()
				if retry.state.State.Pending != nil {
					t.Fatal("receipt was not promoted before detach")
				}
				results := retry.Detach(config.ExpandPath)
				if len(results) != 1 || len(results[0].Errors) != 0 {
					t.Fatalf("detach=%+v", results)
				}
				if single {
					if got := sb.ReadFile(output); got != "original user lines\n" {
						t.Fatalf("restore=%q", got)
					}
				} else {
					if mode == "symlink" {
						output = target
					}
					if _, err := os.Lstat(output); !os.IsNotExist(err) {
						t.Fatalf("owned output remained: %v", err)
					}
				}
			})
		}
	}
}

func TestSharedIntentFailureAndAmbiguousRetryPreserveEvidence(t *testing.T) {
	for _, boundary := range []string{"intent", "content", "parent", "detach-checkpoint"} {
		t.Run(boundary, func(t *testing.T) {
			sb, cfg, source, target := sharedRunFixture(t, "copy", false)
			session, err := BeginSharedExtras(cfg, false)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(session.Close)
			session.state.SetWriteFaultForTest(func() error {
				if boundary == "intent" || session.state.State.Pending == nil {
					return errors.New("state fault")
				}
				return nil
			})
			RunExtraTargets(cfg.Extras[0], source, ExtraRunOptions{ResolvePath: config.ExpandPath, Shared: session})
			session.Close()
			output := filepath.Join(target, "fixture.md")
			if boundary == "intent" {
				if _, err := os.Lstat(output); !os.IsNotExist(err) {
					t.Fatal("intent fault allowed native placement")
				}
				return
			}
			if boundary == "detach-checkpoint" {
				retry, err := BeginSharedExtras(cfg, false)
				if err != nil {
					t.Fatal(err)
				}
				retry.Close()
				replacement := filepath.Join(sb.Root, "replacement")
				sb.WriteFile(filepath.Join(replacement, "fixture.md"), "replacement\n")
				shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
				sb.WriteFile(shared, strings.ReplaceAll(sb.ReadFile(shared), filepath.ToSlash(source), filepath.ToSlash(replacement)))
				cfg, err = config.LoadReadOnlyPath(sb.ConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				retry, err = BeginSharedExtras(cfg, false)
				if err != nil {
					t.Fatal(err)
				}
				defer retry.Close()
				retry.state.SetWriteFaultForTest(func() error { return errors.New("detach checkpoint fault") })
				if results := retry.Detach(config.ExpandPath); len(results) < 2 || retry.failed == nil {
					t.Fatalf("checkpoint=%+v", results)
				}
				RunExtraTargets(cfg.Extras[0], replacement, ExtraRunOptions{ResolvePath: config.ExpandPath, Force: true, Shared: retry})
				if _, err := os.Lstat(output); !os.IsNotExist(err) {
					t.Fatal("apply followed failed detach checkpoint")
				}
				return
			}
			if boundary == "content" {
				sb.WriteFile(output, "user changed it\n")
			} else {
				if !CanCreateFileLink() {
					t.Skip("requires directory links")
				}
				native := filepath.Join(sb.Root, "native")
				sb.WriteFile(filepath.Join(native, "fixture.md"), "managed content\n")
				if err := os.Rename(target, target+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(native, target); err != nil {
					t.Fatal(err)
				}
			}
			statePath, _ := config.SharedStatePath(sb.ConfigPath)
			before := sb.ReadFile(statePath)
			if retry, err := BeginSharedExtras(cfg, false); err == nil {
				retry.Close()
				t.Fatal("ambiguous recovery proceeded")
			}
			if sb.ReadFile(statePath) != before {
				t.Fatal("refusal discarded pending evidence")
			}
		})
	}
}

func TestSharedRecoveredTransformDoesNotExecuteTwice(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell transform fixture")
	}
	sb, _, source, target := sharedRunFixture(t, "copy", false)
	shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
	sb.WriteFile(shared, strings.ReplaceAll(sb.ReadFile(shared), "mode: copy", "mode: copy, flatten: true, extension: convert"))
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	count := filepath.Join(sb.Root, "extension-count")
	t.Setenv("SHARED_RECEIPT_COUNT", count)
	opts := ExtraRunOptions{ResolvePath: config.ExpandPath, Force: true, ResolveExtension: func(string) (*ExtensionSpec, error) {
		return &ExtensionSpec{Name: "convert", Dir: source, OutputExt: "txt", Run: []string{"sh", "-c", `printf 'run\n' >> "$SHARED_RECEIPT_COUNT"; cat`}}, nil
	}}
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	session.state.SetWriteFaultForTest(func() error {
		if session.state.State.Pending == nil {
			return errors.New("promotion fault")
		}
		return nil
	})
	opts.Shared = session
	RunExtraTargets(cfg.Extras[0], source, opts)
	session.Close()
	retry, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer retry.Close()
	opts.Shared = retry
	RunExtraTargets(retry.Config().Extras[0], source, opts)
	if sb.ReadFile(count) != "run\n" || sb.ReadFile(filepath.Join(target, "fixture.txt")) != "managed content\n" {
		t.Fatal("recovery reran the extension or lost frozen output")
	}
}

func TestSharedIdempotentAdoptionRequiresNativeRestoreProof(t *testing.T) {
	for _, mode := range []string{"copy", "merge"} {
		for _, proof := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/proof=%v", mode, proof), func(t *testing.T) {
				if mode == "merge" && !CanCreateFileLink() {
					t.Skip("requires file links")
				}
				sb, cfg, source, target := sharedRunFixture(t, mode, true)
				f := NewExtraFile(source, "fixture.md", target, "", mode)
				if proof {
					if _, err := SyncExtraFile(f, false, ""); err != nil {
						t.Fatal(err)
					}
				} else if mode == "copy" {
					sb.WriteFile(f.Target, "managed content\n")
				} else {
					if err := os.MkdirAll(target, 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(f.Source, f.Target); err != nil {
						t.Fatal(err)
					}
				}
				session, err := BeginSharedExtras(cfg, false)
				if err != nil {
					t.Fatal(err)
				}
				defer session.Close()
				RunExtraTargets(cfg.Extras[0], source, ExtraRunOptions{ResolvePath: config.ExpandPath, Force: true, Shared: session})
				if proof && (session.failed != nil || len(session.state.State.Extras) != 1) {
					t.Fatalf("proven output not adopted: %v", session.failed)
				}
				if !proof && (len(session.state.State.Extras) != 0 || session.state.State.Pending != nil || mode == "merge" && session.failed == nil) {
					t.Fatal("unproven native output acquired or unmarked link not reported")
				}
				if sb.ReadFile(f.Target) != "managed content\n" {
					t.Fatal("adoption changed native content")
				}
			})
		}
	}
}

func TestSharedPartialPromotionRetainsSuccessfulAndSkippedOutputs(t *testing.T) {
	sb, cfg, source, target := sharedRunFixture(t, "copy", false)
	if err := os.Rename(filepath.Join(source, "fixture.md"), filepath.Join(source, "a.md")); err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(filepath.Join(source, "b.md"), "shared b\n")
	sb.WriteFile(filepath.Join(target, "b.md"), "user b\n")
	sb.WriteFile(filepath.Join(source, "z.md"), "shared z\n")
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	session.state.SetWriteFaultForTest(func() error {
		if session.state.State.Pending == nil && len(session.state.State.Extras) > 0 && len(session.state.State.Extras[0].Receipt.Targets[0].Files) == 2 {
			return errors.New("second receipt fault")
		}
		return nil
	})
	RunExtraTargets(cfg.Extras[0], source, ExtraRunOptions{ResolvePath: config.ExpandPath, Shared: session})
	session.Close()
	state, err := config.LoadSharedState(sb.ConfigPath, cfg.EffectiveGitRoot(), filepath.Dir(cfg.SharedStatus().Path))
	if err != nil {
		t.Fatal(err)
	}
	if state.State.Pending == nil || len(state.State.Extras) != 1 || len(state.State.Extras[0].Receipt.Targets[0].Files) != 1 {
		t.Fatal("later failure lost prior receipt")
	}
	statePath, _ := config.SharedStatePath(sb.ConfigPath)
	before := sb.ReadFile(statePath)
	if other, err := BeginSharedExtras(cfg, false, SharedExtrasScope{Name: "unselected"}); err == nil {
		other.Close()
		t.Fatal("out-of-scope pending recovered")
	}
	if sb.ReadFile(statePath) != before {
		t.Fatal("selected refusal changed evidence")
	}
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\n")
	cfg, err = config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer retry.Close()
	retry.Detach(config.ExpandPath)
	for _, name := range []string{"a.md", "z.md"} {
		if _, err := os.Lstat(filepath.Join(target, name)); !os.IsNotExist(err) {
			t.Fatalf("partial output remains: %s", name)
		}
	}
	if sb.ReadFile(filepath.Join(target, "b.md")) != "user b\n" {
		t.Fatal("skipped user output was detached")
	}
}

func TestSharedForeignLinkReferentRecovery(t *testing.T) {
	if !CanCreateFileLink() {
		t.Skip("requires file links")
	}
	for _, mode := range []string{"import", "append", "copy"} {
		for _, edited := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/edited=%v", mode, edited), func(t *testing.T) {
				sb, cfg, source, target := sharedRunFixture(t, mode, true)
				f := NewExtraFile(source, "fixture.md", target, "", mode)
				referent := filepath.Join(sb.Root, "own.md")
				base := "own lines\n"
				content := base
				if mode == "copy" {
					content, _ = addImportLine(base, f.importLine())
				}
				sb.WriteFile(referent, content)
				if err := os.MkdirAll(target, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(referent, f.Target); err != nil {
					t.Fatal(err)
				}
				session, err := BeginSharedExtras(cfg, false)
				if err != nil {
					t.Fatal(err)
				}
				session.state.SetWriteFaultForTest(func() error {
					if session.state.State.Pending == nil {
						return errors.New("receipt fault")
					}
					return nil
				})
				RunExtraTargets(cfg.Extras[0], source, ExtraRunOptions{ResolvePath: config.ExpandPath, Shared: session})
				if session.failed == nil {
					t.Fatal("missing receipt fault")
				}
				session.Close()
				if mode == "copy" && sb.ReadFile(referent) != base {
					t.Fatal("leaving import did not strip old referent")
				}
				if edited {
					sb.WriteFile(referent, "external edit\n")
				}
				statePath, _ := config.SharedStatePath(sb.ConfigPath)
				before := sb.ReadFile(statePath)
				sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\n")
				cfg, err = config.LoadReadOnlyPath(sb.ConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				retry, err := BeginSharedExtras(cfg, false)
				if edited {
					if err == nil {
						retry.Close()
						t.Fatal("changed referent acquired")
					}
					if sb.ReadFile(statePath) != before || sb.ReadFile(referent) != "external edit\n" {
						t.Fatal("refusal changed evidence or referent")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				defer retry.Close()
				for _, result := range retry.Detach(config.ExpandPath) {
					if len(result.Errors) != 0 {
						t.Fatal(result.Errors)
					}
				}
				if sb.ReadFile(referent) != base {
					t.Fatal("detach changed user referent")
				}
				if raw, err := os.Readlink(f.Target); err != nil || raw != referent {
					t.Fatalf("user link not preserved/restored: %q %v", raw, err)
				}
			})
		}
	}
}

func TestSharedBeforeStateCancelsOnlyAttemptMetadata(t *testing.T) {
	_, cfg, source, target := sharedRunFixture(t, "copy", true)
	session, err := BeginSharedExtras(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	f := NewExtraFile(source, "fixture.md", target, "", "copy")
	place := session.writer(cfg.Extras[0], source, config.SharedPlacedTarget{Path: target, Mode: "copy"})
	fault := errors.New("native stopped before output")
	err = place(placementChange{path: f.Target, before: "absent", after: extraContentFingerprint([]byte("managed content\n")), metadata: map[string]string{attachCreated: extraContentFingerprint(nil)}}, func() error {
		if err := markExtraCreated(f.Target); err != nil {
			return err
		}
		return fault
	})
	if !errors.Is(err, fault) {
		t.Fatalf("native error lost: %v", err)
	}
	if session.state.State.Pending != nil || len(session.state.State.Extras) != 0 || extraAttached(f.Target) {
		t.Fatal("unused intent acquired output or left attempt metadata")
	}
	if _, err := os.Lstat(f.Target); !os.IsNotExist(err) {
		t.Fatal("cancellation wrote output")
	}
}
