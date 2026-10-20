package main

import (
	"reflect"
	"testing"

	"skillshare/internal/config"
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
