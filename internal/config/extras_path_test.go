package config

import (
	"path/filepath"
	"testing"
)

func TestExtrasSourceDirProject(t *testing.T) {
	got := ExtrasSourceDirProject("/projects/myapp/.skillshare/extras", "rules")
	want := filepath.Join("/projects/myapp/.skillshare/extras", "rules")
	if got != want {
		t.Errorf("ExtrasSourceDirProject() = %q, want %q", got, want)
	}
}

func TestResolveExtrasSourceDir_PerExtraSource(t *testing.T) {
	extra := ExtraConfig{Name: "rules", Source: "/custom/rules"}
	got := ResolveExtrasSourceDir(extra, "/global-extras", "/skills")
	if got != "/custom/rules" {
		t.Errorf("expected /custom/rules, got %s", got)
	}
}

func TestResolveExtrasSourceDir_GlobalExtrasSource(t *testing.T) {
	extra := ExtraConfig{Name: "rules"}
	got := ResolveExtrasSourceDir(extra, "/global-extras", "/skills")
	want := filepath.Join("/global-extras", "rules")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestResolveExtrasSourceDir_Default(t *testing.T) {
	extra := ExtraConfig{Name: "rules"}
	got := ResolveExtrasSourceDir(extra, "", "/home/user/.config/skillshare/skills")
	want := filepath.Join("/home/user/.config/skillshare", "extras", "rules")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestResolveExtrasSourceDir_PerExtraOverridesAll(t *testing.T) {
	extra := ExtraConfig{Name: "rules", Source: "/exact/path"}
	got := ResolveExtrasSourceDir(extra, "/global-extras", "/skills")
	if got != "/exact/path" {
		t.Errorf("expected /exact/path, got %s", got)
	}
}

func TestResolveExtrasSourceDir_EmptyExtrasSource(t *testing.T) {
	extra := ExtraConfig{Name: "rules"}
	got := ResolveExtrasSourceDir(extra, "", "/home/user/.config/skillshare/skills")
	want := filepath.Join("/home/user/.config/skillshare", "extras", "rules")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestResolveExtrasSourceDir_EmptyPerExtraSource(t *testing.T) {
	extra := ExtraConfig{Name: "rules", Source: ""}
	got := ResolveExtrasSourceDir(extra, "/global-extras", "/skills")
	want := filepath.Join("/global-extras", "rules")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestResolveExtrasSourceType(t *testing.T) {
	tests := []struct {
		name         string
		extra        ExtraConfig
		extrasSource string
		want         string
	}{
		{"per-extra", ExtraConfig{Source: "/custom"}, "/global", "per-extra"},
		{"extras_source", ExtraConfig{}, "/global", "extras_source"},
		{"default", ExtraConfig{}, "", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveExtrasSourceType(tt.extra, tt.extrasSource)
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestExtrasParentDir(t *testing.T) {
	got := ExtrasParentDir("/home/user/.config/skillshare/skills")
	want := filepath.Join("/home/user/.config/skillshare", "extras")
	if got != want {
		t.Errorf("ExtrasParentDir() = %q, want %q", got, want)
	}
}

func TestResolveExtrasSourceDirProject(t *testing.T) {
	root := filepath.FromSlash("/projects/myapp")
	parent := filepath.Join(root, ".skillshare", "extras")
	got := ResolveExtrasSourceDirProject(ExtraConfig{Name: "a", Source: ".skillshare/extras/prompts"}, parent, root)
	if want := filepath.Join(parent, "prompts"); got != want {
		t.Errorf("with source = %q, want %q", got, want)
	}
	got = ResolveExtrasSourceDirProject(ExtraConfig{Name: "a"}, parent, root)
	if want := filepath.Join(parent, "a"); got != want {
		t.Errorf("without source = %q, want %q", got, want)
	}
}

func TestValidateProjectExtraSource(t *testing.T) {
	for _, source := range []string{"", ".skillshare/extras/prompts", "docs/prompts"} {
		if err := ValidateProjectExtraSource(source); err != nil {
			t.Errorf("%q: unexpected error %v", source, err)
		}
	}
	for _, source := range []string{"/abs/prompts", "~/prompts", "..", "../prompts", "a/../../prompts"} {
		if err := ValidateProjectExtraSource(source); err == nil {
			t.Errorf("%q: expected an error", source)
		}
	}
}

// Two single-file extras may share one project folder, as in global mode.
func TestProjectValidateExtras_SharedSourceFolder(t *testing.T) {
	root := t.TempDir()
	cfg := &ProjectConfig{Extras: []ExtraConfig{
		{Name: "a", Source: ".skillshare/extras/prompts", File: "a.md", Targets: []ExtraTargetConfig{{Path: ".claude/commands"}}},
		{Name: "b", Source: ".skillshare/extras/prompts", File: "b.md", Targets: []ExtraTargetConfig{{Path: ".claude/commands"}}},
	}}
	if err := cfg.ValidateExtras(root); err != nil {
		t.Fatalf("ValidateExtras() = %v, want nil", err)
	}
}

func TestProjectValidateExtras_RejectsEscapingSource(t *testing.T) {
	cfg := &ProjectConfig{Extras: []ExtraConfig{
		{Name: "a", Source: "../prompts", File: "a.md", Targets: []ExtraTargetConfig{{Path: ".claude/commands"}}},
	}}
	if err := cfg.ValidateExtras(t.TempDir()); err == nil {
		t.Fatal("ValidateExtras() = nil, want an error for a source outside the project")
	}
}
