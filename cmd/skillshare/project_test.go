package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/projects"
	ssync "skillshare/internal/sync"
)

func TestProjectParsePreservesValuesAndTerminator(t *testing.T) {
	o, err := parseProjectArgs([]string{"edit", "--name", "-p", "--target", "claude", "--", "--json"})
	if err != nil || o.json || o.token != "--json" || o.values["--name"][0] != "-p" {
		t.Fatalf("%+v %v", o, err)
	}
	o, err = parseProjectArgs([]string{"--name=--json", "edit", "app"})
	if err != nil || o.json || o.values["--name"][0] != "--json" {
		t.Fatalf("%+v %v", o, err)
	}
}
func TestProjectPatchKeepsResourceSettingsAndCopiesLists(t *testing.T) {
	entry := config.ManagedProject{Targets: []string{"claude"}, Skills: &config.ResourceTargetConfig{Mode: "copy", Include: []string{"old"}}, Agents: &config.ResourceTargetConfig{Mode: "merge"}}
	original := *entry.Skills
	o, err := parseProjectArgs([]string{"edit", "app", "--skills", "--skills-include", "new", "--agents-exclude", "ignore"})
	if err != nil {
		t.Fatal(err)
	}
	next, err := patchProject(entry, o)
	if err != nil {
		t.Fatal(err)
	}
	if next.Skills.Mode != "copy" || !reflect.DeepEqual(next.Skills.Include, []string{"new"}) || !reflect.DeepEqual(entry.Skills.Include, original.Include) || len(entry.Agents.Exclude) != 0 {
		t.Fatalf("next %+v original %+v", next, entry)
	}
}

func TestProjectPatchExplicitEnablePreservesDisabledSettings(t *testing.T) {
	for _, kind := range []string{"skills", "agents"} {
		for _, flag := range []string{"--" + kind, "--name=renamed", "--" + kind + "-include=new"} {
			t.Run(kind+flag, func(t *testing.T) {
				rc := &config.ResourceTargetConfig{Mode: "copy", Include: []string{"old"}}
				rc.SetEnabled(false)
				entry := config.ManagedProject{Targets: []string{"claude"}}
				if kind == "skills" {
					entry.Skills = rc
				} else {
					entry.Agents = rc
				}
				o, err := parseProjectArgs([]string{"edit", "app", flag})
				if err != nil {
					t.Fatal(err)
				}
				next, err := patchProject(entry, o)
				if err != nil {
					t.Fatal(err)
				}
				got := next.Skills
				if kind == "agents" {
					got = next.Agents
				}
				wantEnabled := flag == "--"+kind
				wantInclude := []string{"old"}
				if strings.Contains(flag, "-include=") {
					wantInclude = []string{"new"}
				}
				if !reflect.DeepEqual(got.Include, wantInclude) {
					t.Errorf("settings lost: got %v want %v", got.Include, wantInclude)
				}
				if got.IsEnabled() != wantEnabled || got.Mode != "copy" || rc.IsEnabled() || rc.Include[0] != "old" {
					t.Fatalf("explicit enable=%v: next=%+v original=%+v", wantEnabled, got, rc)
				}
			})
		}
	}
}

func TestProjectTextPreparedSyncDiagnostics(t *testing.T) {
	r := projects.Result{Sync: &projects.ExecuteResult{
		Unmatched:       []ssync.TargetUnmatchedInclude{{Target: "app@claude", Patterns: []string{"typo"}, Suggestions: []string{"team_alpha"}, All: true}},
		FolderConflicts: []config.SkillsFolderConflict{{Targets: []string{"app@claude", "ordinary"}, Path: "/prepared/skills", Stop: []string{"ordinary", "app@claude"}}},
		PathOverlap:     2,
	}}
	output := stripANSIWarnings(captureStdout(t, func() {
		if err := renderProjectResult(projectOptions{action: "sync", dryRun: true}, r, nil, time.Now()); err != nil {
			t.Fatal(err)
		}
	}))
	for _, want := range []string{`app@claude: include filter "typo" matches no skill`, `did you mean "team_alpha"`, "app@claude and ordinary sync skills to /prepared/skills with different settings", "keep one: skillshare target ordinary --skills=false", "keep one: skillshare project edit --no-skills -- 'app'", "2 targets share skill folders", "skillshare doctor"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in %s", want, output)
		}
	}
}
