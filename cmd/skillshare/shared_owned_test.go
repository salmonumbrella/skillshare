package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

// Refusals must happen before source collection, restores, backups or logs.
func TestSharedOwnedMutatorsHaveNoSideEffects(t *testing.T) {
	for _, action := range []string{"init", "remove", "remove-target", "add-target", "mode", "collect", "tui-mode", "tui-remove", "tui-collect", "tui-flatten"} {
		t.Run(action, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			source, target := filepath.Join(sb.Root, "rules"), filepath.Join(sb.Root, "target")
			sb.WriteFile(filepath.Join(source, "rules.md"), "shared source\n")
			sb.WriteFile(filepath.Join(target, "rules.md"), "local content\n")
			shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
			sb.WriteFile(shared, fmt.Sprintf("version: 1\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: copy}]}]\n", filepath.ToSlash(source), filepath.ToSlash(target)))
			sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {}\n")
			cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			before := sharedMutationSnapshot(t, sb.Root)
			m := extrasListTUIModel{cfg: cfg}
			switch action {
			case "init":
				err = extrasInitGlobal(extrasInitOptions{name: "rules", force: true, targets: []extrasInitTarget{{path: target}}}, time.Now())
			case "remove":
				_, err = removeExtraFromGlobalConfig(cfg, "rules")
			case "remove-target":
				err = cmdExtrasRemoveTarget([]string{"rules", "--remove-target", target, "--prune", "-g"})
			case "add-target":
				err = cmdExtrasAddTarget([]string{"rules", "--add-target", filepath.Join(sb.Root, "other"), "-g"})
			case "mode":
				err = cmdExtrasMode([]string{"rules", "--mode", "copy", "-g"})
			case "collect":
				err = extrasCollectGlobal("rules", target, false, true, time.Now())
			case "tui-mode":
				_, err = m.doSetMode("rules", target, "copy")
			case "tui-remove":
				_, err = m.doRemove("rules")
			case "tui-collect":
				_, err = m.doCollect("rules", target)
			case "tui-flatten":
				err = m.doFlattenToggle("rules", extrasTargetInfo{Path: target, Mode: "copy"})().(extrasActionDoneMsg).err
			}
			var sharedErr *config.SharedError
			if !errors.As(err, &sharedErr) || sharedErr.Code != config.SharedOwned {
				t.Fatalf("error = %v", err)
			}
			if after := sharedMutationSnapshot(t, sb.Root); !reflect.DeepEqual(before, after) {
				t.Fatalf("refusal changed files: before=%v after=%v", before, after)
			}
		})
	}
}

func sharedMutationSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += link
		} else if !d.IsDir() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf("%x", sha256.Sum256(raw))
		}
		out[path] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSharedUnavailableApplyEntrypoints(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		sb := testutil.NewSandbox(t)
		sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./missing.yaml}\ntargets: {}\n")
		before := sharedMutationSnapshot(t, sb.Root)
		err := cmdSyncExtrasGlobal(false, true, jsonOutput, false, time.Now())
		var sharedErr *config.SharedError
		if !errors.As(err, &sharedErr) || sharedErr.Code != config.SharedUnavailable {
			t.Fatalf("apply=%v", err)
		}
		if !reflect.DeepEqual(before, sharedMutationSnapshot(t, sb.Root)) {
			t.Fatal("unavailable apply changed files")
		}
		sb.Cleanup()
	}
}
