package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/sharedfile"
	"skillshare/internal/testutil"
)

func sharedFixture(t *testing.T, local, shared string) (*testutil.Sandbox, *Config) {
	t.Helper()
	sb := testutil.NewSandbox(t)
	t.Cleanup(sb.Cleanup)
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml"), shared)
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\n" + local)
	cfg, err := LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	return sb, cfg
}

func TestSharedMergeAndPaths(t *testing.T) {
	sb, cfg := sharedFixture(t, "extras: [{name: override, targets: [{path: ~/local}]}]\n", `version: 1
extras:
  - name: rules
    source: extras/rules
    targets:
      - path: ~/rules
        extension: ./transform.sh
  - name: override
    file: fixture.md
    targets: [{path: ~/shared, mode: copy}]
`)
	if !cfg.SharedStatus().Usable || len(cfg.Extras) != 2 {
		t.Fatalf("status=%+v extras=%v", cfg.SharedStatus(), cfg.Extras)
	}
	if cfg.Extras[0].Source != filepath.Join(filepath.Dir(sb.ConfigPath), "extras", "rules") || cfg.Extras[0].Targets[0].Path != filepath.Join(sb.Home, "rules") || cfg.Extras[0].Targets[0].Extension != filepath.Join(filepath.Dir(sb.ConfigPath), "transform.sh") {
		t.Fatal(cfg.Extras[0])
	}
	if cfg.Extras[1].File != "" || cfg.ExtraOrigin("override") != sharedfile.OverridesShared || cfg.CheckExtraWritable("override") != nil {
		t.Fatal("local did not win whole")
	}
	if cfg.CheckExtraWritable("rules") == nil {
		t.Fatal("shared entry writable")
	}
	for _, pattern := range []string{"rules", "ru*", "*"} {
		cfg.Shared.Skip = map[string][]string{"extras": {pattern}}
		raw, err := serializeConfig(sb.ConfigPath, cfg)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := DecodeReadOnly(sb.ConfigPath, raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(loaded.Extras) != 1 || loaded.ExtraOrigin("rules") != sharedfile.Skipped {
			t.Fatalf("skip %q: %v", pattern, loaded.Extras)
		}
	}
}

func TestSharedUnavailableStillLoadsAndSavesLocal(t *testing.T) {
	for _, shared := range []string{"version: 2", "version: 1\nmcp: {}", "version: 1\nsources: {}", "version: 1\ncolour: blue", "version: 1\nextras: [{name: invalid.name, targets: []}]", "version: 1\nextras: [{name: a, targets: []}, {name: A, targets: []}]", "version: 1\nextras: [{name: a, targets: [{path: rules, mode: wrong}]}]", "["} {
		t.Run(shared, func(t *testing.T) {
			sb, cfg := sharedFixture(t, "extras: [{name: local, targets: []}]\n", shared)
			if cfg.SharedStatus().Usable || cfg.SharedStatus().Reason == "" || len(cfg.Extras) != 1 || cfg.CheckSharedApply() == nil {
				t.Fatalf("status=%+v", cfg.SharedStatus())
			}
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(sb.ReadFile(sb.ConfigPath), "path: ./team.yaml") || sb.ReadFile(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")) != shared {
				t.Fatal("save changed shared bytes or pointer")
			}
		})
	}
	sb, cfg := sharedFixture(t, "", "version: 1")
	if err := os.Remove(filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadReadOnlyPath(sb.ConfigPath)
	if err != nil || cfg.SharedStatus().Reason != sharedfile.Missing {
		t.Fatalf("missing: %v %v", cfg, err)
	}
}

func TestSharedPointerStrictAndFolded(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, tc := range []struct{ text, key string }{
		{"shared: {path: ''}", "shared.path"},
		{"shared: null", "shared.path"},
		{"shared: {path: ./config.yaml}", "shared.path"},
		{"shared: {path: team.yaml, accounts: {}}", "shared.accounts"},
		{"shared: {path: team.yaml, skip: {x: [a]}}", "shared.skip.x"},
		{"shared: {path: team.yaml, skip: {extras: ['[']}}", "shared.skip.extras"},
	} {
		_, err := DecodeReadOnly(sb.ConfigPath, []byte(tc.text))
		if err == nil || !strings.Contains(err.Error(), tc.key) {
			t.Fatalf("%s: %v", tc.text, err)
		}
	}
	sb.WriteConfig("shared: {path: " + filepath.ToSlash(filepath.Join(sb.Home, "team.yaml")) + ", skip: {hooks: ['*'], mcp: [a], plugins: [a]}}\n")
	cfg, err := LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.ReadFile(sb.ConfigPath), "path: ~/team.yaml") {
		t.Fatal("pointer not folded independent of preserve_tilde_on_save")
	}
}

func TestSharedPointerSavePortableSpelling(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, path := range []string{filepath.Join(sb.Home, "team.yaml"), filepath.ToSlash(filepath.Join(sb.Home, "team.yaml")), "~/team.yaml", "./team.yaml"} {
		payload := &Config{Shared: &SharedPointer{Path: path}}
		if err := sharedSavePayload(payload); err != nil {
			t.Fatal(err)
		}
		want := "~/team.yaml"
		if path == "./team.yaml" {
			want = path
		}
		if payload.Shared.Path != want {
			t.Fatalf("pointer %q saved as %q, want %q", path, payload.Shared.Path, want)
		}
	}
}

func TestSharedExtraPathTable(t *testing.T) {
	want := []string{"source", "targets[].path", "targets[].extension"}
	if !reflect.DeepEqual(sharedExtraPaths, want) {
		t.Fatal(sharedExtraPaths)
	}
	// Pin every path-looking field; File/As are validated plain filenames.
	fields := map[string]bool{"Source": true, "Path": true, "Extension": true}
	for _, typ := range []reflect.Type{reflect.TypeOf(ExtraConfig{}), reflect.TypeOf(ExtraTargetConfig{})} {
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if strings.Contains(f.Name, "Path") || strings.Contains(f.Name, "Source") || strings.Contains(f.Name, "Extension") {
				if !fields[f.Name] {
					t.Fatalf("unmapped path field %s", f.Name)
				}
			}
		}
	}
}

func TestSharedAbsolutePathWarnsAndBareExtensionStaysLocal(t *testing.T) {
	sb, cfg := sharedFixture(t, "", "version: 1\nextras: [{name: rules, source: ~/rules, targets: [{path: ~/target, extension: formatter}]}]\n")
	if cfg.Extras[0].Targets[0].Extension != "formatter" || len(cfg.SharedStatus().Warnings) != 0 {
		t.Fatal(cfg.SharedStatus())
	}
	shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
	sb.WriteFile(shared, "version: 1\nextras: [{name: rules, source: '"+filepath.ToSlash(filepath.Join(sb.Home, "rules"))+"', targets: []}]\n")
	cfg, err := LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.SharedStatus().Warnings) != 1 || !strings.Contains(cfg.SharedStatus().Warnings[0], "not portable") {
		t.Fatal(cfg.SharedStatus())
	}
}
