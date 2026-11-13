package config

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"skillshare/internal/testutil"
)

func fixtureSharedIntent(t *testing.T, sb *testutil.Sandbox) *SharedExtraIntent {
	t.Helper()
	target := filepath.Join(sb.Home, "native")
	return &SharedExtraIntent{Placement: SharedPlacement{
		Name: "fromEnv", Declaration: fixtureDeclaration(t), DeclarationDir: filepath.Dir(sb.ConfigPath),
		Receipt: SharedReceipt{Source: filepath.Join(sb.Home, "rules"), Targets: []SharedPlacedTarget{{Path: target, Mode: "copy"}}},
	}, Relative: "guide.md", Effects: []SharedExtraEffect{{Path: filepath.Join(target, "guide.md"), Before: "absent", After: "file:" + strings.Repeat("a", 64)}}}
}

func TestSharedStateLeaseSerializesAndReleases(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	first, release, err := BeginSharedStateWrite(context.Background(), sb.ConfigPath, sb.SourcePath, "")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, unlock, err := BeginSharedStateWrite(ctx, sb.ConfigPath, sb.SourcePath, ""); err == nil {
		unlock()
		t.Fatal("competing lease succeeded")
	}
	// A different config identity is independent while the first is leased.
	_, otherRelease, err := BeginSharedStateWrite(context.Background(), filepath.Join(sb.Root, "other.yaml"), sb.SourcePath, "")
	if err != nil {
		t.Fatal(err)
	}
	otherRelease()
	first.State.Pending = fixtureSharedIntent(t, sb)
	if err := first.Save(); err != nil {
		t.Fatal(err)
	}
	release()
	release()
	if err := first.Save(); err == nil {
		t.Fatal("closed lease allowed a write")
	}
	next, closeNext, err := BeginSharedStateWrite(context.Background(), sb.ConfigPath, sb.SourcePath, "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeNext()
	if next.State.Pending == nil {
		t.Fatal("leased reload lost pending receipt")
	}
	path, _ := SharedStatePath(sb.ConfigPath)
	if _, err := os.Stat(strings.TrimSuffix(path, ".yaml") + ".lock"); err != nil {
		t.Fatal("stable lock inode missing", err)
	}
}

func TestSharedStateLeaseAliasAndStaleCandidates(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	stale, err := LoadSharedState(sb.ConfigPath, sb.SourcePath, "")
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := stale.stateCandidate()
	if err != nil {
		t.Fatal(err)
	}
	leased, release, err := BeginSharedStateWrite(context.Background(), sb.ConfigPath, sb.SourcePath, "")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	alias := filepath.Join(sb.Root, "alias.yaml")
	if err := os.Symlink(sb.ConfigPath, alias); err != nil {
		t.Skip("file links unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, unlock, err := BeginSharedStateWrite(ctx, alias, sb.SourcePath, ""); !errors.Is(err, context.DeadlineExceeded) {
		if err == nil {
			unlock()
		}
		t.Fatalf("alias did not contend: %v", err)
	}
	leased.RecordPointer(filepath.Join(sb.Root, "team.yaml"))
	if err := leased.Save(); err != nil {
		t.Fatal(err)
	}
	release()
	if err := stale.Save(); err == nil {
		t.Fatal("stale direct save overwrote state")
	}
	if err := candidate.Write(); err == nil {
		t.Fatal("prepared bytes silently refreshed")
	}
}

func TestSharedIntentStrictOptionalFields(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	p := fixtureSharedIntent(t, sb)
	state := SharedState{Version: 1, Pending: p}
	raw, err := yaml.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeSharedState(raw); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("managed content")) {
		t.Fatal("payload in private intent")
	}
	for _, corrupt := range []func(*SharedExtraIntent){
		func(p *SharedExtraIntent) { p.Relative = "../escape" },
		func(p *SharedExtraIntent) { p.Effects[0].Path = filepath.Join(sb.Root, "elsewhere") },
		func(p *SharedExtraIntent) { p.Effects[0].After = "file:invalid" },
		func(p *SharedExtraIntent) { p.Placement.DeclarationDir = "relative" },
		func(p *SharedExtraIntent) { p.MetadataAfter = map[string]string{"unrecognized": "absent"} },
	} {
		bad := fixtureSharedIntent(t, sb)
		corrupt(bad)
		data, err := yaml.Marshal(SharedState{Version: 1, Pending: bad})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decodeSharedState(data); err == nil {
			t.Fatalf("invalid intent accepted: %s", data)
		}
	}
}

func fixtureDeclaration(t *testing.T) yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte("# declaration\nname: fromEnv\nsource: ~/rules\ntargets: [{path: ./rules}]\n"), &doc); err != nil {
		t.Fatal(err)
	}
	return *doc.Content[0]
}

