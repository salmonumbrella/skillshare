package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

func TestSharedGlobalExtrasLastRemoval(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		t.Run(fmt.Sprint(jsonOutput), func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
			source := filepath.Join(sb.Root, "rules")
			target := filepath.Join(sb.Root, "placed")
			sb.WriteFile(filepath.Join(source, "fixture.md"), "managed\n")
			sb.WriteFile(shared, fmt.Sprintf("version: 1\nextras: [{name: rules,source: %q,targets: [{path: %q,mode: copy}]}]\n", filepath.ToSlash(source), filepath.ToSlash(target)))
			sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {}\n")
			if err := cmdSyncExtrasGlobal(false, false, jsonOutput, false, time.Now()); err != nil {
				t.Fatal(err)
			}
			sb.WriteFile(shared, "version: 1\nextras: []\n")
			if err := cmdSyncExtrasGlobal(false, false, jsonOutput, false, time.Now()); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(target, "fixture.md")); !os.IsNotExist(err) {
				t.Fatal("last placement was not detached")
			}
		})
	}
}

func TestSharedDoctorAvailability(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./missing.yaml}\ntargets: {}\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	result := &doctorResult{}
	checkSharedFile(cfg, result)
	if len(result.checks) != 1 || result.checks[0].Name != "shared_file" || result.checks[0].Status != checkError {
		t.Fatalf("checks=%+v", result.checks)
	}
}

func TestTUISharedExtraSyncDetachesSelectedReplacement(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, target := filepath.Join(sb.Root, "rules"), filepath.Join(sb.Root, "placed")
	shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
	sb.WriteFile(filepath.Join(source, "fixture.md"), "managed\n")
	sb.WriteFile(shared, fmt.Sprintf("version: 1\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: copy}]}]\n", filepath.ToSlash(source), filepath.ToSlash(target)))
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {}\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	model := extrasListTUIModel{cfg: cfg}
	if _, err := model.doSync("rules", target); err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(filepath.Join(target, "fixture.md"), "edited\n")
	replacement := filepath.Join(sb.Root, "replacement")
	sb.WriteFile(filepath.Join(replacement, "fixture.md"), "replacement\n")
	sb.WriteFile(shared, strings.ReplaceAll(sb.ReadFile(shared), filepath.ToSlash(source), filepath.ToSlash(replacement)))
	message, err := model.doSync("rules", target)
	if err != nil || !strings.Contains(message, "kept 1") {
		t.Fatalf("sync=%q %v", message, err)
	}
	if sb.ReadFile(filepath.Join(target, "fixture.md")) != "edited\n" {
		t.Fatal("TUI apply overwrote protected output")
	}
}

func TestTUISharedSyncRefusesRemovedSelectedTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, old, current := filepath.Join(sb.Root, "rules"), filepath.Join(sb.Root, "old"), filepath.Join(sb.Root, "current")
	shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
	sb.WriteFile(filepath.Join(source, "fixture.md"), "managed\n")
	sb.WriteFile(shared, fmt.Sprintf("version: 1\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: copy}]}]\n", filepath.ToSlash(source), filepath.ToSlash(old)))
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {}\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	model := extrasListTUIModel{cfg: cfg}
	sb.WriteFile(shared, strings.ReplaceAll(sb.ReadFile(shared), filepath.ToSlash(old), filepath.ToSlash(current)))
	if _, err := model.doSync("rules", old); err == nil {
		t.Fatal("removed selection applied to all refreshed targets")
	}
	if _, err := os.Lstat(current); !os.IsNotExist(err) {
		t.Fatal("unselected target changed")
	}
	// The refused selection must release its session for the next action.
	if _, err := model.doSync("rules", current); err != nil {
		t.Fatal(err)
	}
	if sb.ReadFile(filepath.Join(current, "fixture.md")) != "managed\n" {
		t.Fatal("fresh selection not applied")
	}
}

func TestSharedJSONAllRefreshesAndReleases(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, target := filepath.Join(sb.Root, "rules"), filepath.Join(sb.Root, "placed")
	shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
	sb.WriteFile(shared, "version: 1\n")
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {}\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(filepath.Join(source, "fixture.md"), "managed\n")
	sb.WriteFile(shared, fmt.Sprintf("version: 1\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: copy}]}]\n", filepath.ToSlash(source), filepath.ToSlash(target)))
	entries, err := runGlobalExtrasSyncEntries(cfg, false, false, nil)
	if err != nil || len(entries) != 1 {
		t.Fatalf("fresh JSON-all=%+v %v", entries, err)
	}
	if sb.ReadFile(filepath.Join(target, "fixture.md")) != "managed\n" {
		t.Fatal("cached declarations used")
	}
	sb.WriteFile(shared, "version: 1\n")
	if _, err := runGlobalExtrasSyncEntries(cfg, false, false, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(target, "fixture.md")); !os.IsNotExist(err) {
		t.Fatal("released retry did not detach")
	}
}

func TestSharedAdaptersPartialFailureRetainsReceiptAndReleases(t *testing.T) {
	for _, adapter := range []string{"json-all", "tui-target"} {
		t.Run(adapter, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			source, good := filepath.Join(sb.Root, "rules"), filepath.Join(sb.Root, "good")
			blocked := filepath.Join(sb.Root, "blocked")
			bad := filepath.Join(blocked, "child")
			shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
			sb.WriteFile(filepath.Join(source, "fixture.md"), "managed\n")
			sb.WriteFile(blocked, "user-owned parent\n")
			sb.WriteFile(shared, fmt.Sprintf("version: 1\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: copy}, {path: %q, mode: copy}]}]\n", filepath.ToSlash(source), filepath.ToSlash(good), filepath.ToSlash(bad)))
			sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {}\n")
			cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			model := extrasListTUIModel{cfg: cfg}
			if adapter == "json-all" {
				entries, err := runGlobalExtrasSyncEntries(cfg, false, false, nil)
				if err == nil || len(entries) != 1 || len(entries[0].Targets) != 2 || entries[0].Targets[0].Synced != 1 || entries[0].Targets[1].Error == "" {
					t.Fatalf("partial JSON=%+v %v", entries, err)
				}
			} else {
				if _, err := model.doSync("rules", bad); err == nil {
					t.Fatal("TUI hid selected target failure")
				}
				if _, err := os.Lstat(good); !os.IsNotExist(err) {
					t.Fatal("target-only failure touched unselected output")
				}
				if _, err := model.doSync("rules", good); err != nil {
					t.Fatal("failed TUI action leaked lease", err)
				}
			}
			state, err := config.LoadSharedState(sb.ConfigPath, cfg.EffectiveGitRoot(), filepath.Dir(shared))
			if err != nil {
				t.Fatal(err)
			}
			if state.State.Pending != nil || len(state.State.Extras) != 1 || state.State.Extras[0].Receipt.Targets[0].Path != good {
				t.Fatal("adapter lost successful receipt or acquired failed output")
			}
			if sb.ReadFile(blocked) != "user-owned parent\n" {
				t.Fatal("adapter changed failed target parent")
			}
			sb.WriteFile(shared, "version: 1\n")
			if _, err := runGlobalExtrasSyncEntries(cfg, false, true, nil); err != nil {
				t.Fatal("fresh retry after adapter error failed", err)
			}
			if _, err := os.Lstat(filepath.Join(good, "fixture.md")); !os.IsNotExist(err) {
				t.Fatal("successful subset was not detached")
			}
		})
	}
}
