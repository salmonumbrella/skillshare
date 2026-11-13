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
