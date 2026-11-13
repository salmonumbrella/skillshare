package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
	"skillshare/internal/testutil"
)

func TestSharedOwnedSaveRefusesBeforeWrites(t *testing.T) {
	for _, edit := range []struct {
		name  string
		apply func(*Config)
	}{
		{"edit", func(c *Config) { c.Extras[0].Targets[0].Path = "changed" }},
		{"rename", func(c *Config) { c.Extras[0].Name = "renamed" }},
		{"remove", func(c *Config) { c.Extras = nil }},
		{"duplicate", func(c *Config) { c.Extras = append(c.Extras, c.Extras[0]) }},
	} {
		t.Run(edit.name, func(t *testing.T) {
			sb, cfg := sharedFixture(t, "", "version: 1\nextras: [{name: rules, targets: [{path: ~/rules}]}]\n")
			before := sb.ReadFile(sb.ConfigPath)
			info, _ := os.Stat(sb.ConfigPath)
			edit.apply(cfg)
			for _, save := range []func() error{cfg.Save, func() error { _, err := PrepareSave(sb.ConfigPath, cfg); return err }, func() error {
				_, err := PrepareProjectLayers(sb.ConfigPath, cfg, func(*ProjectLayers) error { return nil })
				return err
			}} {
				var owned *SharedError
				if err := save(); !errors.As(err, &owned) || owned.Code != SharedOwned {
					t.Fatalf("error=%v", err)
				}
				after, _ := os.Stat(sb.ConfigPath)
				if sb.ReadFile(sb.ConfigPath) != before || !info.ModTime().Equal(after.ModTime()) {
					t.Fatal("refusal wrote config")
				}
			}
		})
	}
}

func TestNoPointerSaveBytesMatchBase(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteFile(filepath.Join(filepath.Dir(sb.ConfigPath), "config.shared.yaml"), "unreadable fixture")
	if err := os.Chmod(filepath.Join(filepath.Dir(sb.ConfigPath), "config.shared.yaml"), 0); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		"source: ~/skills\nmode: merge\ntargets: {}\n",
		"# my config\nsource: ~/skills # shared source\nmode: &mode copy\ntargets:\n  claude:\n    skills:\n      mode: *mode\n",
		"source: ~/skills\nskills: [{name: fixture, source: https://example.com/fixture}]\n",
	} {
		sb.WriteConfig(input)
		cfg, err := LoadReadOnlyPath(sb.ConfigPath)
		if err != nil {
			t.Fatal(err)
		}
		// This is the unchanged base serializer, with no shared hook involved.
		payload := cfg.withoutProjectTargets()
		want, err := marshalConfigFile(sb.ConfigPath, payload, schemaComment)
		if err != nil {
			t.Fatal(err)
		}
		if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(sb.ConfigPath); !bytes.Equal(got, want) {
			t.Fatal("Save changed base bytes")
		}
		// The legacy-skills candidate has its own existing serializer branch.
		sb.WriteConfig(input)
		var value any = payload
		if payload.legacySkills.Kind != 0 {
			type plain Config
			value = struct {
				*plain `yaml:",inline"`
				Skills yaml.Node `yaml:"skills,omitempty"`
			}{(*plain)(payload), payload.legacySkills}
		}
		candidateWant, err := marshalConfigFile(sb.ConfigPath, value, schemaComment)
		if err != nil {
			t.Fatal(err)
		}
		candidate, err := PrepareSave(sb.ConfigPath, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(candidate.Bytes, candidateWant) {
			t.Fatal("PrepareSave changed base serializer bytes")
		}
		if err := candidate.Write(); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(sb.ConfigPath); !bytes.Equal(got, candidate.Bytes) {
			t.Fatal("candidate differs")
		}
		cfg, err = LoadReadOnlyPath(sb.ConfigPath)
		if err != nil {
			t.Fatal(err)
		}
		before := sb.ReadFile(sb.ConfigPath)
		project, err := PrepareProjectLayers(sb.ConfigPath, cfg, func(*ProjectLayers) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		if err := project.Write(); err != nil {
			t.Fatal(err)
		}
		if got := sb.ReadFile(sb.ConfigPath); got != before {
			t.Fatal("no-op project save changed config")
		}
	}
	if _, err := os.Stat(filepath.Join(ProjectStateDir(), "shared")); !os.IsNotExist(err) {
		t.Fatalf("no-pointer state accessed/created: %v", err)
	}
}