func TestSharedStateLocationAndIdentity(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("LOCALAPPDATA", filepath.Join(sb.Home, "local"))
	path, err := sharedStatePath(sb.ConfigPath, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, filepath.Join(sb.Home, "local", "skillshare", "state", "shared")+string(filepath.Separator)) {
		t.Fatal(path)
	}
	other, err := sharedStatePath(filepath.Join(sb.Root, "other.yaml"), "windows")
	if err != nil || other == path {
		t.Fatalf("other=%s err=%v", other, err)
	}
	sb.WriteConfig("targets: {}\n")
	alias := filepath.Join(sb.Root, "alias.yaml")
	if err := os.Symlink(sb.ConfigPath, alias); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks unavailable")
		}
		t.Fatal(err)
	}
	linked, err := sharedStatePath(alias, "windows")
	if err != nil || linked != path {
		t.Fatalf("linked=%s err=%v", linked, err)
	}
}

func TestSharedStateStrictPrivateAndStale(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sharedDir := filepath.Join(sb.Root, "shared")
	state, err := LoadSharedState(sb.ConfigPath, sb.SourcePath, sharedDir)
	if err != nil {
		t.Fatal(err)
	}
	state.RecordPointer(filepath.Join(sharedDir, "team.yaml"))
	state.State.Extras = []SharedPlacement{{Name: "fromEnv", Declaration: fixtureDeclaration(t), Receipt: SharedReceipt{Source: filepath.Join(sb.Home, "rules"), Targets: []SharedPlacedTarget{{Path: filepath.Join(sb.Home, "target"), Mode: "copy", Files: map[string]string{"fixture.md": strings.Repeat("a", 64)}}}}}}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	path, err := SharedStatePath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("source: ~/rules")) || !bytes.Contains(raw, []byte("path: ./rules")) || bytes.Contains(raw, []byte("# declaration")) {
		t.Fatalf("declaration was resolved or comments retained:\n%s", raw)
	}
	if runtime.GOOS != "windows" {
		for _, tc := range []struct {
			path string
			mode os.FileMode
		}{{filepath.Dir(path), 0700}, {path, 0600}} {
			info, err := os.Stat(tc.path)
			if err != nil || info.Mode().Perm() != tc.mode {
				t.Fatalf("mode %s: %v %v", tc.path, info, err)
			}
		}
	}
	loaded, err := LoadSharedState(sb.ConfigPath, sb.SourcePath, sharedDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.State.Extras) != 1 || loaded.State.Extras[0].Declaration.Content[3].Value != "~/rules" {
		t.Fatal("raw declaration lost")
	}
	if err := os.WriteFile(path, append(raw, []byte("# changed\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loaded.Save(); err == nil {
		t.Fatal("stale state replaced")
	}
	for _, bad := range []string{"version: 2\n", "version: 1\nunknown: true\n", "[", "version: 1\nextras: [{name: fromEnv, declaration: {name: fromEnv, remote: forbidden}, receipt: {}}]\n"} {
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadSharedState(sb.ConfigPath, sb.SourcePath, sharedDir); err == nil {
			t.Fatalf("invalid state accepted: %s", bad)
		}
		if got, _ := os.ReadFile(path); string(got) != bad {
			t.Fatal("load wrote state")
		}
	}
}

func TestSharedStateRefusesUnsafeLocationBeforeCreating(t *testing.T) {
	for _, scope := range []string{"git", "shared", "linked ancestor"} {
		t.Run(scope, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			root := filepath.Join(sb.Root, "published")
			if err := os.MkdirAll(root, 0755); err != nil {
				t.Fatal(err)
			}
			stateRoot := filepath.Join(root, "state")
			if scope == "linked ancestor" {
				link := filepath.Join(sb.Root, "redirect")
				if err := os.Symlink(root, link); err != nil {
					if runtime.GOOS == "windows" {
						t.Skip("symlinks unavailable")
					}
					t.Fatal(err)
				}
				stateRoot = filepath.Join(link, "missing", "state")
			}
			t.Setenv("XDG_STATE_HOME", stateRoot)
			gitRoot, sharedDir := root, filepath.Join(sb.Root, "elsewhere")
			if scope == "shared" {
				gitRoot, sharedDir = sharedDir, root
			}
			path, _ := SharedStatePath(sb.ConfigPath)
			_, err := LoadSharedState(sb.ConfigPath, gitRoot, sharedDir)
			if err == nil || !strings.Contains(err.Error(), filepath.Dir(path)) || !strings.Contains(err.Error(), root) {
				t.Fatalf("unsafe state error: %v", err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("unsafe state created")
			}
		})
	}
}

func TestSharedStateRefusesSymlink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	path, _ := SharedStatePath(sb.ConfigPath)
	sb.WriteFile(filepath.Join(sb.Root, "other.yaml"), "version: 1\n")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(sb.Root, "other.yaml"), path); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks unavailable")
		}
		t.Fatal(err)
	}
	if _, err := LoadSharedState(sb.ConfigPath, sb.SourcePath, filepath.Join(sb.Root, "team")); err == nil {
		t.Fatal("followed state symlink")
	}
}

