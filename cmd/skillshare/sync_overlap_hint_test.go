package main

import (
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/sync"
)

func TestSkillsOffCommand_ProjectTargets(t *testing.T) {
	for _, tc := range []struct {
		name      string
		isProject bool
		want      string
	}{
		{"app@claude", false, "skillshare project edit --no-skills -- 'app'"},
		{"app space@claude", false, "skillshare project edit --no-skills -- 'app space'"},
		{"app's@claude", false, "skillshare project edit --no-skills -- 'app'\\''s'"},
		{"-app@claude", false, "skillshare project edit --no-skills -- '-app'"},
		{"codex", false, "skillshare target codex --skills=false"},
		{"codex", true, "skillshare target codex --skills=false -p"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := skillsOffCommand(tc.name, tc.isProject); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPrintSyncOverlapHint_NamesConflictAndFix(t *testing.T) {
	keepPathsUnfolded(t)
	targets := map[string]config.TargetConfig{
		"universal": {Skills: &config.ResourceTargetConfig{Path: "/tmp/agents/skills", Exclude: []string{"feature-radar*"}}},
		"codex":     {Skills: &config.ResourceTargetConfig{Path: "/tmp/agents/skills"}},
	}
	out := captureStdout(t, func() { printSyncOverlapHint(targets, true, false, "", nil) })

	for _, want := range []string{
		"codex and universal sync skills to " + filepath.FromSlash("/tmp/agents/skills") + " with different settings, so each sync undoes the other",
		"keep one: skillshare target codex --skills=false -p",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "share skill folders") {
		t.Errorf("conflict fully explains the overlap, generic line should be gone:\n%s", out)
	}
}

func TestPrintSyncOverlapHint_SameSettingsKeepsGenericLine(t *testing.T) {
	targets := map[string]config.TargetConfig{
		"universal": {Skills: &config.ResourceTargetConfig{Path: "/tmp/agents/skills"}},
		"codex":     {Skills: &config.ResourceTargetConfig{Path: "/tmp/agents/skills"}},
	}
	out := captureStdout(t, func() { printSyncOverlapHint(targets, false, false, "", nil) })

	if !strings.Contains(out, "2 targets share skill folders") {
		t.Errorf("expected generic overlap line:\n%s", out)
	}
	if strings.Contains(out, "undoes") {
		t.Errorf("same settings is not a conflict:\n%s", out)
	}
}

func TestPrintSyncOverlapHint_OpenCodeSameSkillsIsQuiet(t *testing.T) {
	cfg := opencodeAndClaude(t)
	discovered := []sync.DiscoveredSkill{{FlatName: "shared"}}
	out := captureStdout(t, func() { printSyncOverlapHint(cfg.Targets, false, false, cfg.Mode, discovered) })

	if strings.Contains(out, "share skill folders") {
		t.Errorf("opencode loads nothing extra from claude's folder, expected no hint:\n%s", out)
	}
}