func TestSharedPointerHistoryAllWriters(t *testing.T) {
	for _, writer := range []string{"save", "prepare", "raw"} {
		for _, transition := range []string{"change", "remove", "set"} {
			t.Run(writer+"/"+transition, func(t *testing.T) {
				sb, cfg := sharedFixture(t, "", "version: 1\n")
				old := resolvedSharedPointer(sb.ConfigPath, cfg.Shared)
				next := filepath.Join(sb.Root, "other", "team.yaml")
				sb.WriteFile(next, "version: 1\n")
				state, err := LoadSharedState(sb.ConfigPath, sb.SourcePath, filepath.Dir(old))
				if err != nil {
					t.Fatal(err)
				}
				state.State.Extras = []SharedPlacement{{Name: "fromEnv", Declaration: fixtureDeclaration(t), Receipt: SharedReceipt{Source: filepath.Join(sb.Home, "rules")}}}
				if err := state.Save(); err != nil {
					t.Fatal(err)
				}
				if transition == "set" {
					sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\ntargets: {}\n")
					cfg, err = LoadReadOnlyPath(sb.ConfigPath)
					if err != nil {
						t.Fatal(err)
					}
					old = ""
				}
				if transition == "remove" {
					cfg.Shared = nil
					next = ""
				} else {
					cfg.Shared = &SharedPointer{Path: filepath.ToSlash(next)}
				}
				switch writer {
				case "save":
					err = cfg.Save()
				case "prepare":
					var candidate *SaveCandidate
					candidate, err = PrepareSave(sb.ConfigPath, cfg)
					if err == nil {
						err = candidate.Write()
					}
				case "raw":
					raw := "# exact raw editor bytes\nsource: " + filepath.ToSlash(sb.SourcePath) + "\ntargets: {}\n"
					if next != "" {
						raw += "shared: {path: '" + filepath.ToSlash(next) + "'}\n"
					}
					var candidate *SaveCandidate
					candidate, err = PrepareRawConfigSave(sb.ConfigPath, []byte(raw))
					if err == nil {
						err = candidate.Write()
					}
					if err == nil && sb.ReadFile(sb.ConfigPath) != raw {
						t.Fatal("raw bytes changed")
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				loaded, err := LoadSharedState(sb.ConfigPath, sb.SourcePath, filepath.Join(sb.Root, "team"))
				if err != nil {
					t.Fatal(err)
				}
				for _, p := range []string{old, next} {
					if p != "" && !slices.Contains(loaded.State.Pointers, p) {
						t.Fatalf("forgot %s: %v", p, loaded.State.Pointers)
					}
				}
				if loaded.State.Pointer != next || len(loaded.State.Extras) != 1 {
					t.Fatalf("history/placements=%+v", loaded.State)
				}
			})
		}
	}
}

func TestSharedPointerSnapshotFailuresLeaveConfig(t *testing.T) {
	for _, failure := range []string{"state stale", "config stale", "unsafe", "state malformed"} {
		t.Run(failure, func(t *testing.T) {
			sb, cfg := sharedFixture(t, "", "version: 1\n")
			cfg.Shared = &SharedPointer{Path: filepath.Join(sb.Root, "new", "team.yaml")}
			if failure == "unsafe" {
				t.Setenv("XDG_STATE_HOME", sb.SourcePath)
			}
			if failure == "state malformed" {
				path, _ := SharedStatePath(sb.ConfigPath)
				sb.WriteFile(path, "version: 2\n")
			}
			candidate, err := PrepareSave(sb.ConfigPath, cfg)
			if failure == "unsafe" || failure == "state malformed" {
				if err == nil {
					t.Fatal("unsafe state prepared")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if failure == "state stale" {
				path, _ := SharedStatePath(sb.ConfigPath)
				sb.WriteFile(path, "version: 1\n# changed\n")
			}
			if failure == "config stale" {
				sb.WriteFile(sb.ConfigPath, sb.ReadFile(sb.ConfigPath)+"# changed\n")
			}
			before := sb.ReadFile(sb.ConfigPath)
			info, _ := os.Stat(sb.ConfigPath)
			if err := candidate.Write(); err == nil {
				t.Fatal("stale write accepted")
			}
			after, _ := os.Stat(sb.ConfigPath)
			if sb.ReadFile(sb.ConfigPath) != before || !info.ModTime().Equal(after.ModTime()) {
				t.Fatal("failed history write replaced config")
			}
		})
	}
}

func TestSharedEqualPointerAndNoPointerNeverAccessState(t *testing.T) {
	sb, cfg := sharedFixture(t, "", "version: 1\n")
	path, _ := SharedStatePath(sb.ConfigPath)
	sb.WriteFile(path, "malformed state must not be read")
	cfg.Shared.Path = filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
	candidate, err := PrepareSave(sb.ConfigPath, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.before != nil {
		t.Fatal("equal resolved pointer recorded")
	}
	if err := candidate.Write(); err != nil {
		t.Fatal(err)
	}
	if sb.ReadFile(path) != "malformed state must not be read" {
		t.Fatal("state accessed")
	}
	// Both pointers absent retain the base save and raw candidate paths as well.
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\ntargets: {}\n")
	cfg, err = LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	raw := sb.ReadFile(sb.ConfigPath)
	candidate, err = PrepareRawConfigSave(sb.ConfigPath, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if candidate.before != nil {
		t.Fatal("no-pointer state accessed")
	}
	if err := candidate.Write(); err != nil {
		t.Fatal(err)
	}
	if sb.ReadFile(path) != "malformed state must not be read" {
		t.Fatal("state touched")
	}
}

func TestSharedRelativePointerUsesConfigLinkFolder(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	real := filepath.Join(sb.Root, "real", "config.yaml")
	sb.WriteFile(real, "source: "+filepath.ToSlash(sb.SourcePath)+"\nshared: {path: ./team.yaml}\n")
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), "version: 1\nextras: [{name: rules, targets: []}]\n")
	if err := os.Symlink(real, sb.ConfigPath); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks unavailable")
		}
		t.Fatal(err)
	}
	cfg, err := LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.SharedStatus().Usable || len(cfg.Extras) != 1 {
		t.Fatal("pointer resolved beside destination rather than config link")
	}
	cfg.Shared = &SharedPointer{Path: filepath.Join(sb.Root, "elsewhere", "team.yaml")}
	candidate, err := PrepareSave(sb.ConfigPath, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := candidate.Write(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(sb.ConfigPath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("config link replaced")
	}
}

func TestSharedRawRepairMalformedOldConfig(t *testing.T) {
	for _, old := range []string{
		"targets: [invalid YAML\n",
		"shared: {path: 42}\ntargets: {}\n",
		"# shared: {path: ignored.yaml}\ntargets: [invalid YAML\n",
		"note: \"shared: {path: ignored.yaml}\"\ntargets: [invalid YAML\n",
		"note: {shared: {path: ignored.yaml}}\ntargets: [invalid YAML\n",
		"shared: {path: ./unobserved.yaml}\ntargets: [invalid YAML\n",
	} {
		for _, newPointer := range []bool{false, true} {
			t.Run(old+"/"+map[bool]string{false: "no pointer", true: "new pointer"}[newPointer], func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				path, _ := SharedStatePath(sb.ConfigPath)
				prior := filepath.Join(sb.Root, "recorded", "team.yaml")
				if newPointer {
					state, err := LoadSharedState(sb.ConfigPath, sb.SourcePath, filepath.Dir(prior))
					if err != nil {
						t.Fatal(err)
					}
					state.RecordPointer(prior)
					state.State.Extras = []SharedPlacement{{Name: "fromEnv", Declaration: fixtureDeclaration(t), Receipt: SharedReceipt{Source: filepath.Join(sb.Home, "rules")}}}
					if err := state.Save(); err != nil {
						t.Fatal(err)
					}
				} else {
					sb.WriteFile(path, "state must not be read")
				}
				sb.WriteConfig(old)
				raw := "# exact repaired bytes\nsource: " + filepath.ToSlash(sb.SourcePath) + "\ntargets: {}\n"
				next := filepath.Join(sb.Root, "new", "team.yaml")
				if newPointer {
					sb.WriteFile(next, "version: 1\n")
					raw += "shared: {path: '" + filepath.ToSlash(next) + "'}\n"
				}
				candidate, err := PrepareRawConfigSave(sb.ConfigPath, []byte(raw))
				if err != nil {
					t.Fatal(err)
				}
				if err := candidate.Write(); err != nil {
					t.Fatal(err)
				}
				if sb.ReadFile(sb.ConfigPath) != raw {
					t.Fatal("repair changed submitted bytes")
				}
				if !newPointer {
					if candidate.before != nil || sb.ReadFile(path) != "state must not be read" {
						t.Fatal("no-pointer repair accessed state")
					}
					return
				}
				state, err := LoadSharedState(sb.ConfigPath, sb.SourcePath, filepath.Dir(next))
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(state.State.Pointers, []string{prior, next}) || state.State.Pointer != next || len(state.State.Extras) != 1 {
					t.Fatalf("repair history=%+v", state.State)
				}
			})
		}
	}
}

func TestSharedStateWindowsRootNeverStagesState(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows roaming/local layout")
	}
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("APPDATA", filepath.Join(sb.Home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(sb.Home, "AppData", "Local"))
	sb.ConfigPath = filepath.Join(BaseDir(), "config.yaml")
	sb.SetEnv("SKILLSHARE_CONFIG", sb.ConfigPath)
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\ngit_root: root\nshared: {path: ./team.yaml}\n")
	shared := filepath.Join(BaseDir(), "team.yaml")
	sb.WriteFile(shared, "version: 1\n")
	cfg, err := LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	state, err := LoadSharedState(sb.ConfigPath, cfg.EffectiveGitRoot(), filepath.Dir(shared))
	if err != nil {
		t.Fatal(err)
	}
	state.RecordPointer(shared)
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, cfg.EffectiveGitRoot(), "init")
	testutil.RunGit(t, cfg.EffectiveGitRoot(), "add", "-A")
	tracked := testutil.RunGit(t, cfg.EffectiveGitRoot(), "ls-files")
	if !strings.Contains(tracked, "team.yaml") || strings.Contains(tracked, "state/") || strings.Contains(tracked, "shared/") {
		t.Fatal(tracked)
	}
}
